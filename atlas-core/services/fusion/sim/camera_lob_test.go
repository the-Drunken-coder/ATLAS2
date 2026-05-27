package sim

import "testing"

func TestNormalizeAzimuthDegLargeNoise(t *testing.T) {
	tests := []struct {
		in, want float64
	}{
		{450, 90},
		{-90, 270},
		{720, 0},
		{359.9, 359.9},
	}
	for _, tc := range tests {
		got := normalizeAzimuthDeg(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeAzimuthDeg(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
