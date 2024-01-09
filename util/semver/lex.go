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
	"unicode/utf8"
)

type lexer struct {
	str           string // Full input.
	pos           int    // Lexical position.
	wid           int    // Width of last rune.
	allowInfinity bool   // Accept '∞' as a number.
	err           error  // First error.
}

// setError remembers the first error that occurs.
// If the argument is nil, setError is a no-op.
func (l *lexer) setError(err error) {
	if err != nil && l.err == nil {
		l.err = err
	}
}

// setErr calls setError with a decorated version of the message.
func (l *lexer) setErr(msg string) {
	if l.err == nil {
		l.err = fmt.Errorf("%s in %#q", msg, l.str)
	}
}

func (l *lexer) unexpected(typ tokType, tok string) {
	if typ == tokInvalid {
		l.setErr(fmt.Sprintf("invalid %#q", tok))
	} else {
		l.setErr("unexpected " + strings.ToLower(typ.String()))
	}
}

// next returns the next rune and advances.
func (l *lexer) next() rune {
	if l.pos >= len(l.str) {
		l.wid = 0
		return eof
	}
	r, wid := utf8.DecodeRuneInString(l.str[l.pos:])
	l.pos += wid
	l.wid = wid
	// Is an OK character? The tVS type almost works, but we also admit eof.
	if r == eof || r < 0x7F && byteType[r] == tVS {
		return r
	}
	if l.allowInfinity && r == '∞' {
		return r
	}
	// Back up so error points to it.
	l.back()
	l.setErr(fmt.Sprintf("invalid character %q", r))
	return r
}

// back backs up over the previous rune. It can back up only one rune.
func (l *lexer) back() {
	l.pos -= l.wid
	l.wid = 0
}

// peek returns the next rune, without advancing.
func (l *lexer) peek() rune {
	r := l.next()
	l.back()
	return r
}

// alphanumericOrHyphen reports whether the next rune is a letter, digit or hyphen, and, if it is, advances.
func (l *lexer) alphanumericOrHyphen() bool {
	r := l.next()
	if isAlphanumericOrHyphen(r) {
		return true
	}
	l.back()
	return false
}

// digit reports whether the next rune is a digit and, if it is, advances.
func (l *lexer) digit() bool {
	if r := l.next(); isDigit(r) {
		return true
	}
	l.back()
	return false
}

// isAlphanumericOrHyphen reports whether rune is a letter, digit or hyphen.
func isAlphanumericOrHyphen(r rune) bool {
	return r == '-' || isAlphanumeric(r)
}

// isAlphanumeric reports whether rune is a letter or digit.
func isAlphanumeric(r rune) bool {
	return isDigit(r) || isAlpha(r)
}

// isAlpha reports whether rune is a letter.
func isAlpha(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z'
}

// isDigit reports whether rune is a digit.
func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}
