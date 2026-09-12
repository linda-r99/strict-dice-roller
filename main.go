package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"strict-dice-roller/dice"
)

func main() {
	lenient := flag.Bool("lenient", false, "allow relaxed notation: whitespace, implicit counts (d6 = 1d6), uppercase D, leading zeros, mixed-case modifiers")
	seed := flag.Int64("seed", 0, "seed the random number generator for reproducible rolls (0 derives a seed from the current time)")
	count := flag.Int("count", 1, "number of times to roll the expression")
	quiet := flag.Bool("quiet", false, "print only the total for each roll")
	format := flag.String("format", "text", "output format: text or json")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] <notation>\n\n", os.Args[0])
		fmt.Fprintln(os.Stderr, "examples:")
		fmt.Fprintf(os.Stderr, "  %s 3d6\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 1d20+5\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 4d6kh3\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 6d6!\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s 4dF\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --lenient '2d6 + 1d4'\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --format=json 4d6kh3\n", os.Args[0])
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

	if *format != "text" && *format != "json" {
		fmt.Fprintf(os.Stderr, "error: --format must be \"text\" or \"json\", got %q\n", *format)
		os.Exit(2)
	}
	if *format == "json" && *quiet {
		fmt.Fprintln(os.Stderr, "error: --quiet has no effect with --format=json")
		os.Exit(2)
	}

	s := *seed
	if s == 0 {
		s = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(s))

	for i := 0; i < *count; i++ {
		result := dice.Roll(expr, rng)
		if *format == "json" {
			printResultJSON(result)
		} else {
			printResult(result, *quiet)
		}
	}
}

// jsonResult is the --format=json rendering of a single roll.
type jsonResult struct {
	Total int        `json:"total"`
	Terms []jsonTerm `json:"terms"`
}

// jsonTerm renders one term of a roll. Fields that don't apply to the
// term's type (const vs. dice, exploded vs. not, modified vs. not) are
// omitted rather than sent as zero values.
type jsonTerm struct {
	Type     string  `json:"type"`
	Sign     int     `json:"sign"`
	Subtotal int     `json:"subtotal"`
	Value    int     `json:"value,omitempty"`
	Count    int     `json:"count,omitempty"`
	Sides    int     `json:"sides,omitempty"`
	Fudge    bool    `json:"fudge,omitempty"`
	Explode  bool    `json:"explode,omitempty"`
	Mod      string  `json:"mod,omitempty"`
	ModCount int     `json:"mod_count,omitempty"`
	Chains   [][]int `json:"chains,omitempty"`
	Kept     [][]int `json:"kept,omitempty"`
	Dropped  [][]int `json:"dropped,omitempty"`
}

func printResultJSON(result dice.Result) {
	jr := jsonResult{Total: result.Total, Terms: make([]jsonTerm, len(result.Terms))}
	for i, tr := range result.Terms {
		jr.Terms[i] = toJSONTerm(tr)
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(jr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func toJSONTerm(tr dice.TermResult) jsonTerm {
	if tr.Const != nil {
		return jsonTerm{Type: "const", Sign: tr.Const.Sign, Value: tr.Const.Value, Subtotal: tr.Const.Sign * tr.Const.Value}
	}

	jt := jsonTerm{
		Type:     "dice",
		Sign:     tr.Dice.Sign,
		Subtotal: tr.Subtotal,
		Count:    tr.Dice.Count,
		Sides:    tr.Dice.Sides,
		Fudge:    tr.Dice.Fudge,
		Explode:  tr.Dice.Explode,
		Mod:      tr.Dice.Mod.String(),
		ModCount: tr.Dice.ModCount,
	}
	if len(tr.Dropped) == 0 {
		jt.Chains = tr.Chains
	} else {
		jt.Kept = tr.Kept
		jt.Dropped = tr.Dropped
	}
	return jt
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
// ("+", "[5 3 6] (dropped [1])"). An exploded die's chain is joined with
// "+", e.g. "[6+6+2 4 3]". The sign is returned separately so the caller
// can omit it for the leading term.
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
		return sign, formatChains(tr.Chains)
	}
	return sign, fmt.Sprintf("%s (dropped %s)", formatChains(tr.Kept), formatChains(tr.Dropped))
}

func formatChains(chains [][]int) string {
	parts := make([]string, len(chains))
	for i, chain := range chains {
		parts[i] = formatChain(chain)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func formatChain(chain []int) string {
	if len(chain) == 1 {
		return strconv.Itoa(chain[0])
	}
	parts := make([]string, len(chain))
	for i, v := range chain {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, "+")
}
