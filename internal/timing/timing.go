// SPDX-License-Identifier: Apache-2.0

// Package timing implements deterministic Gauge timestamp fallbacks.
package timing

import "time"

// Range contains Allure epoch-millisecond timestamps.
type Range struct {
	Start int64
	Stop  int64
}

// Resolve parses an RFC-3339 timestamp and applies Gauge's millisecond duration.
// fallback is used when the timestamp is absent or invalid.
func Resolve(timestamp string, durationMS int64, fallback time.Time) Range {
	start := fallback
	if parsed, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
		start = parsed
	}
	if start.IsZero() {
		start = time.Unix(0, 0).UTC()
	}
	if durationMS < 0 {
		durationMS = 0
	}
	startMS := start.UnixMilli()
	stopMS := startMS + durationMS
	if stopMS < startMS {
		stopMS = startMS
	}
	return Range{Start: startMS, Stop: stopMS}
}

// Clamp keeps a nested item inside its parent when upstream timing is incomplete.
func Clamp(child, parent Range) Range {
	if parent.Start != 0 && child.Start < parent.Start {
		child.Start = parent.Start
	}
	if parent.Stop != 0 && child.Stop > parent.Stop {
		child.Stop = parent.Stop
	}
	if child.Stop < child.Start {
		child.Stop = child.Start
	}
	return child
}
