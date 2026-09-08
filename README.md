# strict-dice-roller

A command line tool that parses dice notation (`3d6+2`, `4d6kh3`, `1d20-1`)
and rolls it.

Every tabletop tool has its own idea of what counts as valid dice notation:
some accept `d6` as shorthand for `1d6`, some are case-insensitive about the
`d`, some tolerate stray whitespace. That's fine for a human typing into a
chat window, but it's a problem if you're piping notation in from a script,
a bot command, or a save file and you want bad input to fail loudly instead
of being silently reinterpreted. `diceroll` parses a strict grammar by
default and only relaxes it when you explicitly ask for that with
`--lenient`.

## Usage

```
$ diceroll 3d6
[4 2 6] = 12

$ diceroll 1d20+5
[14] + 5 = 19

$ diceroll 4d6kh3
[5 3 6] (dropped [1]) = 14

$ diceroll -1d4+2d6
-[3] + [4 2] = 3

$ diceroll --count 3 --quiet 2d6
7
9
4

$ diceroll 6d6!
[3 6+4 2 6+6+1 5 1] = 34

$ diceroll 4dF
[1 -1 0 1] = 1
```

A `!` after the side count makes the dice explode: any die that rolls its
maximum face rolls again, and the extra roll adds to that die's total
instead of ending it there. A chain can explode more than once, shown above
as `6+6+1`. Explosion is capped at `dice.MaxExplosionChain` rolls per die so
a run of maximum rolls can't loop forever.

`F` in place of a side count rolls fudge/fate dice: each die comes up -1, 0,
or 1 instead of a face number, and the total is the sum. `4dF` is the usual
notation for a fate roll.

```
$ diceroll 4dF+2
[1 -1 0 1] + 2 = 3
```

Strict mode rejects notation that a human might type casually but that a
parser shouldn't have to guess about:

```
$ diceroll 'd6'
error: strict mode: dice count must be explicit, write "1d6" not "d6" (try --lenient)

$ diceroll '2d6 + 3'
error: strict mode: notation may not contain whitespace (got "2d6 + 3", try --lenient)

$ diceroll --lenient '2d6 + 3'
[5 1] + 3 = 9
```

## Grammar

```
expression := term (('+' | '-') term)*
term       := dice | integer
dice       := count 'd' (sides | 'F') ['!'] [modifier]
modifier   := ('kh' | 'kl' | 'dh' | 'dl') count
```

- `count` is the number of dice rolled (1-1000).
- `sides` is the number of faces per die (2-10000), or `F` for fudge dice.
- `!` makes the dice explode (see above). On fudge dice, a die explodes when
  it rolls 1, its highest face.
- `kh`/`kl` keep the highest/lowest N rolls and discard the rest; `dh`/`dl`
  drop the highest/lowest N and keep the rest. Keep/drop compares dice by
  their total after explosion, not by the value of their first roll.
- A bare integer (e.g. the `+3` in `1d20+3`) is a flat modifier.

In strict mode (the default):

- no whitespace anywhere in the notation
- `d` must be lowercase; `D` is rejected
- the fudge specifier must be uppercase (`4dF`); `4df` is rejected
- dice counts and modifier counts must be written explicitly (`1d6`, `kh1`,
  never `d6` or `kh`)
- no leading zeros on any number

`--lenient` relaxes every rule above. It does not change the resource limits
on dice count, sides, or constants - those apply either way.

## Flags

- `--lenient` - accept the relaxed grammar described above.
- `--seed N` - seed the random number generator for reproducible output
  (default: derived from the current time).
- `--count N` - roll the expression N times (default: 1).
- `--quiet` - print only the total for each roll, one per line.

## Build

```
go build -o diceroll .
```

Requires Go 1.22 or later. No third-party dependencies.
