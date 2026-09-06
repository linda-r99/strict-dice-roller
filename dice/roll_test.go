package dice

import (
	"math/rand"
	"reflect"
	"testing"
)

// fixedSeq feeds a scripted sequence of Intn results, one call at a time.
type fixedSeq struct {
	values []int
	i      int
}

func (f *fixedSeq) Intn(n int) int {
	v := f.values[f.i]
	f.i++
	return v
}

// alwaysMax always reports the highest possible roll, for exercising the
// explosion chain cap.
type alwaysMax struct{}

func (alwaysMax) Intn(n int) int { return n - 1 }

func TestRollChainNoExplode(t *testing.T) {
	rng := &fixedSeq{values: []int{2}}
	got := rollChain(6, false, rng)
	want := []int{3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rollChain(6, false, ...) = %v, want %v", got, want)
	}
}

func TestRollChainExplodes(t *testing.T) {
	// 0-indexed Intn results 5, 5, 1 => rolls 6, 6, 2 on a d6.
	rng := &fixedSeq{values: []int{5, 5, 1}}
	got := rollChain(6, true, rng)
	want := []int{6, 6, 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rollChain(6, true, ...) = %v, want %v", got, want)
	}
}

func TestRollChainDoesNotExplodeOnNonMax(t *testing.T) {
	rng := &fixedSeq{values: []int{3}}
	got := rollChain(6, true, rng)
	want := []int{4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rollChain(6, true, ...) = %v, want %v", got, want)
	}
}

func TestRollChainCapsAtMaxExplosionChain(t *testing.T) {
	got := rollChain(2, true, alwaysMax{})
	if len(got) != MaxExplosionChain {
		t.Fatalf("len(rollChain) = %d, want %d", len(got), MaxExplosionChain)
	}
	for _, v := range got {
		if v != 2 {
			t.Errorf("chain value = %d, want 2", v)
		}
	}
}

func TestSelectByModifierKeepHigh(t *testing.T) {
	values := []int{5, 1, 3, 3}
	kept, dropped := selectByModifier(values, ModKeepHigh, 2)
	if len(kept) != 2 || len(dropped) != 2 {
		t.Fatalf("kept=%v dropped=%v, want 2 and 2", kept, dropped)
	}
	keptSum, droppedSum := 0, 0
	for _, i := range kept {
		keptSum += values[i]
	}
	for _, i := range dropped {
		droppedSum += values[i]
	}
	if keptSum != 8 {
		t.Errorf("kept sum = %d, want 8 (5+3)", keptSum)
	}
	if droppedSum != 4 {
		t.Errorf("dropped sum = %d, want 4 (1+3)", droppedSum)
	}
}

func TestSelectByModifierNone(t *testing.T) {
	values := []int{5, 1, 3}
	kept, dropped := selectByModifier(values, ModNone, 0)
	if len(dropped) != 0 {
		t.Errorf("dropped = %v, want empty", dropped)
	}
	if !reflect.DeepEqual(kept, []int{0, 1, 2}) {
		t.Errorf("kept = %v, want [0 1 2]", kept)
	}
}

func TestRollTermExplodeInvariants(t *testing.T) {
	term := Term{Dice: &DiceTerm{Sign: 1, Count: 30, Sides: 3, Explode: true}}
	rng := rand.New(rand.NewSource(1))
	tr := rollTerm(term, rng)

	if len(tr.Chains) != 30 {
		t.Fatalf("len(Chains) = %d, want 30", len(tr.Chains))
	}
	total := 0
	for _, chain := range tr.Chains {
		if len(chain) > MaxExplosionChain {
			t.Errorf("chain length %d exceeds MaxExplosionChain", len(chain))
		}
		for i, v := range chain {
			if v < 1 || v > 3 {
				t.Errorf("roll %d out of range for d3", v)
			}
			if i < len(chain)-1 && v != 3 {
				t.Errorf("chain continued past a non-maximum roll: %v", chain)
			}
			total += v
		}
	}
	if total != tr.Subtotal {
		t.Errorf("Subtotal = %d, want sum of chains %d", tr.Subtotal, total)
	}
}

func TestRollTermNoExplodeSingleRollPerDie(t *testing.T) {
	term := Term{Dice: &DiceTerm{Sign: 1, Count: 10, Sides: 6, Explode: false}}
	rng := rand.New(rand.NewSource(1))
	tr := rollTerm(term, rng)

	for _, chain := range tr.Chains {
		if len(chain) != 1 {
			t.Errorf("chain = %v, want length 1 when Explode is false", chain)
		}
	}
}
