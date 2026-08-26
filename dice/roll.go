package dice

import (
	"math/rand"
	"sort"
)

// TermResult holds one term's raw rolls (if any) and its signed
// contribution to the total, for building a readable breakdown.
type TermResult struct {
	Dice     *DiceTerm
	Const    *ConstTerm
	Rolls    []int
	Kept     []int
	Dropped  []int
	Subtotal int
}

// Result is the outcome of rolling an entire Expression once.
type Result struct {
	Terms []TermResult
	Total int
}

// Roll evaluates expr once using rng, rolling every dice term and summing
// every term's signed contribution.
func Roll(expr *Expression, rng *rand.Rand) Result {
	var result Result
	for _, term := range expr.Terms {
		tr := rollTerm(term, rng)
		result.Terms = append(result.Terms, tr)
		result.Total += tr.Subtotal
	}
	return result
}

func rollTerm(term Term, rng *rand.Rand) TermResult {
	if term.Const != nil {
		return TermResult{
			Const:    term.Const,
			Subtotal: term.Const.Sign * term.Const.Value,
		}
	}

	dt := term.Dice
	rolls := make([]int, dt.Count)
	for i := range rolls {
		rolls[i] = rng.Intn(dt.Sides) + 1
	}

	kept, dropped := applyModifier(rolls, dt.Mod, dt.ModCount)
	sum := 0
	for _, v := range kept {
		sum += v
	}
	return TermResult{
		Dice:     dt,
		Rolls:    rolls,
		Kept:     kept,
		Dropped:  dropped,
		Subtotal: dt.Sign * sum,
	}
}

// applyModifier splits rolls into kept and dropped according to mod. For
// ties it keeps/drops by sorted position rather than by original index,
// since dice notation doesn't distinguish "which" die among equal values.
func applyModifier(rolls []int, mod ModKind, n int) (kept, dropped []int) {
	if mod == ModNone {
		return append([]int(nil), rolls...), nil
	}

	sorted := append([]int(nil), rolls...)
	sort.Ints(sorted)
	total := len(sorted)

	switch mod {
	case ModKeepHigh:
		return sorted[total-n:], sorted[:total-n]
	case ModKeepLow:
		return sorted[:n], sorted[n:]
	case ModDropHigh:
		return sorted[:total-n], sorted[total-n:]
	case ModDropLow:
		return sorted[n:], sorted[:n]
	}
	return sorted, nil
}
