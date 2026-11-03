package hll

import (
	"bytes"
	"errors"
	"fmt"
)

// Binary format, versioned so future formats stay readable:
//   magic (4 bytes)  "HLL1"
//   precision (1 byte)
//   registers (2^p bytes, raw)
// No checksums are added: the file is a compact sketch, and a corrupt
// register can only shift the estimate slightly, never crash a reader.
// A wrong magic or a truncated body is rejected outright.

var magic = [4]byte{'H', 'L', 'L', '1'}

// MarshalBinary serializes the sketch to its compact binary form.
func (s *Sketch) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 0, 5+len(s.registers))
	buf = append(buf, magic[:]...)
	buf = append(buf, s.p)
	return append(buf, s.registers...), nil
}

// UnmarshalBinary restores a sketch serialized by MarshalBinary.
// The returned sketch is independent of the input slice.
func UnmarshalBinary(data []byte) (*Sketch, error) {
	if len(data) < 5 || !bytes.Equal(data[:4], magic[:]) {
		return nil, errors.New("hll: not a HyperLogLog sketch (bad magic)")
	}
	p := int(data[4])
	if p < MinPrecision || p > MaxPrecision {
		return nil, fmt.Errorf("hll: invalid precision %d in sketch data", p)
	}
	want := 5 + (1 << uint(p))
	if len(data) != want {
		return nil, fmt.Errorf("hll: truncated sketch: got %d bytes, want %d", len(data), want)
	}
	s, err := New(p)
	if err != nil {
		return nil, err
	}
	copy(s.registers, data[5:])
	return s, nil
}
