// Copyright 2025 Google LLC
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

package pypi

/*
This file contains parsers for PEP 508 environment markers
(https://www.python.org/dev/peps/pep-0508/#environment-markers).
The relevant parts of the grammar are:
marker       = marker_or
marker_or    = marker_and wsp* 'or' marker_or
             | marker_and
marker_and   = marker_expr wsp* 'and' marker_and
             | marker_expr
marker_expr  = marker_var marker_op marker_var
             | wsp* '(' marker ')'
marker_var   = wsp* (env_var | python_str)
env_var      = 'python_version' | 'python_full_version' | os_name'
             | 'sys_platform' | 'platform_release' | 'platform_system'
             | 'platform_machine' | 'platform_python_implementation'
             | 'implementation_name' | 'implementation_version' | 'extra'
python_str   = (squote (python_str_c | dquote)* squote)
             | (dquote (python_str_c | squote)* dquote)
squote       = '\''
dquote       = '"'
python_str_c = wsp | letter | digit | '(' | ')' | '.' | '{' | '}' | '-' | '_'
             | '*' | '#' | ':' | ';' | ',' | '/' | '?' | '[' | ']' | '!' | '~'
             | '`' | '@' | '$' | '%' | '^' | '&' | '=' | '+' | '|' | '<' | '>'
marker_op    = version_cmp | (wsp* 'in') | (wsp* 'not' wsp+ 'in')
version_cmp  = wsp* ('<=' | '<' | '!=' | '==' | '>=' | '>' | '~=' | '===')
wsp          = ' ' | '\t'

The rules for marker_or and marker_and have been modified to allow for more
than one marker_or/marker_and in a marker without the need for parentheses.
This reflects the actual implementation of pip.
*/

import (
	"fmt"
	"strings"

	"deps.dev/util/resolve/pypi/internal"
	"deps.dev/util/semver"
)

func parseMarker(raw string) (marker, error) {
	p := &envParser{input: raw}
	m, err := p.parseMarkerOr()
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.input) {
		return nil, p.expected("EOF")
	}
	return m, nil
}

// marker is a parsed environment marker.
type marker interface {
	String() string
	Eval(extras map[string]bool) bool
}

// envParser parses PEP 508 environment markers.
type envParser struct {
	// input holds the string being parsed, which is assumed to be ASCII as per
	// PEP 508.
	input string
	pos   int // The current position in input.
}

// skipWsp skips zero or more characters of whitespace. By PEP 508, allowed
// whitespace is spaces or tabs. It returns true if any characters were skipped.
func (p *envParser) skipWsp() bool {
	newPos := p.pos
	for ; newPos < len(p.input) && isSpace(p.input[newPos]); newPos++ {
	}
	if newPos == p.pos {
		return false
	}
	p.pos = newPos
	return true
}

// isSpace reports whether the provided byte is one of the allowed space
// characters.
func isSpace(b byte) bool {
	return b == ' ' || b == '\t'
}

// accept attempts to take a literal string from the current position of the
// input and reports whether this was successful. If it succeeds the position is
// advanced past the string.
func (p *envParser) accept(s string) bool {
	if !strings.HasPrefix(p.input[p.pos:], s) {
		return false
	}
	p.pos += len(s)
	return true
}

const eof byte = 255

// peek returns the next byte in the input or eof if there is none.
func (p *envParser) peek() byte {
	if p.pos >= len(p.input) {
		return eof
	}
	return p.input[p.pos]
}

// expected produces a formatted error to indicate the parser having not
// found what it expected.
func (p *envParser) expected(want string) error {
	end := p.input[p.pos:]
	if len(end) > 10 {
		end = end[:10]
	}
	if len(end) == 0 {
		end = "EOF"
	}
	return fmt.Errorf("expected: %s, found: %q", want, end)
}

// parseMarkerAnd parses a marker_or.
// marker_or    = marker_and wsp* 'or' marker_or
//
//	| marker_and
func (p *envParser) parseMarkerOr() (marker, error) {
	l, err := p.parseMarkerAnd()
	if err != nil {
		return nil, err
	}
	p.skipWsp()
	if !p.accept("or") {
		return l, nil
	}
	r, err := p.parseMarkerOr()
	if err != nil {
		return nil, err
	}
	return markerOr{left: l, right: r}, nil
}

// parseMarkerAnd parses a marker_and.
// marker_and   = marker_expr wsp* 'and' marker_and
//
//	| marker_expr
func (p *envParser) parseMarkerAnd() (marker, error) {
	l, err := p.parseMarkerExpr()
	if err != nil {
		return nil, err
	}
	p.skipWsp()
	if !p.accept("and") {
		return l, nil
	}
	r, err := p.parseMarkerAnd()
	if err != nil {
		return nil, err
	}
	return markerAnd{left: l, right: r}, nil
}

// parseMarkerVar parses a marker_var, which is either a known variable name or
// a literal string with quotes:
// marker_var   = wsp* (env_var | python_str)
// env_var      = 'python_version' | 'python_full_version' | os_name'
//
//	| 'sys_platform' | 'platform_release' | 'platform_system'
//	| 'platform_machine' | 'platform_python_implementation'
//	| 'implementation_name' | 'implementation_version' | 'extra'
func (p *envParser) parseMarkerVar() (markerVar, error) {
	p.skipWsp()
	// Test if is a python_str.
	if str, err := p.parsePythonStr(); err == nil {
		return mkMarkerVar("", str), nil
	}
	// Could it be a known variable name?
	switch c := p.peek(); c {
	default:
		return markerVar{}, p.expected("known variable name")
	case 'e', 'i', 'o', 'p', 's':
		// Possibly, continue on to check.
	}
	// None of the names are prefixes of one another, so we can just try them
	// all in any order.
	for n, v := range environmentVariables {
		if p.accept(n) {
			return v, nil
		}
	}
	return markerVar{}, p.expected("string or variable name")
}

// parsePythonStr loosely parses a python_str, which is a string literal.
// The grammar defines a precise set of what is allowed inside the string,
// but pip itself (at version 20.3) does not seem to care so neither do we.
//
// python_str   = (squote (python_str_c | dquote)* squote)
//
//	| (dquote (python_str_c | squote)* dquote)
//
// squote       = '\”
// dquote       = '"'
// python_str_c = wsp | letter | digit | '(' | ')' | '.' | '{' | '}' | '-' | '_'
//
//	| '*' | '#' | ':' | ';' | ',' | '/' | '?' | '[' | ']' | '!' | '~'
//	| '`' | '@' | '$' | '%' | '^' | '&' | '=' | '+' | '|' | '<' | '>'
func (p *envParser) parsePythonStr() (string, error) {
	s := p.peek()
	if s != '\'' && s != '"' {
		return "", p.expected("string literal")
	}
	i := strings.IndexByte(p.input[p.pos+1:], s)
	if i < 0 {
		return "", p.expected(fmt.Sprintf("%q terminating a string", s))
	}
	val := p.input[p.pos+1 : p.pos+i+1]
	p.pos += i + 2
	return val, nil
}

// parseMarkerOp parses a marker_op:
// marker_op   = version_cmp | (wsp* 'in') | (wsp* 'not' wsp+ 'in')
// version_cmp = wsp* ('<=' | '<' | '!=' | '==' | '>=' | '>' | '~=' | '===')
func (p *envParser) parseMarkerOp() (markerOp, error) {
	p.skipWsp()
	// Apart from "not in", the markerOps are between one and three
	// characters and some are prefixes of each other (such as < and <=).
	// There aren't that many of them, so just start by trying the largest
	// possible and work down.
	for _, o := range markerOpsByLength {
		if p.accept(o.String()) {
			return o, nil
		}
	}
	// It may be "not in", with at least one character of whitespace in the
	// middle.
	if !p.accept("not") {
		return markerOpUnknown, p.expected("not")
	}
	if !p.skipWsp() {
		return markerOpUnknown, p.expected("whitespace, in the middle of 'not in'")
	}
	if !p.accept("in") {
		return markerOpUnknown, p.expected("in after not")
	}
	return markerOpNotIn, nil
}

// parseMarkerExpr parses a marker_expr, returning an error if it is not valid.
// A marker_expr is either two marker_var separated by a marker_op or an entire
// marker expression in parentheses.
// marker_expr  = marker_var marker_op marker_var
//
//	| wsp* '(' marker ')'
func (p *envParser) parseMarkerExpr() (marker, error) {
	// marker_var allows leading whitespace, so it is acceptable in both
	// cases and we can skip it here.
	p.skipWsp()
	if p.accept("(") {
		m, err := p.parseMarkerOr()
		if err != nil {
			return nil, err
		}
		if !p.accept(")") {
			return nil, p.expected("closing )")
		}
		return m, nil
	}
	l, err := p.parseMarkerVar()
	if err != nil {
		return nil, err
	}
	o, err := p.parseMarkerOp()
	if err != nil {
		return nil, err
	}
	r, err := p.parseMarkerVar()
	if err != nil {
		return nil, err
	}
	expr := markerExpr{
		op:    o,
		left:  l,
		right: r,
	}

	// ~= can only compare versions.
	if (l.version == nil || r.version == nil) && o == markerOpTildeEqual {
		return nil, fmt.Errorf("~= must compare versions, got %s %s %s", l, o, r)
	}

	// If it appears to be a valid version comparison, parse the operator
	// and right hand side as a constraint. === is a special case because
	// its purpose to force string comparison, see PEP 440 for details
	// (https://www.python.org/dev/peps/pep-0440/).
	if l.version != nil && r.version != nil && o != markerOpEqualEqualEqual {
		c, err := semver.PyPI.ParseConstraint(o.String() + r.value)
		if err != nil {
			return nil, err
		}
		expr.constraint = c
	}

	if (l.name == "extra" || r.name == "extra") && o != markerOpEqualEqual {
		// If extras are involved then only one comparison makes sense.
		// This is not in the grammar but it is the only expressions
		// involving extras setuptools (and therefore pip) will
		// generate.
		return nil, fmt.Errorf("extra can only be compared with '==', got: %s %s %s", l, o, r)
	}
	return expr, nil
}

// markerOr corresponds to the first case of marker_or in the grammar, which is
// two marker_and whose results will be joined by a logical OR.
type markerOr struct {
	left, right marker
}

func (mo markerOr) String() string {
	return fmt.Sprintf("(%s or %s)", mo.left, mo.right)
}

func (mo markerOr) Eval(extras map[string]bool) bool {
	return mo.left.Eval(extras) || mo.right.Eval(extras)
}

// markerAnd corresponds to the first case of a marker_and in the grammar, which
// is two marker_expr whose results will be joined by a logical AND.
type markerAnd struct {
	left, right marker
}

func (ma markerAnd) String() string {
	return fmt.Sprintf("(%s and %s)", ma.left, ma.right)
}

func (ma markerAnd) Eval(extras map[string]bool) bool {
	return ma.left.Eval(extras) && ma.right.Eval(extras)
}

// environmentVariables holds the known variables and their values.
var environmentVariables = map[string]markerVar{
	"os_name":                        platformVar("os_name"),
	"sys_platform":                   platformVar("sys_platform"),
	"platform_machine":               platformVar("platform_machine"),
	"platform_python_implementation": platformVar("platform_python_implementation"),
	"platform_release":               platformVar("platform_release"),
	"platform_system":                platformVar("platform_system"),
	"platform_version":               platformVar("platform_version"),
	"python_version":                 platformVar("python_version"),
	"python_full_version":            platformVar("python_full_version"),
	"implementation_name":            platformVar("implementation_name"),
	"implementation_version":         platformVar("implementation_version"),
	// extra is special: its value can only be known at resolution time.
	"extra": {name: "extra"},
}

// platformVar creates a markerVar for one of the pre-defined names, looking it
// up in the values generated from the canonical platform. Panics if the name is
// not defined.
func platformVar(name string) markerVar {
	value, ok := internal.Markers[name]
	if !ok {
		panic("Undefined marker variable: " + name)
	}
	return mkMarkerVar(name, value)
}

// mkMarkerVar makes a new markerVar, populating the version if the value is the
// valid PEP 440 version.
func mkMarkerVar(name, value string) markerVar {
	mv := markerVar{name: name, value: value}
	v, err := semver.PyPI.Parse(value)
	if err == nil {
		mv.version = v
	}
	return mv
}

// markerExpr is a binary comparison between two marker_var. As per
// https://www.python.org/dev/peps/pep-0508/#environment-markers the intention
// is to prefer version comparisons where possible but otherwise fall back to
// Python string comparisons.
type markerExpr struct {
	op          markerOp
	left, right markerVar
	// constraint is the set created by the operator and the right operand.
	// It is only set if both sides are valid versions and the operator is a
	// valid version comparison.
	constraint *semver.Constraint
}

func (me markerExpr) String() string {
	constraint := "nil"
	if me.constraint != nil {
		constraint = me.constraint.Set().String()
	}
	return fmt.Sprintf("(%s %s %s (%s))", me.left, me.op, me.right, constraint)
}

// Eval evaluates the expression in light of the requested extras. Where
// possible it will use a PEP 440 version comparison, otherwise it uses
// Python-like string operations. If extras are involved the only possible
// operator is ==, which actually checks whether the other operand is an exact
// match for one of the requested extras.
func (me markerExpr) Eval(extras map[string]bool) bool {
	if me.left.name == "extra" || me.right.name == "extra" {
		e := me.left.value
		if me.left.name == "extra" {
			e = me.right.value
		}
		return extras[e]
	}
	// Try a version comparison first.
	if me.constraint != nil {
		return me.constraint.MatchVersion(me.left.version)
	}
	// Fall back to Python string behaviour where possible.
	switch me.op {
	case markerOpLessEqual:
		return me.left.value <= me.right.value
	case markerOpLess:
		return me.left.value < me.right.value
	case markerOpNotEqual:
		return me.left.value != me.right.value
	case markerOpEqualEqual, markerOpEqualEqualEqual:
		return me.left.value == me.right.value
	case markerOpGreaterEqual:
		return me.left.value >= me.right.value
	case markerOpGreater:
		return me.left.value > me.right.value
	case markerOpIn:
		return strings.Contains(me.right.value, me.left.value)
	case markerOpNotIn:
		return !strings.Contains(me.right.value, me.left.value)
	default:
		panic(fmt.Errorf("unknown or invalid op: %v", me.op))
	}
}

// markerOp covers everything from marker_op in the grammar.
type markerOp byte

//go:generate stringer -type=markerOp -linecomment

const (
	markerOpUnknown markerOp = iota
	// Operators in version_cmp which are capable of comparing versions.
	markerOpLessEqual    // <=
	markerOpLess         // <
	markerOpNotEqual     // !=
	markerOpEqualEqual   // ==
	markerOpGreaterEqual // >=
	markerOpGreater      // >
	// Ops that are only defined on versions (although === is slightly unusual)
	markerOpTildeEqual      // ~=
	markerOpEqualEqualEqual // ===
	// Ops that are only defined on strings
	markerOpIn    // in
	markerOpNotIn // not in
)

// markerOpsByLength contains all the markerOps that have a fixed-length
// string representation (everything except markerOpNotIn) in descending order
// of the length of their string representation.
var markerOpsByLength = []markerOp{
	markerOpEqualEqualEqual, // the only 3 character op
	// 2 character ops.
	markerOpLessEqual,
	markerOpNotEqual,
	markerOpEqualEqual,
	markerOpGreaterEqual,
	markerOpTildeEqual,
	markerOpIn,
	// 1 character ops.
	markerOpLess,
	markerOpGreater,
}

// markerVar corresponds to marker_var in the PEP 508 grammar: either a variable
// from a predefined set of names or a literal. Both options may need to be
// treated as strings, as semver versions or as part of a semver constraint.
type markerVar struct {
	name    string // Only set if this is a variable.
	value   string
	version *semver.Version // Only set if value is a valid version.
}

func (v markerVar) String() string {
	if v.name != "" {
		return fmt.Sprintf("%s(%q)", v.name, v.value)
	}
	return fmt.Sprintf("%q", v.value)
}
