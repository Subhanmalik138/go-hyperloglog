package hll

import (
	"fmt"
	"math"
	"testing"
)

// within reports whether got is within pct percent of want.
func within(got uint64, want int, pct float64) bool {
	diff := math.Abs(float64(got) - float64(want))
	return diff <= float64(want)*pct/100
}

func TestEmptySketchCountsZero(t *testing.T) {
	s := MustNew(14)
	if got := s.Count(); got != 0 {
		t.Fatalf("empty sketch counted %d, want 0", got)
	}
	if !s.IsEmpty() {
		t.Fatal("empty sketch reports non-empty")
	}
}

func TestSingleValueCountsOne(t *testing.T) {
	s := MustNew(14)
	s.AddString("hello")
	if got := s.Count(); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
	if s.IsEmpty() {
		t.Fatal("non-empty sketch reports empty")
	}
}

func TestDuplicatesDoNotInflate(t *testing.T) {
	s := MustNew(14)
	for i := 0; i < 10000; i++ {
		s.AddString("same-value")
	}
	if got := s.Count(); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
}

func TestSmallCardinalityIsAccurate(t *testing.T) {
	s := MustNew(14)
	for i := 0; i < 100; i++ {
		s.AddString(fmt.Sprintf("user-%d", i))
	}
	if got := s.Count(); !within(got, 100, 5) {
		t.Fatalf("count = %d, want ~100", got)
	}
}

func TestLargeCardinalityWithinError(t *testing.T) {
	s := MustNew(14)
	const n = 100000
	for i := 0; i < n; i++ {
		s.AddString(fmt.Sprintf("event-%d", i))
	}
	// p=14 gives ~0.81% standard error; 3% is a generous bound that
	// should essentially never fail for a correct implementation.
	if got := s.Count(); !within(got, n, 3) {
		t.Fatalf("count = %d, want ~%d", got, n)
	}
}

func TestMergeEqualsUnion(t *testing.T) {
	a := MustNew(14)
	b := MustNew(14)
	for i := 0; i < 5000; i++ {
		a.AddString(fmt.Sprintf("a-%d", i))
	}
	for i := 2500; i < 7500; i++ {
		b.AddString(fmt.Sprintf("a-%d", i)) // 2500 overlap, 2500 new
	}
	if err := a.Merge(b); err != nil {
		t.Fatal(err)
	}
	if got := a.Count(); !within(got, 7500, 3) {
		t.Fatalf("merged count = %d, want ~7500", got)
	}
}

func TestMergePrecisionMismatchErrors(t *testing.T) {
	if err := MustNew(14).Merge(MustNew(12)); err == nil {
		t.Fatal("merging different precisions should error")
	}
	if err := MustNew(14).Merge(nil); err == nil {
		t.Fatal("merging nil should error")
	}
}

func TestInvalidPrecisionErrors(t *testing.T) {
	for _, p := range []int{0, 3, 19, 100} {
		if _, err := New(p); err == nil {
			t.Fatalf("precision %d should error", p)
		}
	}
}

func TestSerializationRoundTrip(t *testing.T) {
	s := MustNew(14)
	for i := 0; i < 1000; i++ {
		s.AddString(fmt.Sprintf("item-%d", i))
	}
	data, err := s.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := UnmarshalBinary(data)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Precision() != s.Precision() || restored.Count() != s.Count() {
		t.Fatalf("round trip changed sketch: %d -> %d", s.Count(), restored.Count())
	}
}

func TestSerializationRejectsBadData(t *testing.T) {
	for _, bad := range [][]byte{
		nil,
		[]byte("not a sketch"),
		[]byte{'H', 'L', 'L', '1', 14, 1, 2, 3}, // truncated registers
	} {
		if _, err := UnmarshalBinary(bad); err == nil {
			t.Fatalf("bad data %q should error", bad)
		}
	}
}

func TestCloneIsIndependent(t *testing.T) {
	s := MustNew(14)
	s.AddString("x")
	c := s.Clone()
	c.AddString("y")
	if s.Count() == c.Count() {
		t.Fatal("mutating clone changed the original")
	}
}

func TestStdErrorDecreasesWithPrecision(t *testing.T) {
	if MustNew(10).StdError() <= MustNew(14).StdError() {
		t.Fatal("higher precision should give lower standard error")
	}
}
