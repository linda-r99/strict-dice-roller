package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"strict-dice-roller/dice"
)

func main() {
	lenient := flag.Bool("lenient", false, "allow relaxed notation: whitespace, implicit counts (d6 = 1d6), uppercase D, leading zeros, mixed-case modifiers")
	seed := flag.Int64("seed", 0, "seed the random number generator for reproducible rolls (0 derives a seed from the current time)")
	count := flag.Int("count", 1, "number of times to roll the expression")
	quiet := flag.Bool("quiet", false, "print only the total for each roll")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <notation>\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "examples:")
		fmt.Fprintf(os.Stderr, "  %s 3d6\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 1d20+5\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 4d6kh3\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --lenient '2d6 + 1d4'\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "\nflags:")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
		os.Exit(2)
	}
	notation := args[0]

	expr, err := dice.Parse(notation, *lenient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if *count < 1 {
		fmt.Fprintln(os.Stderr, "error: --count must be at least 1")
		os.Exit(2)
	}

	s := *seed
	if s == 0 {
		s = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(s))

	for i := 0; i < *count; i++ {
		printResult(dice.Roll(expr, rng), *quiet)
	}
}

func printResult(result dice.Result, quiet bool) {
	if quiet {
		fmt.Println(result.Total)
		return
	}

	var b strings.Builder
	for i, tr := range result.Terms {
		sign, body := describeTerm(tr)
		if i == 0 {
			if sign == "-" {
				b.WriteString("-")
			}
			b.WriteString(body)
			continue
		}
		b.WriteString(" ")
		b.WriteString(sign)
		b.WriteString(" ")
		b.WriteString(body)
	}
	fmt.Printf("%s = %d\n", b.String(), result.Total)
}

// describeTerm renders one term's contribution, e.g. ("+", "[4 2 6]") or
// ("+", "[5 3 6] (dropped [1])"). The sign is returned separately so the
// caller can omit it for the leading term.
func describeTerm(tr dice.TermResult) (sign, body string) {
	if tr.Const != nil {
		sign = "+"
		if tr.Const.Sign < 0 {
			sign = "-"
		}
		return sign, fmt.Sprintf("%d", tr.Const.Value)
	}

	sign = "+"
	if tr.Dice.Sign < 0 {
		sign = "-"
	}
	if len(tr.Dropped) == 0 {
		return sign, fmt.Sprintf("%v", tr.Rolls)
	}
	return sign, fmt.Sprintf("%v (dropped %v)", tr.Kept, tr.Dropped)
}
