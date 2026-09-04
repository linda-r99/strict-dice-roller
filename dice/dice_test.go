package dice

import (
	"reflect"
	"strings"
	"testing"
)

func diceTerm(sign, count, sides int, mod ModKind, modCount int) Term {
	return Term{Dice: &DiceTerm{Sign: sign, Count: count, Sides: sides, Mod: mod, ModCount: modCount}}
}

func constTerm(sign, value int) Term {
	return Term{Const: &ConstTerm{Sign: sign, Value: value}}
}

func TestParseValid(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		lenient bool
		want    []Term
	}{
		{"single die", "1d6", false, []Term{diceTerm(1, 1, 6, ModNone, 0)}},
		{"dice plus constant", "3d6+2", false, []Term{diceTerm(1, 3, 6, ModNone, 0), constTerm(1, 2)}},
		{"keep high", "4d6kh3", false, []Term{diceTerm(1, 4, 6, ModKeepHigh, 3)}},
		{"drop low", "4d6dl1", false, []Term{diceTerm(1, 4, 6, ModDropLow, 1)}},
		{"drop high", "4d6dh1", false, []Term{diceTerm(1, 4, 6, ModDropHigh, 1)}},
		{"keep low", "4d6kl1", false, []Term{diceTerm(1, 4, 6, ModKeepLow, 1)}},
		{"negative dice term then positive", "-1d4+2d6", false, []Term{diceTerm(-1, 1, 4, ModNone, 0), diceTerm(1, 2, 6, ModNone, 0)}},
		{"dice minus constant", "1d20-1", false, []Term{diceTerm(1, 1, 20, ModNone, 0), constTerm(-1, 1)}},
		{"leading negative constant", "-5", false, []Term{constTerm(-1, 5)}},
		{"min sides", "1d2", false, []Term{diceTerm(1, 1, 2, ModNone, 0)}},
		{"max count and sides", "1000d10000", false, []Term{diceTerm(1, 1000, 10000, ModNone, 0)}},
		{"max constant", "1000000", false, []Term{constTerm(1, 1000000)}},

		{"lenient implicit count", "d6", true, []Term{diceTerm(1, 1, 6, ModNone, 0)}},
		{"lenient uppercase d", "D6", true, []Term{diceTerm(1, 1, 6, ModNone, 0)}},
		{"lenient whitespace", " 2d6 + 3 ", true, []Term{diceTerm(1, 2, 6, ModNone, 0), constTerm(1, 3)}},
		{"lenient leading zeros", "01d06", true, []Term{diceTerm(1, 1, 6, ModNone, 0)}},
		{"lenient uppercase modifier", "4d6KH3", true, []Term{diceTerm(1, 4, 6, ModKeepHigh, 3)}},
		{"lenient implicit modifier count", "4d6kh", true, []Term{diceTerm(1, 4, 6, ModKeepHigh, 1)}},
		{"lenient leading plus", "+1d6", true, []Term{diceTerm(1, 1, 6, ModNone, 0)}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expr, err := Parse(c.input, c.lenient)
			if err != nil {
				t.Fatalf("Parse(%q, %v) returned error: %v", c.input, c.lenient, err)
			}
			if !reflect.DeepEqual(expr.Terms, c.want) {
				t.Errorf("Parse(%q, %v) terms = %+v, want %+v", c.input, c.lenient, expr.Terms, c.want)
			}
			if expr.Raw != c.input {
				t.Errorf("Parse(%q, %v) Raw = %q, want %q", c.input, c.lenient, expr.Raw, c.input)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		lenient bool
		want    string // substring the error message must contain
	}{
		{"empty expression", "", false, "empty expression"},
		{"whitespace-only lenient collapses to empty", "   ", true, "empty expression"},
		{"strict whitespace rejected", "2d6 + 3", false, "--lenient"},
		{"strict leading plus rejected", "+1d6", false, "--lenient"},
		{"strict implicit count rejected", "d6", false, "--lenient"},
		{"strict uppercase d rejected", "D6", false, "--lenient"},
		{"strict uppercase modifier rejected", "4d6KH3", false, "--lenient"},
		{"strict implicit modifier count rejected", "4d6kh", false, "--lenient"},
		{"strict leading zero on count rejected", "01d6", false, "--lenient"},
		{"strict leading zero on constant rejected", "007", false, "--lenient"},

		{"empty term between operators", "1d6++2", false, "empty term"},
		{"dangling trailing operator", "1d6+", false, "dangling operator"},
		{"dice count zero", "0d6", false, "out of range"},
		{"dice count too high", "1001d6", false, "out of range"},
		{"dice count too high even lenient", "1001d6", true, "out of range"},
		{"sides too low", "1d1", false, "out of range"},
		{"sides too high", "1d10001", false, "out of range"},
		{"missing sides", "1d", false, "missing a number of sides"},
		{"non-numeric sides", "1dabc", false, "missing a number of sides"},
		{"unknown modifier", "4d6xy3", false, "not a valid modifier"},
		{"modifier count zero", "4d6kh0", false, "must be between 1 and the number of dice rolled"},
		{"modifier count exceeds dice", "4d6kh5", false, "must be between 1 and the number of dice rolled"},
		{"non-numeric dice count", "abcd6", false, "not a plain number"},
		{"non-numeric modifier count", "4d6khx", false, "not a plain number"},
		{"constant too high", "1000001", false, "out of range"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.input, c.lenient)
			if err == nil {
				t.Fatalf("Parse(%q, %v) succeeded, want error containing %q", c.input, c.lenient, c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("Parse(%q, %v) error = %q, want it to contain %q", c.input, c.lenient, err.Error(), c.want)
			}
		})
	}
}
