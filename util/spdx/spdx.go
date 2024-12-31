// Copyright 2024 Google LLC
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

/*
Package spdx handles SPDX identifiers and license expressions.

SPDX, or Software Package Data Exchange, is a standard for specifying software
metadata in a machine-readable format.

License Expressions

This package parses expressions such as "(LGPL-2.1 OR MIT)" and can match and
manipulate them. The syntax of these expressions is documented at
https://spdx.dev/spdx-specification-21-web-version (Appendix IV).

*/
package spdx

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

//go:generate go run genlist.go -o list.gen.go

// LicenseExpression represents an SPDX License Expression,
// indicating the set of license terms that may apply to some software.
type LicenseExpression struct {
	e *compoundExpression
}

func (le *LicenseExpression) String() string {
	return le.e.String()
}

// Canon canonicalizes this license expression in place.
func (le *LicenseExpression) Canon() {
	le.e.Canon()

	// Outer-most parens are always unnecessary.
	if p := le.e.paren; p != nil {
		le.e = p
	}
}

// Valid reports whether this license expression is valid.
// This checks that all license and exception identifiers are known.
func (le *LicenseExpression) Valid() error {
	return le.e.Valid()
}

// simpleExpression represents a simple-expression production.
type simpleExpression struct {
	value string
	plus  bool // Whether the license ID was followed by a "+".
}

func (se *simpleExpression) String() string {
	s := se.value
	if se.plus {
		s += "+"
	}
	return s
}

func (se *simpleExpression) Canon() {
	if licenseIDs[se.value] {
		return
	}
	if id, ok := licenseIDIndex[strings.ToLower(se.value)]; ok {
		se.value = id
	}
}

func (se *simpleExpression) Valid() error {
	if _, ok := licenseIDIndex[strings.ToLower(se.value)]; !ok {
		return fmt.Errorf("unknown license %q", se.value)
	}
	return nil
}

type withExpression struct {
	simple    *simpleExpression
	exception string
}

func (we *withExpression) String() string {
	return we.simple.String() + " WITH " + we.exception
}

func (we *withExpression) Canon() {
	we.simple.Canon()
	if exceptionIDs[we.exception] {
		return
	}
	if id, ok := exceptionIDIndex[strings.ToLower(we.exception)]; ok {
		we.exception = id
	}
}

func (we *withExpression) Valid() error {
	if err := we.simple.Valid(); err != nil {
		return err
	}
	if _, ok := exceptionIDIndex[strings.ToLower(we.exception)]; !ok {
		return fmt.Errorf("unknown exception %q", we.exception)
	}
	return nil
}

// compoundExpression represents a compound-expression production.
type compoundExpression struct {
	// Exactly one of these will be set.
	simple *simpleExpression
	with   *withExpression
	and    []*compoundExpression // n-tuple
	or     []*compoundExpression // n-tuple
	paren  *compoundExpression
}

func (ce *compoundExpression) String() string {
	switch {
	case ce.simple != nil:
		return ce.simple.String()
	case ce.with != nil:
		return ce.with.String()
	case ce.and != nil:
		return joinList(ce.and, " AND ")
	case ce.or != nil:
		return joinList(ce.or, " OR ")
	case ce.paren != nil:
		return "(" + ce.paren.String() + ")"
	}
	panic("unreachable")
}

func joinList(list []*compoundExpression, join string) string {
	s := make([]string, 0, len(list))
	for _, ce := range list {
		s = append(s, ce.String())
	}
	return strings.Join(s, join)
}

func (ce *compoundExpression) Canon() {
	switch {
	case ce.simple != nil:
		ce.simple.Canon()
	case ce.with != nil:
		ce.with.Canon()
	case ce.and != nil:
		ce.canonList(ce.and)
	case ce.or != nil:
		ce.canonList(ce.or)
	case ce.paren != nil:
		ce.paren.Canon()

		// Only preserve parens around AND/OR.
		if ce.paren.and == nil && ce.paren.or == nil {
			*ce = *ce.paren
		}
	}
}

// conjMismatch reports whether a and b mismatch in whether they are conjunctions or disjunctions.
// That is, this reports whether a is AND and b is OR, or a is OR and b is AND.
func conjMismatch(a, b *compoundExpression) bool {
	if a.and != nil {
		return b.or != nil
	}
	if a.or != nil {
		return b.and != nil
	}
	return false
}

func (ce *compoundExpression) canonList(list []*compoundExpression) {
	for i, ice := range list {
		ice.Canon()
		if conjMismatch(ce, ice) {
			// Add parens.
			list[i] = &compoundExpression{paren: ice}
		}
	}

	// Sort list so that simple expressions come first, alphabetized.
	sort.Slice(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if (a.simple != nil) != (b.simple != nil) {
			return a.simple != nil
		}
		if a.simple != nil && b.simple != nil {
			return a.simple.value < b.simple.value
		}
		// Preserve existing order.
		return i < j
	})
}

func (ce *compoundExpression) Valid() error {
	switch {
	case ce.simple != nil:
		return ce.simple.Valid()
	case ce.with != nil:
		return ce.with.Valid()
	case ce.and != nil:
		return validList(ce.and)
	case ce.or != nil:
		return validList(ce.or)
	case ce.paren != nil:
		return ce.paren.Valid()
	}
	panic("unreachable")
}

func validList(list []*compoundExpression) error {
	for _, ce := range list {
		if err := ce.Valid(); err != nil {
			return err
		}
	}
	return nil
}

// ParseLicenseExpression parses an SPDX license expression.
func ParseLicenseExpression(s string) (*LicenseExpression, error) {
	p := &leParser{s: s}
	le, err := p.parseLicenseExpression()
	if err != nil {
		return nil, err
	}
	if rem := p.rem(); rem != "" {
		return nil, fmt.Errorf("trailing content %q", rem)
	}
	return le, nil
}

type leParser struct {
	s      string // Remaining input.
	done   bool   // Whether the parsing is finished (success or error).
	backed bool   // Whether back() was called.
	cur    leToken
}

type leToken struct {
	value string
	err   error
}

func (p *leParser) rem() string {
	if p.backed {
		return p.cur.value + p.s
	}
	return p.s
}

func (p *leParser) errorf(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	p.cur.err = err
	p.done = true
	return err
}

func (p *leParser) skipSpace() {
	i := 0
	for i < len(p.s) && p.s[i] == ' ' {
		i++
	}
	p.s = p.s[i:]
	if p.s == "" {
		p.done = true
	}
}

// idstringChar reports whether c is a valid character for an idstring terminal.
func idstringChar(c byte) bool {
	// idstring = 1*(ALPHA / DIGIT / "-" / "." )
	switch {
	case 'A' <= c && c <= 'Z':
		return true
	case 'a' <= c && c <= 'z':
		return true
	case '0' <= c && c <= '9':
		return true
	case c == '-' || c == '.':
		return true
	}
	return false
}

// advance moves the parser to the next token, which will be available in p.cur.
func (p *leParser) advance() {
	p.skipSpace()
	if p.done {
		return
	}

	p.cur.err = nil

	switch p.s[0] {
	case '(', ')', '+', ':', '/':
		// Single character symbol.
		p.cur.value, p.s = p.s[:1], p.s[1:]
		return
	}

	// The only other valid token is an `idstring`.
	i := 0
	for i < len(p.s) && idstringChar(p.s[i]) {
		i++
	}
	if i == 0 {
		p.errorf("unexpected character %q", p.s[:1])
		i++
	}
	p.cur.value, p.s = p.s[:i], p.s[i:]
}

// back steps the parser back one token. It cannot be called twice in succession.
func (p *leParser) back() {
	if p.backed {
		panic("parser backed up twice")
	}
	p.done = false
	p.backed = true
	// If an error was being recovered, we wish to ignore the error.
	// Don't do that for io.EOF since that'll be returned next.
	if p.cur.err != io.EOF {
		p.cur.err = nil
	}
}

// next returns the next token.
func (p *leParser) next() leToken {
	if p.backed || p.done {
		p.backed = false
		return p.cur
	}
	p.advance()
	if p.done && p.cur.err == nil {
		p.cur.value = ""
		p.cur.err = io.EOF
	}
	return p.cur
}

// accept reports whether the next token is as specified, and consumes it.
func (p *leParser) accept(want string) bool {
	tok := p.next()
	if tok.err == nil && tok.value == want {
		return true
	}
	p.back()
	return false
}

func (p *leParser) expect(want string) error {
	tok := p.next()
	if tok.err != nil {
		return tok.err
	}
	if tok.value != want {
		return p.errorf("got %q while expecting %q", tok.value, want)
	}
	return nil
}

// parseLicenseExpression parses a license-expression production.
//
// license-expression =  1*1(simple-expression / compound-expression)
func (p *leParser) parseLicenseExpression() (*LicenseExpression, error) {
	ce, err := p.parseCompoundExpression()
	if err != nil {
		return nil, err
	}
	return &LicenseExpression{e: ce}, nil
}

// parseCompoundExpression parses a compound-expression production.
//
// compound-expression =  1*1(simple-expression /
//		 simple-expression "WITH" license-exception-id /
//		 compound-expression "AND" compound-expression /
//		 compound-expression "OR" compound-expression ) /
//		 "(" compound-expression ")" )
//
// The order of precedence is:
//	WITH
//	AND
//	OR
func (p *leParser) parseCompoundExpression() (*compoundExpression, error) {
	return p.parseOr()
}

func (p *leParser) parseOr() (*compoundExpression, error) {
	lhs, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for {
		// There are two forms: "OR" or "/".
		// Slash is a deprecated OR equivalent.
		tok := p.next()
		if tok.err != nil || (tok.value != "OR" && tok.value != "/") {
			p.back()
			return lhs, nil
		}
		rhs, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		if lhs.or != nil {
			lhs.or = append(lhs.or, rhs)
		} else {
			lhs = &compoundExpression{or: []*compoundExpression{lhs, rhs}}
		}
	}
}

func (p *leParser) parseAnd() (*compoundExpression, error) {
	lhs, err := p.parseWith()
	if err != nil {
		return nil, err
	}
	for {
		if !p.accept("AND") {
			return lhs, nil
		}
		rhs, err := p.parseWith()
		if err != nil {
			return nil, err
		}
		if lhs.and != nil {
			lhs.and = append(lhs.and, rhs)
		} else {
			lhs = &compoundExpression{and: []*compoundExpression{lhs, rhs}}
		}
	}
}

func (p *leParser) parseWith() (*compoundExpression, error) {
	// If it starts with a paren, this is the parenthetical production.
	if p.accept("(") {
		ce, err := p.parseCompoundExpression()
		if err != nil {
			return nil, err
		}
		if err := p.expect(")"); err != nil {
			return nil, err
		}
		return &compoundExpression{paren: ce}, nil
	}

	se, err := p.parseSimpleExpression()
	if err != nil {
		return nil, err
	}
	if !p.accept("WITH") {
		return &compoundExpression{simple: se}, nil
	}
	tok := p.next()
	if tok.err != nil {
		return nil, tok.err
	}
	return &compoundExpression{with: &withExpression{se, tok.value}}, nil
}

// parseSimpleExpression parses a simple-expression production.
//
// simple-expression = license-id / license-id”+” / license-ref
func (p *leParser) parseSimpleExpression() (*simpleExpression, error) {
	tok := p.next()
	if tok.err != nil {
		return nil, tok.err
	}
	se := &simpleExpression{value: tok.value}

	// Look for trailing "+".
	if p.accept("+") {
		se.plus = true
	}

	return se, nil
}
