// SPDX-License-Identifier: Apache-2.0

// Package attachments securely imports Gauge messages and screenshots.
package attachments

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/allure-framework/allure-go/commons/model"
	"github.com/jsabak/gauge-allure-report/internal/identity"
	"github.com/jsabak/gauge-allure-report/internal/output"
)

var safeExtension = regexp.MustCompile(`^\.[A-Za-z0-9]{1,10}$`)

// Manager enforces the attachment security policy before invoking the writer.
type Manager struct {
	writer        output.Writer
	uuids         identity.UUIDGenerator
	projectRoot   string
	searchRoots   []string
	maximum       int64
	followLinks   bool
	allowExternal bool
	redactions    []string
}

// New creates an attachment manager.
func New(writer output.Writer, uuids identity.UUIDGenerator, projectRoot string, maximum int64, followLinks, allowExternal bool, redactions []string) *Manager {
	return &Manager{writer: writer, uuids: uuids, projectRoot: filepath.Clean(projectRoot), maximum: maximum, followLinks: followLinks, allowExternal: allowExternal, redactions: append([]string(nil), redactions...)}
}

// WithSearchRoots adds Gauge-managed locations used when a runner reports only
// an attachment basename (notably gauge_screenshots_dir). Unsafe external roots
// remain subject to the same containment policy as direct paths.
func (m *Manager) WithSearchRoots(roots ...string) *Manager {
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		if !filepath.IsAbs(root) {
			root = filepath.Join(m.projectRoot, root)
		}
		m.searchRoots = append(m.searchRoots, filepath.Clean(root))
	}
	return m
}

// Copy validates and streams a regular file into the Allure results directory.
func (m *Manager) Copy(ctx context.Context, displayName, source string) (model.Attachment, error) {
	if strings.TrimSpace(source) == "" {
		return model.Attachment{}, errors.New("attachment path is empty")
	}
	path := m.resolve(source)
	info, err := os.Lstat(path)
	if err != nil {
		return model.Attachment{}, fmt.Errorf("inspect attachment %s: %w", filepath.Base(path), err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		if !m.followLinks {
			return model.Attachment{}, fmt.Errorf("refusing symlink attachment %s", filepath.Base(path))
		}
		path, err = filepath.EvalSymlinks(path)
		if err != nil {
			return model.Attachment{}, fmt.Errorf("resolve attachment symlink: %w", err)
		}
		info, err = os.Stat(path)
		if err != nil {
			return model.Attachment{}, fmt.Errorf("stat resolved attachment: %w", err)
		}
	}
	if !info.Mode().IsRegular() {
		return model.Attachment{}, fmt.Errorf("attachment is not a regular file: %s", filepath.Base(path))
	}
	if !m.allowExternal && !within(m.projectRoot, path) {
		return model.Attachment{}, fmt.Errorf("attachment is outside GAUGE_PROJECT_ROOT: %s", filepath.Base(path))
	}
	if info.Size() > m.maximum {
		return model.Attachment{}, fmt.Errorf("attachment %s is %d bytes; limit is %d", filepath.Base(path), info.Size(), m.maximum)
	}
	file, err := os.Open(path)
	if err != nil {
		return model.Attachment{}, fmt.Errorf("open attachment: %w", err)
	}
	defer file.Close()
	mediaType, extension, err := sniff(file, path)
	if err != nil {
		return model.Attachment{}, err
	}
	id, err := m.uuids.New()
	if err != nil {
		return model.Attachment{}, err
	}
	name := id + "-attachment" + extension
	if err := m.writer.WriteAttachment(ctx, name, file, info.Size()); err != nil {
		return model.Attachment{}, err
	}
	return model.Attachment{Name: displayName, Type: mediaType, Source: name}, nil
}

func (m *Manager) resolve(source string) string {
	if filepath.IsAbs(source) {
		return filepath.Clean(source)
	}
	for _, root := range append([]string{m.projectRoot}, m.searchRoots...) {
		candidate := filepath.Clean(filepath.Join(root, source))
		if _, err := os.Lstat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Clean(filepath.Join(m.projectRoot, source))
}

// Bytes writes a bounded in-memory attachment, including legacy screenshot bytes.
func (m *Manager) Bytes(ctx context.Context, displayName, mediaType, extension string, data []byte) (model.Attachment, error) {
	if int64(len(data)) > m.maximum {
		return model.Attachment{}, fmt.Errorf("attachment %s is %d bytes; limit is %d", displayName, len(data), m.maximum)
	}
	if mediaType == "" {
		mediaType = http.DetectContentType(data)
	}
	if !safeExtension.MatchString(extension) {
		extensions, _ := mime.ExtensionsByType(mediaType)
		if len(extensions) > 0 && safeExtension.MatchString(extensions[0]) {
			extension = extensions[0]
		} else {
			extension = ".bin"
		}
	}
	id, err := m.uuids.New()
	if err != nil {
		return model.Attachment{}, err
	}
	name := id + "-attachment" + strings.ToLower(extension)
	if err := m.writer.WriteAttachment(ctx, name, bytes.NewReader(data), int64(len(data))); err != nil {
		return model.Attachment{}, err
	}
	return model.Attachment{Name: displayName, Type: mediaType, Source: name}, nil
}

// Messages aggregates scope messages into one redacted UTF-8 attachment.
func (m *Manager) Messages(ctx context.Context, displayName string, messages []string) (model.Attachment, error) {
	if len(messages) == 0 {
		return model.Attachment{}, errors.New("no messages to attach")
	}
	value := strings.Join(messages, "\n")
	for _, secret := range m.redactions {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return m.Bytes(ctx, displayName, "text/plain; charset=utf-8", ".txt", []byte(value+"\n"))
}

func sniff(file *os.File, path string) (string, string, error) {
	buffer := make([]byte, 512)
	count, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", "", fmt.Errorf("read attachment header: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", "", fmt.Errorf("rewind attachment: %w", err)
	}
	extension := strings.ToLower(filepath.Ext(path))
	mediaType := mime.TypeByExtension(extension)
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = http.DetectContentType(buffer[:count])
	}
	if !safeExtension.MatchString(extension) {
		extensions, _ := mime.ExtensionsByType(mediaType)
		if len(extensions) > 0 && safeExtension.MatchString(extensions[0]) {
			extension = extensions[0]
		} else {
			extension = ".bin"
		}
	}
	return mediaType, extension, nil
}

func within(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
