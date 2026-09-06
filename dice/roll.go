package dice

import (
	"math/rand"
	"sort"
)

// TermResult holds one term's raw rolls (if any) and its signed
// contribution to the total, for building a readable breakdown.
//
// Chains holds one entry per die rolled. A die that didn't explode has a
// chain of length 1; an exploded die's chain is every roll in the sequence,
// in order, whose sum is that die's effective value. Kept and Dropped
// partition Chains according to the term's keep/drop modifier, comparing
// dice by their chain sums.
type TermResult struct {
	Dice     *DiceTerm
	Const    *ConstTerm
	Chains   [][]int
	Kept     [][]int
	Dropped  [][]int
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
	chains := make([][]int, dt.Count)
	values := make([]int, dt.Count)
	for i := range chains {
		chain := rollChain(dt.Sides, dt.Explode, rng)
		chains[i] = chain
		sum := 0
		for _, v := range chain {
			sum += v
		}
		values[i] = sum
	}

	keptIdx, droppedIdx := selectByModifier(values, dt.Mod, dt.ModCount)
	kept := chainsAt(chains, keptIdx)
	dropped := chainsAt(chains, droppedIdx)
	sum := 0
	for _, i := range keptIdx {
		sum += values[i]
	}
	return TermResult{
		Dice:     dt,
		Chains:   chains,
		Kept:     kept,
		Dropped:  dropped,
		Subtotal: dt.Sign * sum,
	}
}

// intSource is the part of *rand.Rand that die rolling needs, factored out
// so tests can drive rollChain with a fixed sequence.
type intSource interface {
	Intn(n int) int
}

// rollChain rolls one die, and if explode is set, keeps rolling and
// appending while the most recent roll came up the maximum face. The chain
// is capped at MaxExplosionChain rolls so a pathological seed can't spin
// forever.
func rollChain(sides int, explode bool, rng intSource) []int {
	chain := []int{rng.Intn(sides) + 1}
	for explode && chain[len(chain)-1] == sides && len(chain) < MaxExplosionChain {
		chain = append(chain, rng.Intn(sides)+1)
	}
	return chain
}

// selectByModifier splits die indices into kept and dropped according to
// mod, comparing dice by value. For ties it keeps/drops by sorted position
// rather than by original index, since dice notation doesn't distinguish
// "which" die among equal values.
func selectByModifier(values []int, mod ModKind, n int) (kept, dropped []int) {
	idx := make([]int, len(values))
	for i := range idx {
		idx[i] = i
	}
	if mod == ModNone {
		return idx, nil
	}

	sort.Slice(idx, func(a, b int) bool { return values[idx[a]] < values[idx[b]] })
	total := len(idx)

	switch mod {
	case ModKeepHigh:
		return idx[total-n:], idx[:total-n]
	case ModKeepLow:
		return idx[:n], idx[n:]
	case ModDropHigh:
		return idx[:total-n], idx[total-n:]
	case ModDropLow:
		return idx[n:], idx[:n]
	}
	return idx, nil
}

func chainsAt(chains [][]int, idx []int) [][]int {
	out := make([][]int, len(idx))
	for i, j := range idx {
		out[i] = chains[j]
	}
	return out
}
