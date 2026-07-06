// Package safeconv provides overflow-safe integer conversions for values that
// are provably small in practice (counts, limits) but trigger gosec G115.
package safeconv

import "math"

// IntFromUint64 converts a uint64 to int, clamping at math.MaxInt32 for safety
// on all platforms. Counts/limits in this codebase never approach MaxInt32.
func IntFromUint64(v uint64) int {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(v)
}

// Int32FromInt converts an int to int32, clamping at the int32 range.
func Int32FromInt(v int) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}
