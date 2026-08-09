// SPDX-License-Identifier: Apache-2.0

// Package logging provides a small stderr-only structured logger.
package logging

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type Level int

const (
	levelDebug Level = iota
	levelInfo
	levelWarn
	levelError
)

// Logger serializes log lines emitted by concurrent Gauge streams.
type Logger struct {
	mu      sync.Mutex
	out     io.Writer
	minimum Level
}

// New returns a logger whose output can never corrupt Gauge's stdout handshake.
func New(out io.Writer, level string) *Logger {
	minimum := levelInfo
	switch level {
	case "debug":
		minimum = levelDebug
	case "warn":
		minimum = levelWarn
	case "error":
		minimum = levelError
	}
	return &Logger{out: out, minimum: minimum}
}
func (l *Logger) Debug(format string, values ...any) { l.log(levelDebug, "DEBUG", format, values...) }
func (l *Logger) Info(format string, values ...any)  { l.log(levelInfo, "INFO", format, values...) }
func (l *Logger) Warn(format string, values ...any)  { l.log(levelWarn, "WARN", format, values...) }
func (l *Logger) Error(format string, values ...any) { l.log(levelError, "ERROR", format, values...) }
func (l *Logger) log(level Level, name, format string, values ...any) {
	if l == nil || level < l.minimum {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.out, "%s [%s] %s\n", time.Now().UTC().Format(time.RFC3339Nano), name, fmt.Sprintf(format, values...))
}
