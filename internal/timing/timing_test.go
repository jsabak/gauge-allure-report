// SPDX-License-Identifier: Apache-2.0

package timing

import (
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	fallback := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name, stamp         string
		duration            int64
		wantStart, wantStop int64
	}{
		{"rfc3339 nanos", "2025-02-03T04:05:06.123456Z", 7, 1738555506123, 1738555506130},
		{"invalid fallback", "not-a-time", 10, fallback.UnixMilli(), fallback.UnixMilli() + 10},
		{"negative duration", "", -1, fallback.UnixMilli(), fallback.UnixMilli()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Resolve(test.stamp, test.duration, fallback)
			if got.Start != test.wantStart || got.Stop != test.wantStop {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestClamp(t *testing.T) {
	got := Clamp(Range{Start: 5, Stop: 30}, Range{Start: 10, Stop: 20})
	if got != (Range{Start: 10, Stop: 20}) {
		t.Fatalf("got %+v", got)
	}
}
