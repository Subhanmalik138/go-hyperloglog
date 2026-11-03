# HyperLogLog — Probabilistic Cardinality Estimation in Go

How many distinct users visited today? How many unique IPs hit an API
this month? Answering exactly means storing every value — or a hash of
every value — which grows without bound. HyperLogLog answers
*approximately*, with a fixed 16 KiB of memory and about 0.81% error,
no matter whether the input has a thousand or a hundred million
distinct values.

## How it works

Each value is hashed to 64 bits. The top `p` bits pick one of `m = 2^p`
registers; the position of the first 1-bit in the remaining bits updates
that register to a running maximum. Rare long runs of leading zeroes
only happen for a small fraction of the input, so the pattern of
register maxima encodes the cardinality: the estimate is the harmonic
mean of `2^-register` over all registers, with linear counting for
small ranges and the standard large-range correction.

Two sketches with the same precision merge by taking the element-wise
maximum of their registers — so daily sketches, per-shard sketches, or
sketches from different machines combine into the union's count without
ever exchanging raw data.

## Design decisions

1. **Deterministic hashing (SHA-256, truncated).** A sketch is only
   mergeable if every producer hashes identically. A fast hash with a
   per-process random seed would silently corrupt merges across
   processes; SHA-256 costs throughput this use case never needs.
2. **Registers as raw bytes, one per register.** The binary format is
   a 5-byte header (`HLL1` magic + precision) followed by the registers
   verbatim — a p=14 sketch serializes to exactly 16,389 bytes.
3. **Precision validated at construction and on merge.** Merging
   sketches of different precisions is an error, not a silent
   approximation, and invalid precisions are rejected up front.
4. **Standard library only.** No dependencies to audit or pin.

## Usage

```go
sketch, _ := hll.New(14) // 16384 registers, ~0.81% std error
sketch.AddString("user-42")
sketch.AddString("user-42") // duplicates change nothing
fmt.Println(sketch.Count()) // 1

other, _ := hll.New(14)
other.AddString("user-99")
sketch.Merge(other)
fmt.Println(sketch.Count()) // 2
```

## CLI

```sh
go build -o hll ./cmd/hll

# Estimate distinct lines from a file (or stdin)
hll add -in visitors.txt
# ... and save the sketch for later
hll add -in day1.txt -out day1.sketch
hll add -in day2.txt -out day2.sketch
# Union of two days, without the raw logs
hll merge -out week.sketch day1.sketch day2.sketch
hll count week.sketch
```

## Run tests

```sh
go test ./...
```

## Tech stack

Go (standard library only), `go test` for the test suite.
