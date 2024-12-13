// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package semver

import (
	"fmt"
	"strings"
)

// Constraint holds a parsed constraint specification.
type Constraint struct {
	str    string // Trimmed input to ParseConstraint.
	sys    System // Packaging system in which constraint was expressed.
	simple bool   // Constraint was created by a simple version, not a wildcard, no operators except ==.
	set    Set
}

// String returns the string used to create the Constraint, trimmed of
// leading and trailing space.
func (c *Constraint) String() string {
	return c.str
}

// IsSimple reports whether the constraint is formed by a simple version or, for those
// systems that support it, an equality operator and a simple version.
// A simple constraint can still match multiple versions, depending on the System.
// For Maven, a simple constraint is a "soft" constraint; for Cargo, a simple version
// is equivalent to the version preceded by the caret operator, and so on.
// Present mostly to assist in understanding version resolution in Maven.
func (c *Constraint) IsSimple() bool {
	return c.simple
}

// Set returns the set representation of the constraint.
func (c *Constraint) Set() Set {
	return c.set
}

// HasPrerelease reports whether the constraint contains a prerelease tag.
func (c *Constraint) HasPrerelease() bool {
	for _, sp := range c.set.span {
		if sp.min.IsPrerelease() || sp.max.IsPrerelease() {
			return true
		}
	}
	return false
}

type constraintParser struct {
	*Constraint     // Accumulator for result.
	weight      int // Incremented for each version or operator. Used to set Constraint.simple.
	lex         lexer
}

// ParseConstraint returns the result of parsing the constraint string in the
// packaging system.
// The syntax is System-dependent and is defined in the package comment.
func (sys System) ParseConstraint(str string) (retC *Constraint, retErr error) {
	str = strings.TrimSpace(str)
	// Special case: The empty constraint actually means everything, not nothing.
	// Simplest approach for this case is to replace the incoming string to avoid
	// creating the empty set, which means the opposite.
	lexStr := str
	if lexStr == "" {
		if sys == NuGet {
			return nil, fmt.Errorf("invalid empty constraint")
		}
		lexStr = ">=0.0.0"
	}
	parser := constraintParser{
		Constraint: &Constraint{
			str: str,
			sys: sys,
		},
		weight: 0,
		lex:    lexer{str: lexStr},
	}
	return parser.constraint()
}

// ParseSetConstraint returns the result of parsing the set represented by str
// in the packaging system. The syntax is system-independent. Trimmed of leading
// and trailing spaces, the syntax is a list of comma-separated spans inside a
// braces:
//
//	{span,span,...}
//
// There are no extraneous spaces. The string "{}" is the empty set and matches
// nothing.
// Spans are formatted according to their rank:
//
//	An empty span: <empty>
//	A single version: 1.2.3-alpha
//	A span between two versions: [1.2.3:2.3.4]
//
// In the last case, a bracket will be ( or ) if the span is open on the
// corresponding side. On the right-hand side of a span, a number (major, minor,
// patch) may be replaced with ∞ to represent a value greater than all numeric
// values.
func (sys System) ParseSetConstraint(str string) (*Constraint, error) {
	str = strings.TrimSpace(str)
	set, simple, err := sys.parseSet(str)
	if err != nil {
		return nil, err
	}
	return &Constraint{
		str:    str,
		sys:    sys,
		simple: simple,
		set:    set,
	}, nil
}

func (p *constraintParser) constraint() (*Constraint, error) {
	sys := p.Constraint.sys
	// Go is very restrictive: No operators at all, but a version as a constraint
	// represents a range.
	if sys == Go {
		lo, err := Go.Parse(p.lex.str)
		p.lex.setError(err)
		var s span
		if err == nil {
			hi := lo.copy()
			// v3.2.4 as a constraint means ^v3.2.4 == [v3.2.4,v4.0.0).
			// But if the major is zero, as in v0.2.4, it is defined
			// to be compatible with v1, so it means [v0.2.4,v2.0.0).
			if lo.major() == 0 {
				hi.incN(nMajor)
			}
			hi.incN(nMajor)
			hi.setMinor(0)
			hi.setPatch(0)
			s, err = newSpan(lo, closed, hi, open)
			p.lex.setError(err)
			p.Constraint.set = Set{
				sys:  Go,
				span: []span{s},
			}
			p.Constraint.simple = true // Always true for Go.
		}
		return p.Constraint, err
	}

	// Some systems require an operator be present, that is, that the first
	// element of the constraint is not a version.
	switch sys {
	case PyPI:
		typ, _, _ := sys.token(p.lex.str)
		if typ == tokVersion {
			p.lex.setErr("missing operator")
		}
	}

	set := p.orList()
	if len(set.span) != 0 {
		p.set = set
		if p.lex.err == nil {
			typ, tok, _ := sys.token(p.lex.str[p.lex.pos:])
			if typ != tokEOF {
				p.lex.unexpected(typ, tok)
			}
		}
	}
	typ, tok, _ := sys.token(p.lex.str[p.lex.pos:])
	if typ != tokEOF {
		p.lex.unexpected(typ, tok)
	}
	p.Constraint.simple = p.weight == 1
	return p.Constraint, p.lex.err
}

/*
orList = span // See value method below.

	| andList
	| orList '||' andList // NPM, Default only.
	| orList ',' andList // Maven and NuGet only.

span = VERSION ' ' '-' ' ' VERSION // NPM, Default only. Spaces required.
*/
func (p *constraintParser) orList() Set {
	sys := p.Constraint.sys
	lastWasOr := false
	var spans []span
	orToken := tokOr
	if sys == Maven || sys == NuGet {
		orToken = tokComma
	}
	for {
		set, ok := p.andList()
		if !ok {
			if lastWasOr {
				p.lex.setErr("missing item after " + strings.ToLower(orToken.String()))
				return Set{}
			}
			break
		}
		lastWasOr = false
		spans = append(spans, set.span...)
		if sys == NuGet && len(spans) > 1 {
			p.lex.setErr("cannot have more than one range")
			return Set{}
		}
		typ, _, i := sys.token(p.lex.str[p.lex.pos:])
		if typ == orToken {
			lastWasOr = true
			p.lex.pos += i
			continue
		}
		break
	}
	spans, err := canon(spans)
	if err != nil {
		p.lex.setError(err)
		return Set{}
	}
	return Set{
		sys:  sys,
		span: spans,
	}
}

/*
	andList = value
		| andList value
		| andList ',' value // If comma is supported for AND.

If the value is a span, it must be the only item in the list.
See the value method below.

Spans in Maven are bracketed; in NPM they are hyphenated.
Return values are the set, whether the item contains only simple
(unadorned) versions, and whether the item is valid.
*/
func (p *constraintParser) andList() (Set, bool) {
	sys := p.Constraint.sys
	switch sys {
	case Maven, NuGet:
		return p.setRange(sys)
	}
	var set Set
	first := true
	lastWasComma := false // Last token we saw was a comma.
	for ; ; first = false {
		spans, hyphenated, ok := p.value()
		if !ok {
			if lastWasComma {
				p.lex.setErr("missing item after comma")
			}
			break
		}
		lastWasComma = false
		if first {
			set.span = spans
			if hyphenated {
				return set, true
			}
		} else {
			if hyphenated {
				p.lex.setErr("unexpected range after version")
				break
			}
			err := set.Intersect(Set{span: spans})
			if err != nil {
				p.lex.setError(err)
				return set, false
			}
		}
		typ, tok, i := sys.token(p.lex.str[p.lex.pos:])
		switch typ {
		case tokEOF:
			// OK
		case tokInvalid:
			p.lex.unexpected(typ, tok)
		case tokComma:
			lastWasComma = true
			p.lex.pos += i
		case tokOr:
			// OK
		default:
			if !sys.supportsAnd() {
				p.lex.setErr("and list not supported in " + sys.String())
			}
			if sys == RubyGems {
				p.lex.setErr("missing comma in " + sys.String())
			}
		}
	}
	return set, !first
}

/*
	value = VERSION
		| span
		| unop VERSION
		| WILDCARD
	span = vs '-' vs
	vs = VERSION
		| SPAN

The booleans report whether the value is a hyphenated range (span
such as "1 - 3"), whether there was no value, just a comma (Ruby
presents constraint strings as comma-separated lists) and whether
the returned value is valid or a comma (if not, we're at EOF or a
non-value token). If it is hyphenated, andList (above) will stop
scanning immediately. Spans cannot be "anded" with other items, a
quirk of the NPM (at least) grammar with no apparent justification.
*/
func (p *constraintParser) value() (spans []span, hyphenated, valid bool) {
	sys := p.Constraint.sys
	typ, tok, i := sys.token(p.lex.str[p.lex.pos:])
	switch typ {
	case tokEOF:
		return
	case tokInvalid:
		p.lex.unexpected(typ, tok)
		return
	case tokEqual, tokGreater, tokGreaterEqual, tokLess, tokLessEqual, tokNotEqual, tokCaret, tokTilde, tokBacon:
		unop := tok
		typ2, tok2, j := sys.token(p.lex.str[p.lex.pos+i:])
		if typ2 != tokVersion && typ2 != tokWildcard {
			p.lex.setErr("expected version after operator")
			return
		}
		version, err := sys.Parse(tok2)
		if err != nil {
			p.lex.setError(err)
			return
		}
		p.lex.pos += i + j
		// Special case for !=. This is the one case we need two spans.
		// TODO: Maybe opVersionToSpan should handle this, but that would
		// require changing its signature and affects a lot of code. Do that if
		// != shows up more broadly.
		if unop == "!=" {
			var left, right span
			left, right, err = excludeToSpans(version)
			spans = []span{left, right}
		} else {
			var s span
			s, err = opVersionToSpan(typ, unop, version)
			spans = []span{s}
		}
		if err != nil {
			p.lex.setError(err)
			return
		}
		p.weight++
		if typ != tokEqual {
			p.weight++
		}
	case tokVersion, tokWildcard:
		// Is it a version span?
		typ2, text, j := sys.token(p.lex.str[p.lex.pos+i:])
		if typ2 == tokInvalid {
			p.lex.setErr(fmt.Sprintf("invalid text %#q", text))
			return
		}
		if typ2 != tokHyphen {
			// No.
			version, err := sys.Parse(tok)
			p.weight++
			if version.IsWildcard() {
				p.weight++
			}
			p.lex.setError(err)
			p.lex.pos += i
			if version != nil {
				var err error
				op := ""
				opType := tokEmpty
				// In Cargo, the default operator is '^'.
				if sys == Cargo && typ == tokVersion {
					op = "^"
					opType = tokCaret
				}
				s, err := opVersionToSpan(opType, op, version)
				if err != nil {
					p.lex.setError(err)
					return
				}
				spans = []span{s}
			}
			break
		}
		// Yes. Expect a version (possibly with wildcard) after the hyphen.
		typ2, tok2, k := sys.token(p.lex.str[p.lex.pos+i+j:])
		if typ2 != tokVersion && typ2 != tokWildcard {
			p.lex.setErr("expected version after hyphen")
			return
		}
		lo, err := sys.Parse(tok)
		p.lex.setError(err)
		hi, err := sys.Parse(tok2)
		p.lex.setError(err)
		p.lex.pos += i + j + k
		p.weight += 2
		if lo != nil && hi != nil {
			if hi.lessThan(lo) {
				p.lex.setErr("impossible constraint: max greater than min")
				return
			}
			hi.fill(infinity)
			s, err := newSpan(lo, closed, hi, closed)
			if err != nil {
				p.lex.setError(err)
				return
			}
			spans = []span{s}
		}
		return spans, true, true
	default:
		return nil, false, false
	}
	return spans, false, true
}

/*
setRange parses a version range in Maven/NuGet syntax for constraints.

	range = VERSION
		| '[' VERSION ']'
		| lbra opVersion ',' opVersion rbra
	lbra = '[' | '('
	rbra = ']' | ')'
	opVersion =
		| VERSION
*/
func (p *constraintParser) setRange(sys System) (set Set, ok bool) {
	var sp span
	typ, tok, i := sys.token(p.lex.str[p.lex.pos:])
	switch typ {
	case tokEOF:
		return
	case tokInvalid:
		p.lex.unexpected(typ, tok)
		return
	case tokWildcard:
		if sys != NuGet {
			p.lex.setErr(fmt.Sprintf("internal error: %s: invalid System with wildcard in setRange", sys))
			return
		}
		fallthrough
	case tokVersion:
		v, err := sys.Parse(tok)
		if err != nil {
			p.lex.setError(err)
			return
		}
		p.weight++
		if v.IsWildcard() {
			p.weight++
		}
		switch sys {
		case Maven:
			// "Soft" requirement, which matches anything so
			// we ignore the value. The resolver should prefer
			// this version, but the constraint will match anything.
			zero, err := newVersion(sys, tok, "0.0.0", 0, nil)
			if err != nil {
				p.lex.setError(err)
				return
			}
			sp, _ = opVersionToSpan(tokGreaterEqual, ">=", zero)
		case NuGet:
			// 1.0.0 as a constraint means >=1.0.0.
			sp, _ = opVersionToSpan(tokGreaterEqual, ">=", v)
		default:
			p.lex.setErr(fmt.Sprintf("internal error: %s: invalid System with version in setRange", sys))
			return
		}
		p.lex.pos += i
	case tokLbracket:
		var err error
		var min, max *Version
		p.weight += 2
		minOpen := tok == "("
		p.lex.pos += i
		typ, tok, i = sys.token(p.lex.str[p.lex.pos:])
		if typ == tokVersion {
			min, err = sys.Parse(tok)
			if err != nil {
				p.lex.setError(err)
				return
			}
			p.lex.pos += i
			typ, tok, i = sys.token(p.lex.str[p.lex.pos:])
		}
		// If min is nil, we have an empty LHS.
		if min == nil {
			min, _ = sys.Parse("0")
			minOpen = false
		}
		// Need a comma or closing bracket.
		if typ != tokComma && typ != tokRbracket {
			p.lex.setErr("expected comma or closing bracket")
			return
		}
		p.lex.pos += i
		if typ == tokRbracket {
			// Only one version, a "hard" requirement like [1.0].
			// TODO: How best to support this? User can check str.
			if minOpen || tok == ")" {
				p.lex.setErr("hard requirement must be closed on both ends")
			}
			sp, err = newSpan(min, false, min, false)
			if err != nil {
				p.lex.setError(err)
				return
			}
			break
		}
		// Have a comma, need another optional version, then closing bracket.
		typ, tok, i = sys.token(p.lex.str[p.lex.pos:])
		if typ == tokVersion {
			max, err = sys.Parse(tok)
			if err != nil {
				p.lex.setError(err)
				return
			}
			p.lex.pos += i
			typ, tok, i = sys.token(p.lex.str[p.lex.pos:])
		}
		maxOpen := tok == ")"
		// If max is nil, we have an empty LHS.
		if max == nil {
			max, err = newVersion(sys, "∞.∞.∞", "∞.∞.∞", infinity, nil)
			if err != nil {
				p.lex.setError(err)
				return
			}
			maxOpen = false
		}
		// Need a closing bracket.
		if typ != tokRbracket {
			p.lex.setErr("expected closing bracket")
			return
		}
		sp, err = newSpan(min, minOpen, max, maxOpen)
		if err != nil {
			p.lex.setError(err)
			return
		}
		p.lex.pos += i
	default:
		return
	}
	set = Set{
		sys:  sys,
		span: []span{sp},
	}
	return set, true
}
