// SPDX-License-Identifier: Apache-2.0

// Package identity creates stable Gauge test identities and per-attempt UUIDs.
package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// UUIDGenerator allows deterministic tests without weakening production UUIDs.
type UUIDGenerator interface {
	New() (string, error)
}

// RandomUUID creates RFC 4122 version 4 UUIDs from crypto/rand.
type RandomUUID struct{}

func (RandomUUID) New() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

var windowsAbs = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

// CanonicalPath returns a slash-normalized path relative to projectRoot. It
// never returns an absolute checkout path.
func CanonicalPath(projectRoot, name string) string {
	cleanName := strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	cleanRoot := strings.TrimSuffix(strings.ReplaceAll(filepath.Clean(projectRoot), "\\", "/"), "/")
	if cleanRoot != "." && cleanRoot != "" {
		lowerName, lowerRoot := strings.ToLower(cleanName), strings.ToLower(cleanRoot)
		if lowerName == lowerRoot {
			return "."
		}
		if strings.HasPrefix(lowerName, lowerRoot+"/") {
			cleanName = cleanName[len(cleanRoot)+1:]
		}
	}
	cleanName = strings.TrimPrefix(cleanName, "./")
	if strings.HasPrefix(cleanName, "/") || windowsAbs.MatchString(cleanName) {
		cleanName = filepath.Base(cleanName)
	}
	parts := make([]string, 0)
	for _, part := range strings.Split(cleanName, "/") {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(parts) > 0 {
				parts = parts[:len(parts)-1]
			}
		default:
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return "unknown.spec"
	}
	return strings.Join(parts, "/")
}

// Scenario returns the human-readable canonical identity of a Gauge scenario.
func Scenario(project, specPath, scenarioID, heading string, start, end int64) string {
	project = normalizeText(project)
	if project == "" {
		project = "Gauge project"
	}
	stable := strings.TrimSpace(scenarioID)
	if stable == "" {
		stable = fmt.Sprintf("L%d-L%d:%s", start, end, normalizeText(heading))
	}
	return project + "::" + specPath + "::" + stable
}

// TestCaseID is a SHA-256 digest of the canonical scenario identity.
func TestCaseID(canonical string) string { return digest(canonical) }

// HistoryID includes ordered logical parameters while deliberately excluding
// retry and stream allocation metadata.
func HistoryID(testCaseID string, parameters [][2]string) string {
	var builder strings.Builder
	builder.WriteString(testCaseID)
	for _, parameter := range parameters {
		builder.WriteByte(0)
		builder.WriteString(parameter[0])
		builder.WriteByte('=')
		builder.WriteString(parameter[1])
	}
	return digest(builder.String())
}

// AttemptKey identifies a concrete retry and table row within one launch.
func AttemptKey(canonical string, specRow, scenarioRow, retry int32) string {
	return fmt.Sprintf("%s#spec-row=%d#scenario-row=%d#retry=%d", canonical, specRow, scenarioRow, retry)
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
