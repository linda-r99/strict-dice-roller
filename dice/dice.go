// Package dice parses dice notation such as "3d6+2" or "4d6kh3" into a
// structure that can be rolled.
//
// Parsing is strict by default: a single die separator case, no implicit
// dice counts, no leading zeros, no stray whitespace. The lenient flag
// relaxes each of those checks independently rather than switching to a
// different grammar, so strict and lenient inputs agree on everything the
// strict grammar already accepts.
package dice

import (
	"fmt"
	"strconv"
	"strings"
)

// ModKind identifies a keep/drop modifier applied to a group of dice.
type ModKind int

const (
	ModNone ModKind = iota
	ModKeepHigh
	ModKeepLow
	ModDropHigh
	ModDropLow
)

// Bounds on counts and sides. These apply in both strict and lenient mode;
// lenient only relaxes formatting rules, not resource limits.
const (
	MaxDiceCount = 1000
	MaxSides     = 10000
	MaxConstant  = 1000000
)

// DiceTerm is a single dice group, e.g. "4d6kh3", with its sign within the
// enclosing expression.
type DiceTerm struct {
	Sign     int
	Count    int
	Sides    int
	Mod      ModKind
	ModCount int
}

// ConstTerm is a flat numeric modifier, e.g. the "+2" in "1d20+2".
type ConstTerm struct {
	Sign  int
	Value int
}

// Term is exactly one of Dice or Const.
type Term struct {
	Dice  *DiceTerm
	Const *ConstTerm
}

// Expression is a parsed dice notation string, ready to roll.
type Expression struct {
	Raw   string
	Terms []Term
}

// Parse turns notation into an Expression. With lenient set to false,
// notation must match the strict grammar described in the README; any
// deviation is rejected with an error explaining what --lenient would have
// allowed.
func Parse(input string, lenient bool) (*Expression, error) {
	if input == "" {
		return nil, fmt.Errorf("empty expression")
	}

	work := input
	if strings.ContainsAny(work, " \t\n\r") {
		if !lenient {
			return nil, fmt.Errorf("strict mode: notation may not contain whitespace (got %q, try --lenient)", input)
		}
		work = strings.Join(strings.Fields(work), "")
		if work == "" {
			return nil, fmt.Errorf("empty expression")
		}
	}

	pieces, err := splitTerms(work, lenient)
	if err != nil {
		return nil, err
	}

	expr := &Expression{Raw: input}
	for _, p := range pieces {
		term, err := parseTerm(p.sign, p.text, lenient)
		if err != nil {
			return nil, err
		}
		expr.Terms = append(expr.Terms, term)
	}
	return expr, nil
}

type signedText struct {
	sign int
	text string
}

// splitTerms breaks an expression into signed chunks at top-level + and -.
// Dice notation has no parentheses or nested operators, so a plain
// character scan is enough; the only subtlety is the optional sign on the
// very first term.
func splitTerms(s string, lenient bool) ([]signedText, error) {
	sign := 1
	pos := 0
	switch s[0] {
	case '+':
		if !lenient {
			return nil, fmt.Errorf("strict mode: leading %q is redundant (try --lenient)", "+")
		}
		pos = 1
	case '-':
		sign = -1
		pos = 1
	}

	var pieces []signedText
	start := pos
	for i := pos; i < len(s); i++ {
		switch s[i] {
		case '+', '-':
			text := s[start:i]
			if text == "" {
				return nil, fmt.Errorf("empty term before position %d in %q", i, s)
			}
			pieces = append(pieces, signedText{sign: sign, text: text})
			if s[i] == '+' {
				sign = 1
			} else {
				sign = -1
			}
			start = i + 1
		}
	}
	text := s[start:]
	if text == "" {
		return nil, fmt.Errorf("expression %q ends with a dangling operator", s)
	}
	pieces = append(pieces, signedText{sign: sign, text: text})
	return pieces, nil
}

func parseTerm(sign int, text string, lenient bool) (Term, error) {
	dIdx := strings.IndexAny(text, "dD")
	if dIdx == -1 {
		return parseConstTerm(sign, text, lenient)
	}
	if text[dIdx] == 'D' && !lenient {
		return Term{}, fmt.Errorf("strict mode: die separator must be lowercase %q, not %q (try --lenient)", "d", text)
	}

	countStr := text[:dIdx]
	rest := text[dIdx+1:]

	count := 1
	if countStr == "" {
		if !lenient {
			return Term{}, fmt.Errorf("strict mode: dice count must be explicit, write %q not %q (try --lenient)", "1"+text, text)
		}
	} else {
		n, err := parseNumber(countStr, lenient, "dice count")
		if err != nil {
			return Term{}, err
		}
		count = n
	}
	if count < 1 || count > MaxDiceCount {
		return Term{}, fmt.Errorf("dice count %d is out of range (1-%d)", count, MaxDiceCount)
	}

	sidesStr, modSpec := scanDigits(rest)
	if sidesStr == "" {
		return Term{}, fmt.Errorf("%q is missing a number of sides after %q", text, "d")
	}
	sides, err := parseNumber(sidesStr, lenient, "side count")
	if err != nil {
		return Term{}, err
	}
	if sides < 2 || sides > MaxSides {
		return Term{}, fmt.Errorf("side count %d is out of range (2-%d)", sides, MaxSides)
	}

	dt := &DiceTerm{Sign: sign, Count: count, Sides: sides, Mod: ModNone}
	if modSpec != "" {
		mod, modCount, err := parseModifier(modSpec, count, lenient)
		if err != nil {
			return Term{}, err
		}
		dt.Mod = mod
		dt.ModCount = modCount
	}
	return Term{Dice: dt}, nil
}

// parseModifier handles the keep/drop suffix: kh, kl, dh, or dl followed by
// a count. "kh" alone (no count) only parses in lenient mode, as kh1.
func parseModifier(spec string, diceCount int, lenient bool) (ModKind, int, error) {
	lower := strings.ToLower(spec)
	if spec != lower && !lenient {
		return ModNone, 0, fmt.Errorf("strict mode: modifier %q must be lowercase (try --lenient)", spec)
	}
	if len(lower) < 2 {
		return ModNone, 0, fmt.Errorf("%q is not a valid modifier (expected kh, kl, dh, or dl followed by a count)", spec)
	}

	var kind ModKind
	switch lower[:2] {
	case "kh":
		kind = ModKeepHigh
	case "kl":
		kind = ModKeepLow
	case "dh":
		kind = ModDropHigh
	case "dl":
		kind = ModDropLow
	default:
		return ModNone, 0, fmt.Errorf("%q is not a valid modifier (expected kh, kl, dh, or dl followed by a count)", spec)
	}

	countStr := lower[2:]
	count := 1
	if countStr == "" {
		if !lenient {
			return ModNone, 0, fmt.Errorf("strict mode: modifier count must be explicit, write %q not %q (try --lenient)", lower+"1", spec)
		}
	} else {
		n, err := parseNumber(countStr, lenient, "modifier count")
		if err != nil {
			return ModNone, 0, err
		}
		count = n
	}
	if count < 1 || count > diceCount {
		return ModNone, 0, fmt.Errorf("modifier count %d must be between 1 and the number of dice rolled (%d)", count, diceCount)
	}
	return kind, count, nil
}

func parseConstTerm(sign int, text string, lenient bool) (Term, error) {
	n, err := parseNumber(text, lenient, "constant")
	if err != nil {
		return Term{}, err
	}
	if n < 0 || n > MaxConstant {
		return Term{}, fmt.Errorf("constant %d is out of range (0-%d)", n, MaxConstant)
	}
	return Term{Const: &ConstTerm{Sign: sign, Value: n}}, nil
}

// parseNumber parses a plain non-negative integer, rejecting leading zeros
// unless lenient is set. Dice notation has no use for signed or fractional
// numbers inside a term, so anything else is a format error.
func parseNumber(s string, lenient bool, label string) (int, error) {
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%s %q is not a plain number", label, s)
		}
	}
	if len(s) > 1 && s[0] == '0' && !lenient {
		return 0, fmt.Errorf("strict mode: %s %q has a leading zero (try --lenient)", label, s)
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a valid number: %w", label, s, err)
	}
	return n, nil
}

func scanDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i], s[i:]
}
