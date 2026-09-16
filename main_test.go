package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"strict-dice-roller/dice"
)

func TestToJSONTermConst(t *testing.T) {
	cases := []struct {
		name     string
		term     dice.TermResult
		wantSign int
		wantVal  int
		wantSub  int
	}{
		{"positive", dice.TermResult{Const: &dice.ConstTerm{Sign: 1, Value: 5}}, 1, 5, 5},
		{"negative", dice.TermResult{Const: &dice.ConstTerm{Sign: -1, Value: 5}}, -1, 5, -5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			jt := toJSONTerm(c.term)
			if jt.Type != "const" {
				t.Errorf("Type = %q, want %q", jt.Type, "const")
			}
			if jt.Sign != c.wantSign || jt.Value != c.wantVal || jt.Subtotal != c.wantSub {
				t.Errorf("got sign=%d value=%d subtotal=%d, want sign=%d value=%d subtotal=%d",
					jt.Sign, jt.Value, jt.Subtotal, c.wantSign, c.wantVal, c.wantSub)
			}
		})
	}
}

func TestToJSONTermDiceNoModifier(t *testing.T) {
	tr := dice.TermResult{
		Dice:     &dice.DiceTerm{Sign: 1, Count: 3, Sides: 6, Mod: dice.ModNone},
		Chains:   [][]int{{4}, {2}, {6}},
		Subtotal: 12,
	}
	jt := toJSONTerm(tr)
	if jt.Type != "dice" {
		t.Errorf("Type = %q, want %q", jt.Type, "dice")
	}
	if jt.Chains == nil || len(jt.Chains) != 3 {
		t.Errorf("Chains = %v, want 3 entries", jt.Chains)
	}
	if jt.Kept != nil || jt.Dropped != nil {
		t.Errorf("Kept=%v Dropped=%v, want both nil when there's no modifier", jt.Kept, jt.Dropped)
	}
	if jt.Mod != "" {
		t.Errorf("Mod = %q, want empty", jt.Mod)
	}
}

func TestToJSONTermDiceWithModifier(t *testing.T) {
	tr := dice.TermResult{
		Dice:     &dice.DiceTerm{Sign: 1, Count: 4, Sides: 6, Mod: dice.ModKeepHigh, ModCount: 3},
		Chains:   [][]int{{5}, {3}, {6}, {1}},
		Kept:     [][]int{{5}, {3}, {6}},
		Dropped:  [][]int{{1}},
		Subtotal: 14,
	}
	jt := toJSONTerm(tr)
	if jt.Chains != nil {
		t.Errorf("Chains = %v, want nil when a modifier dropped dice", jt.Chains)
	}
	if len(jt.Kept) != 3 || len(jt.Dropped) != 1 {
		t.Errorf("Kept=%v Dropped=%v, want 3 and 1", jt.Kept, jt.Dropped)
	}
	if jt.Mod != "kh" || jt.ModCount != 3 {
		t.Errorf("Mod=%q ModCount=%d, want kh and 3", jt.Mod, jt.ModCount)
	}
}

func TestToJSONTermFudgeAndExplode(t *testing.T) {
	tr := dice.TermResult{
		Dice:     &dice.DiceTerm{Sign: -1, Count: 4, Fudge: true, Explode: true, Mod: dice.ModNone},
		Chains:   [][]int{{1}, {-1}, {0}, {1}},
		Subtotal: -1,
	}
	jt := toJSONTerm(tr)
	if !jt.Fudge {
		t.Error("Fudge = false, want true")
	}
	if !jt.Explode {
		t.Error("Explode = false, want true")
	}
	if jt.Sign != -1 {
		t.Errorf("Sign = %d, want -1", jt.Sign)
	}
	if jt.Sides != 0 {
		t.Errorf("Sides = %d, want 0 for a fudge die", jt.Sides)
	}
}

// TestPrintResultJSONRoundTrips checks the full path from a dice.Result to
// the bytes actually written to stdout, not just the intermediate jsonTerm
// conversion, since that's what a script consuming --format=json depends on.
func TestPrintResultJSONRoundTrips(t *testing.T) {
	result := dice.Result{
		Total: 14,
		Terms: []dice.TermResult{
			{
				Dice:     &dice.DiceTerm{Sign: 1, Count: 4, Sides: 6, Mod: dice.ModKeepHigh, ModCount: 3},
				Kept:     [][]int{{5}, {3}, {6}},
				Dropped:  [][]int{{1}},
				Subtotal: 14,
			},
		},
	}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	printResultJSON(result)
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("reading captured output: %v", err)
	}

	var got jsonResult
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output %q is not valid JSON: %v", buf.String(), err)
	}
	if got.Total != 14 {
		t.Errorf("Total = %d, want 14", got.Total)
	}
	if len(got.Terms) != 1 {
		t.Fatalf("len(Terms) = %d, want 1", len(got.Terms))
	}
	term := got.Terms[0]
	if term.Type != "dice" || term.Mod != "kh" || term.ModCount != 3 {
		t.Errorf("term = %+v, want type dice, mod kh, mod_count 3", term)
	}
	if term.Chains != nil {
		t.Errorf("Chains = %v, want omitted when a modifier dropped dice", term.Chains)
	}
	if len(term.Kept) != 3 || len(term.Dropped) != 1 {
		t.Errorf("term.Kept=%v term.Dropped=%v, want 3 and 1", term.Kept, term.Dropped)
	}
}

func TestCompletionScriptUnknownShell(t *testing.T) {
	if _, err := completionScript("fish"); err == nil {
		t.Error("completionScript(\"fish\") = nil error, want an error for an unsupported shell")
	}
}

func TestCompletionScriptKnownShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		script, err := completionScript(shell)
		if err != nil {
			t.Fatalf("completionScript(%q): %v", shell, err)
		}
		for _, name := range []string{"--lenient", "--seed", "--count", "--quiet", "--format", "--completion"} {
			if !strings.Contains(script, name) {
				t.Errorf("%s completion missing flag %q", shell, name)
			}
		}
	}
}

func TestBashCompletionRegistersFunction(t *testing.T) {
	script := bashCompletion()
	if !strings.Contains(script, "complete -F _diceroll_complete diceroll") {
		t.Errorf("bash completion doesn't register with complete: %s", script)
	}
	if !strings.Contains(script, `compgen -W "text json"`) {
		t.Error("bash completion doesn't offer text/json values for --format")
	}
}

func TestZshCompletionIsWellFormed(t *testing.T) {
	script := zshCompletion()
	if !strings.HasPrefix(script, "#compdef diceroll\n") {
		t.Error("zsh completion missing #compdef header")
	}
	if !strings.Contains(script, "_arguments") {
		t.Error("zsh completion doesn't call _arguments")
	}
	if !strings.Contains(script, "(text json)") {
		t.Error("zsh completion doesn't offer text/json values for --format")
	}
	if strings.Count(script, "'")%2 != 0 {
		t.Error("zsh completion has an unbalanced single quote")
	}
}

func TestZshEscape(t *testing.T) {
	got := zshEscape(`plain text`)
	want := `plain text`
	if got != want {
		t.Errorf("zshEscape(%q) = %q, want %q", "plain text", got, want)
	}

	got = zshEscape(`a [b]: c\d`)
	want = `a \[b\]\: c\\d`
	if got != want {
		t.Errorf("zshEscape produced %q, want %q", got, want)
	}
}
