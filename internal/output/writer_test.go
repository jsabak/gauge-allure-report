// SPDX-License-Identifier: Apache-2.0

package output

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allure-framework/allure-go/commons/model"
)

func TestPrepareOwnershipAndKnownArtifactCleaning(t *testing.T) {
	root := t.TempDir()
	results := filepath.Join(root, "reports", "allure-results")
	if err := Prepare(root, results, true, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(results, "old-result.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(results, "keep.txt"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(root, results, true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(results, "old-result.json")); !os.IsNotExist(err) {
		t.Fatalf("old result remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(results, "keep.txt")); err != nil {
		t.Fatalf("unknown file removed: %v", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestWriterWritesEveryAllureArtifact(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "results")
	writer := New(dir, 1024)
	if writer.Dir() != dir {
		t.Fatalf("Dir() = %q", writer.Dir())
	}
	ctx := context.Background()
	if err := writer.WriteContainer(ctx, model.TestResultContainer{UUID: "container", Name: "hooks"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteEnvironment(ctx, map[string]string{"z:key": "line\nvalue", "a=key": "tab\t\\"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteExecutor(ctx, model.Executor{Name: "CI", Type: "github"}); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteCategories(ctx, []model.Category{{Name: "Broken"}}); err != nil {
		t.Fatal(err)
	}
	environment, err := os.ReadFile(filepath.Join(dir, "environment.properties"))
	if err != nil {
		t.Fatal(err)
	}
	if string(environment) != "a\\=key=tab\\t\\\\\nz\\:key=line\\nvalue\n" {
		t.Fatalf("unexpected escaped/sorted environment: %q", environment)
	}
	for _, name := range []string{"container-container.json", "executor.json", "categories.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestWriterRejectsInvalidInputsAndCancellation(t *testing.T) {
	dir := t.TempDir()
	writer := New(dir, 4)
	if err := writer.WriteResult(context.Background(), model.TestResult{}); err == nil {
		t.Fatal("expected empty result UUID error")
	}
	if err := writer.WriteContainer(context.Background(), model.TestResultContainer{}); err == nil {
		t.Fatal("expected empty container UUID error")
	}
	for _, name := range []string{"", ".", "../escape", "nested/file"} {
		if err := writer.write(context.Background(), name, strings.NewReader("x"), 1); err == nil {
			t.Fatalf("expected unsafe-name error for %q", name)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := writer.WriteAttachment(cancelled, "x-attachment.txt", strings.NewReader("x"), 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled write = %v", err)
	}
	if err := writer.WriteAttachment(context.Background(), "x-attachment.txt", strings.NewReader(""), -1); err == nil {
		t.Fatal("expected negative size error")
	}
	if err := writer.write(context.Background(), "short.txt", strings.NewReader("x"), 2); err == nil {
		t.Fatal("expected changed-size error")
	}
	if err := writer.write(context.Background(), "failure.txt", failingReader{}, -1); err == nil {
		t.Fatal("expected reader error")
	}
	if err := writer.write(context.Background(), "oversized-attachment.txt", io.LimitReader(strings.NewReader("12345"), 5), -1); err == nil {
		t.Fatal("expected streamed attachment limit error")
	}
	if err := writer.writeJSON(context.Background(), "bad.json", make(chan int)); err == nil {
		t.Fatal("expected JSON marshal error")
	}
	//lint:ignore SA1012 This deliberately verifies the documented nil-context compatibility path.
	if err := writer.write(nil, "nil-context.txt", strings.NewReader("ok"), 2); err != nil {
		t.Fatalf("nil context should be supported: %v", err)
	}
}

func TestPrepareAllowsExplicitCleaningAndRejectsFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "foreign")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "old-result.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(root, dir, true, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "old-result.json")); !os.IsNotExist(err) {
		t.Fatalf("owned artifact remains: %v", err)
	}
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(root, file, false, false); err == nil {
		t.Fatal("expected non-directory rejection")
	}
}

func TestPrepareRefusesDangerousAndNonOwned(t *testing.T) {
	root := t.TempDir()
	if err := Prepare(root, root, true, true); err == nil {
		t.Fatal("expected project-root refusal")
	}
	results := filepath.Join(root, "existing")
	if err := os.Mkdir(results, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(results, "data.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Prepare(root, results, true, false); err == nil {
		t.Fatal("expected non-owned refusal")
	}
}

func TestWriterAtomicallyRewritesAndBoundsAttachments(t *testing.T) {
	root := t.TempDir()
	results := filepath.Join(root, "results")
	if err := Prepare(root, results, false, false); err != nil {
		t.Fatal(err)
	}
	writer := New(results, 4)
	ctx := context.Background()
	result := model.TestResult{UUID: "abc", Name: "first", Status: model.StatusPassed}
	if err := writer.WriteResult(ctx, result); err != nil {
		t.Fatal(err)
	}
	result.Name = "second"
	if err := writer.WriteResult(ctx, result); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(results, "abc-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "second") {
		t.Fatalf("rewrite missing: %s", data)
	}
	if err := writer.WriteAttachment(ctx, "x-attachment.txt", strings.NewReader("12345"), 5); err == nil {
		t.Fatal("expected size limit")
	}
}
