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
	"unicode/utf8"
)

// The constraint language has the property that the various tokens are _almost_
// identified by distinct character sets. We use this to tokenize the input
// without needing to parse tricky things like versions, allowing us to treat
// version strings and wildcard strings as terminals in the grammar. The
// "almost" refers to two cases:
// 1. Hyphen is an operator and a version character. However, the grammar
// requires that the operator is bound by spaces, so that's easy.
// 2. Versions and wildcards can both contain x and X. We handle this by treating
// * as a version character and then doing a quick scan to discriminate.

// tokType classifies the tokens according to the characters within.
// The parser uses the type to guide the parse.
type tokType int

//go:generate stringer -type tokType -trimprefix tok

// Token types.
const (
	tokInvalid       tokType = iota
	tokInternalError         // Can't happen.
	tokEmpty                 // Used internally. Empty string (no operator).
	tokEqual
	tokGreater
	tokGreaterEqual
	tokLess
	tokLessEqual
	tokNotEqual
	tokCaret
	tokTilde
	tokBacon
	tokComma
	tokOr
	tokHyphen
	tokLbracket // Maven, NuGet only.
	tokRbracket // Maven, NuGet only.
	tokVersion
	tokWildcard
	tokEOF
)

const (
	tXX = iota // Unknown/illegal char.
	tWS        // Space.
	tVS        // Can be in a version string or wildcard string. Includes hyphens.
	tOP        // Part of an operator like '>=' or ','.
	tBR        // Bracket () [], Maven only.
)

var byteType = [...]uint8{
	tXX, tXX, tXX, tXX, tXX, tXX, tXX, tXX, // 0x00-0x07
	tXX, tWS, tXX, tXX, tXX, tXX, tXX, tXX, // 0x08-0x0f
	tXX, tXX, tXX, tXX, tXX, tXX, tXX, tXX, // 0x10-0x17
	tXX, tXX, tXX, tXX, tXX, tXX, tXX, tXX, // 0x18-0x1f
	tWS, tOP, tXX, tXX, tXX, tXX, tXX, tXX, // ⎵ ! " # $ % & '
	tBR, tBR, tVS, tVS, tOP, tVS, tVS, tXX, // ( ) * + , - . /
	tVS, tVS, tVS, tVS, tVS, tVS, tVS, tVS, // 0 1 2 3 4 5 6 7
	tVS, tVS, tXX, tXX, tOP, tOP, tOP, tXX, // 8 9 : ; < = > ?
	tXX, tVS, tVS, tVS, tVS, tVS, tVS, tVS, // @ A B C D E F G
	tVS, tVS, tVS, tVS, tVS, tVS, tVS, tVS, // H I J K L M N O
	tVS, tVS, tVS, tVS, tVS, tVS, tVS, tVS, // P Q R S T U V W
	tVS, tVS, tVS, tBR, tXX, tBR, tOP, tXX, // X Y Z [ \ ] ^ _
	tXX, tVS, tVS, tVS, tVS, tVS, tVS, tVS, // ` a b c d e f g
	tVS, tVS, tVS, tVS, tVS, tVS, tVS, tVS, // h i j k l m n o
	tVS, tVS, tVS, tVS, tVS, tVS, tVS, tVS, // p q r s t u v w
	tVS, tVS, tVS, tXX, tOP, tXX, tOP, tXX, // x y z { | } ~ del
}

var operators = []map[string]tokType{
	DefaultSystem: {
		"=":  tokEqual,
		">":  tokGreater,
		">=": tokGreaterEqual,
		"<":  tokLess,
		"<=": tokLessEqual,
		"^":  tokCaret,
		"~":  tokTilde,
		"~>": tokBacon,
		",":  tokComma,
		"||": tokOr,
		"-":  tokHyphen,
	},

	Cargo: {
		"=":  tokEqual,
		">":  tokGreater,
		">=": tokGreaterEqual,
		"<":  tokLess,
		"<=": tokLessEqual,
		"^":  tokCaret,
		"~":  tokTilde,
		",":  tokComma,
	},

	NPM: {
		"=":  tokEqual,
		">":  tokGreater,
		">=": tokGreaterEqual,
		"<":  tokLess,
		"<=": tokLessEqual,
		"^":  tokCaret,
		"~":  tokTilde,
		"~>": tokTilde, // NPM accepts ~> but the implementation is just ~.
		"||": tokOr,
		"-":  tokHyphen,
	},

	Go: {},

	Maven: {
		",": tokComma,
	},

	NuGet: {
		",": tokComma,
	},

	PyPI: {
		"==": tokEqual,
		">":  tokGreater,
		">=": tokGreaterEqual,
		"<":  tokLess,
		"<=": tokLessEqual,
		"!=": tokNotEqual,
		"~=": tokBacon,
		",":  tokComma,
	},

	RubyGems: {
		"=":  tokEqual,
		">":  tokGreater,
		">=": tokGreaterEqual,
		"<":  tokLess,
		"<=": tokLessEqual,
		"!=": tokNotEqual,
		"~>": tokBacon,
		",":  tokComma,
	},
}

func (sys System) typeOf(r rune) uint8 {
	// Special cases.
	if r == '_' && sys == Maven {
		return tVS
		// TODO: is + also tVS in Maven?
	}
	if r == '+' && sys == RubyGems {
		return tXX
	}
	if r >= 0x7F {
		return tXX
	}
	return byteType[r]
}

// token scans for the token starting the string. It skips leading
// space and returns the token's type, its contents, and the number
// of bytes consumed to the end of the token.
// If the token is of type tokVersion or tokWildcard, it still needs
// to be checked for correctness.
func (sys System) token(str string) (tokType, string, int) {
	// Skip spaces.
	var i int
	for i = 0; i < len(str) && str[i] < 0x7F && byteType[str[i]] == tWS; i++ {
	}
	if i == len(str) {
		return tokEOF, "", i
	}
	start := i
	r, wid := utf8.DecodeRuneInString(str[start:])
	i += wid
	typ := sys.typeOf(r)
	if typ == tXX {
		return tokInvalid, str[start:i], i
	}
	opSet := operators[sys]
	// Loop as long as the type of the rune matches what we started with.
	for ; ; i += wid {
		r, wid = utf8.DecodeRuneInString(str[i:])
		// Lovely case for PyPI epoch: 1!1.2.3.
		if r == '!' && sys == PyPI && i > 0 && typ == tVS {
			continue
		}
		if sys.typeOf(r) != typ {
			break
		}
		// If an operator or bracket, take the longest valid operator.
		if typ == tOP || typ == tBR {
			if opSet[str[start:i+wid]] == tokInvalid {
				break
			}
		}
	}
	// We have the token. Discover the token type.
	tok := str[start:i]
	switch typ {
	case tOP:
		// Unary constraint comparison operator.
		return opSet[tok], tok, i
	case tVS:
		if tok == "-" {
			// Some systems don't support ranges, in which case this returns tokInvalid.
			return opSet[tok], tok, i
		}
		// It's a version or a wildcard.
		// The wildcard char must occur in the numerical section.
		// We only try to tell versions and wildcards apart, not
		// guarantee that the return value has valid format.
		numDots := 0
		start := true
		for _, r := range tok {
			if start && r == 'v' {
				// Go accepts one leading 'v', but NPM accepts v1.0, vv1.0, vvvvvvvv1.0...
				if sys == Go {
					start = false
				}
				continue
			}
			start = false
			if sys.validWildcard(r) {
				return tokWildcard, tok, i
			}
			switch {
			case '0' <= r && r <= '9':
				continue
			case r == '.':
				numDots++
				if numDots >= 3 {
					return tokVersion, tok, i
				}
				continue
			case r == '_' && sys == Maven:
				continue
			}
			// Some other character; we're past the numbers.
			break
		}
		return tokVersion, tok, i
	case tBR:
		if sys == Maven || sys == NuGet {
			if tok == "(" || tok == "[" {
				return tokLbracket, tok, i
			}
			return tokRbracket, tok, i
		}
		return tokInvalid, tok, i
	}
	return tokInternalError, tok, i
}

func (sys System) validWildcard(r rune) bool {
	switch sys {
	case DefaultSystem, Cargo, NPM:
		return r == 'x' || r == 'X' || r == '*'
	case NuGet, PyPI:
		return r == '*'
	}
	return false
}
