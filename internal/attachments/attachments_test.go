// SPDX-License-Identifier: Apache-2.0

package attachments

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/allure-framework/allure-go/commons/model"
)

type fixedUUID struct{ value string }

func (f fixedUUID) New() (string, error) { return f.value, nil }

type memoryWriter struct {
	mu    sync.Mutex
	names []string
	data  map[string][]byte
}

func (w *memoryWriter) WriteResult(context.Context, model.TestResult) error             { return nil }
func (w *memoryWriter) WriteContainer(context.Context, model.TestResultContainer) error { return nil }
func (w *memoryWriter) WriteEnvironment(context.Context, map[string]string) error       { return nil }
func (w *memoryWriter) WriteExecutor(context.Context, model.Executor) error             { return nil }
func (w *memoryWriter) WriteCategories(context.Context, []model.Category) error         { return nil }
func (w *memoryWriter) WriteAttachment(_ context.Context, name string, reader io.Reader, _ int64) error {
	data, err := io.ReadAll(reader)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.data == nil {
		w.data = map[string][]byte{}
	}
	w.names = append(w.names, name)
	w.data[name] = data
	return err
}

func TestCopyAndMessages(t *testing.T) {
	root := t.TempDir()
	imagePath := filepath.Join(root, "tiny.png")
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0}
	if err := os.WriteFile(imagePath, png, 0o644); err != nil {
		t.Fatal(err)
	}
	writer := &memoryWriter{}
	manager := New(writer, fixedUUID{"11111111-1111-4111-8111-111111111111"}, root, 1024, false, false, []string{"secret"})
	attachment, err := manager.Copy(context.Background(), "Screenshot 1", imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if attachment.Type != "image/png" || !strings.HasSuffix(attachment.Source, ".png") {
		t.Fatalf("attachment: %+v", attachment)
	}
	message, err := manager.Messages(context.Background(), "Messages", []string{"a secret", "line 2"})
	if err != nil {
		t.Fatal(err)
	}
	if string(writer.data[message.Source]) != "a [REDACTED]\nline 2\n" {
		t.Fatalf("message: %q", writer.data[message.Source])
	}
}

func TestCopyRejectsExternalOversizedAndDirectory(t *testing.T) {
	root := t.TempDir()
	externalRoot := t.TempDir()
	external := filepath.Join(externalRoot, "x.txt")
	if err := os.WriteFile(external, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := New(&memoryWriter{}, fixedUUID{"id"}, root, 4, false, false, nil)
	if _, err := manager.Copy(context.Background(), "external", external); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("external error: %v", err)
	}
	inside := filepath.Join(root, "large.txt")
	if err := os.WriteFile(inside, []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Copy(context.Background(), "large", inside); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("size error: %v", err)
	}
	if _, err := manager.Copy(context.Background(), "dir", root); err == nil {
		t.Fatal("expected directory refusal")
	}
}

func TestCopyRejectsSymlinkByDefault(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	link := filepath.Join(root, "link.txt")
	if err := os.WriteFile(target, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	manager := New(&memoryWriter{}, fixedUUID{"id"}, root, 10, false, false, nil)
	if _, err := manager.Copy(context.Background(), "link", link); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink error: %v", err)
	}
}

func TestCopyResolvesGaugeScreenshotBasename(t *testing.T) {
	root := t.TempDir()
	screenshots := filepath.Join(root, ".gauge", "screenshots")
	if err := os.MkdirAll(screenshots, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(screenshots, "tiny.png"), []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := New(&memoryWriter{}, fixedUUID{"id"}, root, 1024, false, false, nil).WithSearchRoots(screenshots)
	attachment, err := manager.Copy(context.Background(), "Screenshot", "tiny.png")
	if err != nil {
		t.Fatal(err)
	}
	if attachment.Type != "image/png" {
		t.Fatalf("attachment: %+v", attachment)
	}
}
