// Package hll implements the HyperLogLog cardinality estimator
// (Flajolet et al., 2007): it estimates how many distinct values have
// been seen, using a fixed amount of memory regardless of the input size.
//
// A sketch with precision p keeps m = 2^p one-byte registers. Each value
// is hashed to 64 bits; the top p bits pick a register and the position
// of the first 1-bit in the remaining bits updates that register with a
// running maximum. The estimate is the harmonic mean of 2^-register over
// all registers, corrected for small ranges (linear counting) and large
// ranges (the 2^32 correction).
//
// Sketches are mergeable: the union of two sketches is the element-wise
// maximum of their registers, so counts from separate machines, days, or
// shards can be combined without ever exchanging the raw values.
package hll

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/bits"
)

const (
	// MinPrecision keeps 16 registers; below this the estimator is too noisy.
	MinPrecision = 4
	// MaxPrecision keeps 262144 registers (256 KiB); above this a register
	// no longer fits the rho value of a 64-bit hash.
	MaxPrecision = 18
	// DefaultPrecision is 14: 16384 registers (16 KiB) with a standard
	// error of about 0.81%, the usual production default.
	DefaultPrecision = 14
)

// Sketch is a HyperLogLog sketch of fixed precision.
type Sketch struct {
	p         uint8
	registers []uint8
}

// New returns an empty sketch with the given precision.
// Precision must be between MinPrecision and MaxPrecision.
func New(precision int) (*Sketch, error) {
	if precision < MinPrecision || precision > MaxPrecision {
		return nil, fmt.Errorf("hll: precision %d out of range [%d, %d]",
			precision, MinPrecision, MaxPrecision)
	}
	return &Sketch{p: uint8(precision), registers: make([]uint8, 1<<uint(precision))}, nil
}

// MustNew is New but panics on an invalid precision. It is convenient for
// package-level sketches and tests with a compile-time-known precision.
func MustNew(precision int) *Sketch {
	s, err := New(precision)
	if err != nil {
		panic(err)
	}
	return s
}

// Precision returns the sketch's precision p (register count is 2^p).
func (s *Sketch) Precision() int { return int(s.p) }

// Registers returns the number of registers in the sketch.
func (s *Sketch) Registers() int { return len(s.registers) }

// StdError returns the theoretical relative standard error, 1.04/sqrt(m).
func (s *Sketch) StdError() float64 {
	return 1.04 / math.Sqrt(float64(len(s.registers)))
}

// hash returns a deterministic 64-bit hash of v. SHA-256 truncated to 8
// bytes is used instead of a faster non-cryptographic hash on purpose:
// the hash must be identical across processes and machines, or sketches
// built in different places cannot be merged. For a cardinality sketch
// the throughput cost is irrelevant next to the correctness guarantee.
func hash(v []byte) uint64 {
	sum := sha256.Sum256(v)
	return binary.BigEndian.Uint64(sum[:8])
}

// Add records one value. Adding the same value any number of times
// changes nothing after the first add.
func (s *Sketch) Add(v []byte) {
	h := hash(v)
	// Top p bits: register index.
	idx := h >> (64 - s.p)
	// Remaining bits: rho is 1 + the number of leading zeroes.
	rest := h << s.p
	rho := uint8(bits.LeadingZeros64(rest)) + 1
	// rho can reach 64-p+1 at most, which always fits in a byte.
	if rho > s.registers[idx] {
		s.registers[idx] = rho
	}
}

// AddString is Add for string values.
func (s *Sketch) AddString(v string) { s.Add([]byte(v)) }

// Count estimates the number of distinct values seen so far.
func (s *Sketch) Count() uint64 {
	m := float64(len(s.registers))
	// Harmonic mean of 2^-register.
	sum := 0.0
	zeros := 0
	for _, r := range s.registers {
		sum += 1.0 / float64(uint64(1)<<r)
		if r == 0 {
			zeros++
		}
	}
	estimate := alpha(len(s.registers)) * m * m / sum

	// Small-range correction: when the raw estimate is small, linear
	// counting on the empty registers is far more accurate.
	if estimate <= 2.5*m {
		if zeros > 0 {
			estimate = m * math.Log(m/float64(zeros))
		}
	} else if estimate > (1.0/30.0)*math.Pow(2, 32) {
		// Large-range correction for the 32-bit hash space.
		estimate = -math.Pow(2, 32) * math.Log(1-estimate/math.Pow(2, 32))
	}
	return uint64(math.Round(estimate))
}

// alpha returns the bias-correction constant for m registers.
func alpha(m int) float64 {
	switch m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	default:
		return 0.7213 / (1 + 1.079/float64(m))
	}
}

// Merge folds other into s, so s afterwards estimates the cardinality of
// the union of both sketches. The sketches must share a precision;
// merging sketches of different precisions is an error, not a guess.
func (s *Sketch) Merge(other *Sketch) error {
	if other == nil {
		return errors.New("hll: cannot merge a nil sketch")
	}
	if s.p != other.p {
		return fmt.Errorf("hll: cannot merge precision %d with precision %d", s.p, other.p)
	}
	for i, r := range other.registers {
		if r > s.registers[i] {
			s.registers[i] = r
		}
	}
	return nil
}

// Clone returns an independent copy of the sketch.
func (s *Sketch) Clone() *Sketch {
	regs := make([]uint8, len(s.registers))
	copy(regs, s.registers)
	return &Sketch{p: s.p, registers: regs}
}

// IsEmpty reports whether no value has been added yet.
func (s *Sketch) IsEmpty() bool {
	for _, r := range s.registers {
		if r != 0 {
			return false
		}
	}
	return true
}
