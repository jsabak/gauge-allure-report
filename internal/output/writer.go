// SPDX-License-Identifier: Apache-2.0

// Package output safely persists native Allure result artifacts.
package output

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/allure-framework/allure-go/commons/model"
)

const ownershipMarker = ".gauge-allure-report-owned"

// Writer is the narrow boundary consumed by mapping and metadata packages.
type Writer interface {
	WriteResult(context.Context, model.TestResult) error
	WriteContainer(context.Context, model.TestResultContainer) error
	WriteAttachment(context.Context, string, io.Reader, int64) error
	WriteEnvironment(context.Context, map[string]string) error
	WriteExecutor(context.Context, model.Executor) error
	WriteCategories(context.Context, []model.Category) error
}

// SafeWriter writes only basename artifacts using temp-file replacement.
type SafeWriter struct {
	dir      string
	maxBytes int64
}

// New returns an output writer for a prepared directory.
func New(dir string, maxBytes int64) *SafeWriter { return &SafeWriter{dir: dir, maxBytes: maxBytes} }

// Dir returns the absolute output location.
func (w *SafeWriter) Dir() string { return w.dir }

// Prepare validates and optionally cleans a results directory. Cleaning never
// recurses and only removes artifacts owned by this plugin.
func Prepare(projectRoot, dir string, clean, allowNonOwned bool) error {
	rootAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return fmt.Errorf("resolve project root: %w", err)
	}
	dirAbs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve results directory: %w", err)
	}
	if err := refuseDangerous(rootAbs, dirAbs); err != nil {
		return err
	}
	if info, err := os.Lstat(dirAbs); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("results path must be a real directory: %s", dirAbs)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect results directory: %w", err)
	}
	if err := os.MkdirAll(dirAbs, 0o755); err != nil {
		return fmt.Errorf("create results directory: %w", err)
	}
	marker := filepath.Join(dirAbs, ownershipMarker)
	if clean {
		owned := false
		if data, err := os.ReadFile(marker); err == nil && strings.HasPrefix(string(data), "gauge-allure-report\n") {
			owned = true
		}
		entries, err := os.ReadDir(dirAbs)
		if err != nil {
			return fmt.Errorf("read results directory: %w", err)
		}
		if !owned && !allowNonOwned && hasMaterialEntries(entries) {
			return fmt.Errorf("refusing to clean non-owned non-empty directory %s; set allow_clean_non_owned explicitly", dirAbs)
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !knownArtifact(entry.Name()) {
				continue
			}
			if err := os.Remove(filepath.Join(dirAbs, entry.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove old artifact %s: %w", entry.Name(), err)
			}
		}
	}
	if err := os.WriteFile(marker, []byte("gauge-allure-report\nformat=1\n"), 0o644); err != nil {
		return fmt.Errorf("write ownership marker: %w", err)
	}
	return nil
}

func refuseDangerous(projectRoot, target string) error {
	volume := filepath.VolumeName(target)
	filesystemRoot := volume + string(filepath.Separator)
	home, _ := os.UserHomeDir()
	dangerous := []string{filesystemRoot, projectRoot, filepath.Join(projectRoot, ".git")}
	if home != "" {
		dangerous = append(dangerous, home)
	}
	cleanTarget := filepath.Clean(target)
	for _, path := range dangerous {
		if path != "" && strings.EqualFold(cleanTarget, filepath.Clean(path)) {
			return fmt.Errorf("refusing dangerous results directory: %s", target)
		}
	}
	if len(filepath.Clean(target)) <= len(filesystemRoot)+2 {
		return fmt.Errorf("refusing suspiciously short results directory: %s", target)
	}
	return nil
}

func hasMaterialEntries(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if entry.Name() != ownershipMarker {
			return true
		}
	}
	return false
}

func knownArtifact(name string) bool {
	return name == ownershipMarker || name == "environment.properties" || name == "executor.json" ||
		name == "categories.json" || strings.HasSuffix(name, "-result.json") ||
		strings.HasSuffix(name, "-container.json") || strings.Contains(name, "-attachment.") ||
		(strings.HasPrefix(name, ".") && strings.Contains(name, ".tmp-"))
}

func (w *SafeWriter) WriteResult(ctx context.Context, value model.TestResult) error {
	if value.UUID == "" {
		return errors.New("write result: UUID is required")
	}
	return w.writeJSON(ctx, value.UUID+"-result.json", value)
}

func (w *SafeWriter) WriteContainer(ctx context.Context, value model.TestResultContainer) error {
	if value.UUID == "" {
		return errors.New("write container: UUID is required")
	}
	return w.writeJSON(ctx, value.UUID+"-container.json", value)
}

func (w *SafeWriter) WriteAttachment(ctx context.Context, name string, reader io.Reader, size int64) error {
	if size < 0 || size > w.maxBytes {
		return fmt.Errorf("attachment size %d exceeds configured limit %d", size, w.maxBytes)
	}
	return w.write(ctx, name, io.LimitReader(reader, w.maxBytes+1), size)
}

func (w *SafeWriter) WriteEnvironment(ctx context.Context, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(escapeProperty(key))
		builder.WriteByte('=')
		builder.WriteString(escapeProperty(values[key]))
		builder.WriteByte('\n')
	}
	data := []byte(builder.String())
	return w.write(ctx, "environment.properties", bytes.NewReader(data), int64(len(data)))
}

func (w *SafeWriter) WriteExecutor(ctx context.Context, value model.Executor) error {
	return w.writeJSON(ctx, "executor.json", value)
}

func (w *SafeWriter) WriteCategories(ctx context.Context, values []model.Category) error {
	return w.writeJSON(ctx, "categories.json", values)
}

func (w *SafeWriter) writeJSON(ctx context.Context, name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}
	data = append(data, '\n')
	return w.write(ctx, name, bytes.NewReader(data), int64(len(data)))
}

func (w *SafeWriter) write(ctx context.Context, name string, reader io.Reader, expected int64) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if filepath.Base(name) != name || name == "." || name == "" {
		return fmt.Errorf("unsafe artifact name %q", name)
	}
	if err := os.MkdirAll(w.dir, 0o755); err != nil {
		return fmt.Errorf("create results directory: %w", err)
	}
	temp, err := os.CreateTemp(w.dir, "."+name+".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary artifact: %w", err)
	}
	tempName := temp.Name()
	keep := false
	defer func() {
		_ = temp.Close()
		if !keep {
			_ = os.Remove(tempName)
		}
	}()
	written, err := io.Copy(temp, reader)
	if err != nil {
		return fmt.Errorf("write temporary artifact: %w", err)
	}
	if written > w.maxBytes && strings.Contains(name, "-attachment.") {
		return fmt.Errorf("attachment exceeded configured limit %d", w.maxBytes)
	}
	if expected >= 0 && written != expected {
		return fmt.Errorf("artifact size changed while copying: expected %d, wrote %d", expected, written)
	}
	if err := temp.Chmod(0o644); err != nil {
		return fmt.Errorf("set artifact permissions: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync artifact: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close artifact: %w", err)
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := atomicReplace(tempName, filepath.Join(w.dir, name)); err != nil {
		return fmt.Errorf("publish artifact %s: %w", name, err)
	}
	keep = true
	return nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func escapeProperty(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r", "\t", "\\t", "=", "\\=", ":", "\\:")
	return replacer.Replace(value)
}
