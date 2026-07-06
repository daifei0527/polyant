package safeconv

import (
	"math"
	"testing"
)

func TestIntFromUint64(t *testing.T) {
	if got := IntFromUint64(0); got != 0 {
		t.Errorf("0 -> %d", got)
	}
	if got := IntFromUint64(42); got != 42 {
		t.Errorf("42 -> %d", got)
	}
	if got := IntFromUint64(1 << 40); got != math.MaxInt32 {
		t.Errorf("overflow not clamped: %d", got)
	}
}

func TestInt32FromInt(t *testing.T) {
	if got := Int32FromInt(0); got != 0 {
		t.Errorf("0 -> %d", got)
	}
	if got := Int32FromInt(100); got != 100 {
		t.Errorf("100 -> %d", got)
	}
	if got := Int32FromInt(math.MaxInt); got != math.MaxInt32 {
		t.Errorf("overflow not clamped: %d", got)
	}
	if got := Int32FromInt(-math.MaxInt); got != math.MinInt32 {
		t.Errorf("underflow not clamped: %d", got)
	}
}
