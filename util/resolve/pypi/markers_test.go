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

import (
	"reflect"
	"testing"

	"deps.dev/util/semver"
)

func TestParseMarkerVar(t *testing.T) {
	mvp := func(name, version string) *markerVar {
		mv := mkMarkerVar(name, version)
		return &mv
	}
	cases := []struct {
		in  string
		out *markerVar
	}{
		{in: "os_name", out: mvp("os_name", "posix")},
		{in: "  sys_platform", out: mvp("sys_platform", "linux")},
		{in: `'literal"'`, out: mvp("", `literal"`)},
		{in: `	"he11o!"`, out: mvp("", "he11o!")},
		{in: `"	' "`, out: mvp("", "\t' ")},
		// Valid versions should turn into semver versions.
		{in: "'1.2.3'", out: mvp("", "1.2.3")},

		// Unknown names with no quotes are an error
		{in: "platform_maschine"},
		{in: "bad_name"},
		// As are incorrectly terminated strings.
		{in: "'hello"},
		{in: `"hi'`},
	}

	for _, c := range cases {
		p := envParser{input: c.in}
		got, err := p.parseMarkerVar()
		if err != nil {
			if c.out != nil {
				t.Errorf("parseMarkerVar(%q) = error: %v, want: %v", c.in, err, *c.out)
			}
			continue
		}
		if c.out == nil {
			t.Errorf("parseMarkerVar(%q) = %v, want: error", c.in, got)
			continue
		}
		if !reflect.DeepEqual(got, *c.out) {
			t.Errorf("parseMarkerVar(%q) = %v, want: %v", c.in, got, *c.out)
			continue
		}
	}
}

func TestParseMarkerOp(t *testing.T) {
	cases := []struct {
		in  string
		out markerOp
	}{
		{in: "<", out: markerOpLess},
		{in: "<=", out: markerOpLessEqual},
		{in: "===", out: markerOpEqualEqualEqual},
		{in: " ==", out: markerOpEqualEqual},
		{in: "not in", out: markerOpNotIn},
		{in: "not\t in", out: markerOpNotIn},

		{in: "~"},
		{in: "hello"},
		{in: ""},
		{in: "not "},
		{in: "notin"},
	}

	for _, c := range cases {
		p := envParser{input: c.in}
		got, err := p.parseMarkerOp()
		if err != nil {
			if c.out != markerOpUnknown {
				t.Errorf("parseMarkerOp(%q) = error: %v, want: %v", c.in, err, c.out)
			}
			continue
		}
		if c.out == markerOpUnknown {
			t.Errorf("parseMarkerOp(%q) = %v, want: error", c.in, got)
			continue
		}
		if got != c.out {
			t.Errorf("parseMarkerOp(%q) = %v, want: %v", c.in, got, c.out)
		}
	}
}

func TestParseMarkerExpr(t *testing.T) {
	cases := []struct {
		in  string
		out markerExpr
	}{{
		in:  `extra == "test"`,
		out: markerExpr{op: markerOpEqualEqual, left: markerVar{name: "extra"}, right: markerVar{value: "test"}},
	}, {
		in: "python_version ~='3.8.1'",
		out: markerExpr{
			op:         markerOpTildeEqual,
			left:       environmentVariables["python_version"],
			right:      mkMarkerVar("", "3.8.1"),
			constraint: constraint(t, "~=3.8.1"),
		},
	}, {
		in: "'a' not   in implementation_name",
		out: markerExpr{
			op:    markerOpNotIn,
			left:  markerVar{value: "a"},
			right: environmentVariables["implementation_name"],
		},
	}, {
		in: `(extra == "test")`,
		out: markerExpr{
			op:    markerOpEqualEqual,
			left:  environmentVariables["extra"],
			right: markerVar{value: "test"},
		},
	}, {
		in: "'1' === '2'",
		out: markerExpr{
			op:    markerOpEqualEqualEqual,
			left:  mkMarkerVar("", "1"),
			right: mkMarkerVar("", "2"),
			// No constraint, because === can't compare versions.
		},
	}, {
		in: "'1' == '2'",
		out: markerExpr{
			op:         markerOpEqualEqual,
			left:       mkMarkerVar("", "1"),
			right:      mkMarkerVar("", "2"),
			constraint: constraint(t, "==2"),
		},
	}, {
		in: `extra != "test"`,
	}, {
		in: "python_version ~= 'hi'",
	}}

	for _, c := range cases {
		p := envParser{input: c.in}
		got, err := p.parseMarkerExpr()
		if err != nil {
			if c.out.op != markerOpUnknown {
				t.Errorf("parseMarkerExpr(%q) = error: %v, want: %v", c.in, err, c.out)
			}
			continue
		}
		if c.out.op == markerOpUnknown {
			t.Errorf("parseMarkerExpr(%q) = %v, want: error", c.in, got)
			continue
		}
		if !reflect.DeepEqual(got, c.out) {
			t.Errorf("parseMarkerExpr(%q) = %v, want: %v", c.in, got, c.out)
		}
	}
}

func TestParseMarkerAnd(t *testing.T) {
	cases := []struct {
		in  string
		out marker
	}{{
		// falls back to just an expression
		in: "extra == 'test'",
		out: markerExpr{
			op:    markerOpEqualEqual,
			left:  markerVar{name: "extra"},
			right: markerVar{value: "test"},
		},
	}, {
		in: "extra == 'test' and sys_platform != 'linux'",
		out: markerAnd{
			left: markerExpr{
				op:    markerOpEqualEqual,
				left:  markerVar{name: "extra"},
				right: markerVar{value: "test"},
			},
			right: markerExpr{
				op:    markerOpNotEqual,
				left:  environmentVariables["sys_platform"],
				right: markerVar{value: "linux"},
			},
		},
	}, {
		in: "'a'inos_nameandpython_version=='b'",
		out: markerAnd{
			left: markerExpr{
				op:    markerOpIn,
				left:  markerVar{value: "a"},
				right: environmentVariables["os_name"],
			},
			right: markerExpr{
				op:    markerOpEqualEqual,
				left:  environmentVariables["python_version"],
				right: markerVar{value: "b"},
			},
		},
	}}

	for _, c := range cases {
		p := envParser{input: c.in}
		got, err := p.parseMarkerAnd()
		if err != nil {
			if c.out != nil {
				t.Errorf("parseMarkerAnd(%q) = error: %v, want: %v", c.in, err, c.out)
			}
			continue
		}
		if c.out == nil {
			t.Errorf("parseMarkerAnd(%q) = %v, want: error", c.in, got)
			continue
		}
		if !reflect.DeepEqual(got, c.out) {
			t.Errorf("parseMarkerAnd(%q) = %v, want: %v", c.in, got, c.out)
		}
	}
}

func TestParseMarkerOr(t *testing.T) {
	cases := []struct {
		in  string
		out marker
	}{{
		in:  "extra == 'test'",
		out: markerExpr{op: markerOpEqualEqual, left: markerVar{name: "extra"}, right: markerVar{value: "test"}},
	}, {
		in: "extra == 'test' or sys_platform > 'linux'",
		out: markerOr{
			left: markerExpr{
				op:    markerOpEqualEqual,
				left:  markerVar{name: "extra"},
				right: markerVar{value: "test"},
			},
			right: markerExpr{
				op:    markerOpGreater,
				left:  environmentVariables["sys_platform"],
				right: markerVar{value: "linux"},
			},
		},
	}, {
		in: "extra == 'test' and sys_platform != 'linux' or python_version ~= '3.8' and '1' > '2'",
		out: markerOr{
			left: markerAnd{
				left: markerExpr{
					op:    markerOpEqualEqual,
					left:  markerVar{name: "extra"},
					right: markerVar{value: "test"},
				},
				right: markerExpr{
					op:    markerOpNotEqual,
					left:  environmentVariables["sys_platform"],
					right: markerVar{value: "linux"},
				},
			},
			right: markerAnd{
				left: markerExpr{
					op:         markerOpTildeEqual,
					left:       environmentVariables["python_version"],
					right:      mkMarkerVar("", "3.8"),
					constraint: constraint(t, "~=3.8"),
				},
				right: markerExpr{
					op:         markerOpGreater,
					left:       mkMarkerVar("", "1"),
					right:      mkMarkerVar("", "2"),
					constraint: constraint(t, ">2"),
				},
			},
		},
	}, {
		in: "(extra == 'test')",
		out: markerExpr{
			op:    markerOpEqualEqual,
			left:  markerVar{name: "extra"},
			right: markerVar{value: "test"},
		},
	},
		{
			in: "extra == 'test' and sys_platform != 'linux' and python_version ~= '3.7' or '1' > '2'",
			out: markerOr{
				left: markerAnd{
					left: markerExpr{
						op:    markerOpEqualEqual,
						left:  markerVar{name: "extra"},
						right: markerVar{value: "test"},
					},
					right: markerAnd{
						left: markerExpr{
							op:    markerOpNotEqual,
							left:  environmentVariables["sys_platform"],
							right: markerVar{value: "linux"},
						},
						right: markerExpr{
							op:         markerOpTildeEqual,
							left:       environmentVariables["python_version"],
							right:      mkMarkerVar("", "3.7"),
							constraint: constraint(t, "~=3.7"),
						},
					},
				},
				right: markerExpr{
					op:         markerOpGreater,
					left:       mkMarkerVar("", "1"),
					right:      mkMarkerVar("", "2"),
					constraint: constraint(t, ">2"),
				},
			},
		}, {
			in: "(extra == 'test' and sys_platform != 'linux') and python_version ~= '3.7' or '1' > '2'",
			out: markerOr{
				left: markerAnd{
					left: markerAnd{
						left: markerExpr{
							op:    markerOpEqualEqual,
							left:  markerVar{name: "extra"},
							right: markerVar{value: "test"},
						},
						right: markerExpr{
							op:    markerOpNotEqual,
							left:  environmentVariables["sys_platform"],
							right: markerVar{value: "linux"},
						},
					},
					right: markerExpr{
						op:         markerOpTildeEqual,
						left:       environmentVariables["python_version"],
						right:      mkMarkerVar("", "3.7"),
						constraint: constraint(t, "~=3.7"),
					},
				},
				right: markerExpr{
					op:         markerOpGreater,
					left:       mkMarkerVar("", "1"),
					right:      mkMarkerVar("", "2"),
					constraint: constraint(t, ">2"),
				},
			},
		}, {
			in: "extra == 'test' or sys_platform != 'linux' and python_version ~= '3.7' or '1' > '2'",
			out: markerOr{
				left: markerExpr{
					op:    markerOpEqualEqual,
					left:  markerVar{name: "extra"},
					right: markerVar{value: "test"},
				},
				right: markerOr{
					left: markerAnd{
						left: markerExpr{
							op:    markerOpNotEqual,
							left:  environmentVariables["sys_platform"],
							right: markerVar{value: "linux"},
						},

						right: markerExpr{
							op:         markerOpTildeEqual,
							left:       environmentVariables["python_version"],
							right:      mkMarkerVar("", "3.7"),
							constraint: constraint(t, "~=3.7"),
						},
					},
					right: markerExpr{
						op:         markerOpGreater,
						left:       mkMarkerVar("", "1"),
						right:      mkMarkerVar("", "2"),
						constraint: constraint(t, ">2"),
					},
				},
			},
		}, {
			in: "(python_version < \"3.7\" and (platform_python_implementation == \"CPython\" and platform_system != \"Windows\")) and extra == 'test'",
			out: markerAnd{
				left: markerAnd{
					left: markerExpr{
						op:         markerOpLess,
						left:       environmentVariables["python_version"],
						right:      mkMarkerVar("", "3.7"),
						constraint: constraint(t, "<3.7"),
					},
					right: markerAnd{
						left: markerExpr{
							op:    markerOpEqualEqual,
							left:  environmentVariables["platform_python_implementation"],
							right: markerVar{value: "CPython"},
						},
						right: markerExpr{
							op:    markerOpNotEqual,
							left:  environmentVariables["platform_system"],
							right: markerVar{value: "Windows"},
						},
					},
				},
				right: markerExpr{
					op:    markerOpEqualEqual,
					left:  markerVar{name: "extra"},
					right: markerVar{value: "test"},
				},
			},
		},
	}

	for _, c := range cases {
		p := envParser{input: c.in}
		got, err := p.parseMarkerOr()
		if err != nil {
			if c.out != nil {
				t.Errorf("parseMarkerOr(%q) = error: %v, want: %v", c.in, err, c.out)
			}
			continue
		}
		if c.out == nil {
			t.Errorf("parseMarkerOr(%q) = %v, want: error", c.in, got)
			continue
		}
		if !reflect.DeepEqual(got, c.out) {
			t.Errorf("parseMarkerAnd(%q) = \n%v, want: \n%v", c.in, got, c.out)
		}
	}
}

func TestMarkerEval(t *testing.T) {
	cases := []struct {
		in     string
		out    bool
		extras []string
	}{{
		in:  "'windows' != 'linux'",
		out: true,
	}, {
		in:  "'python' in 'adder,asp,mamba'",
		out: false,
	}, {
		in:  "'x' not in 'yyy' or '1' <= '1'",
		out: true,
	}, {
		in:  "'x' not in 'yyy' and '1' > '2'",
		out: false,
	}, {
		in:  "extra == 'test'",
		out: false,
	}, {
		in:     "extra == 'test'",
		out:    true,
		extras: []string{"test", "doc"},
	}, {
		in:     "extra == 'test'",
		out:    false,
		extras: []string{"doc"},
	}, {
		in:  "python_version ~= '3.7' or '1' === '2'",
		out: true,
	}}

	for _, c := range cases {
		p := envParser{input: c.in}
		extras := make(map[string]bool)
		for _, e := range c.extras {
			extras[e] = true
		}
		m, err := p.parseMarkerOr()
		if err != nil {
			t.Errorf("parseMarkerOr(%q) = error: %v, want success", c.in, err)
			continue
		}
		if got := m.Eval(extras); got != c.out {
			t.Errorf("%q == %v, want: %v", c.in, got, c.out)
		}
	}
}

func constraint(t *testing.T, s string) *semver.Constraint {
	c, err := semver.PyPI.ParseConstraint(s)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
