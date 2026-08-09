// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOneOf(t *testing.T) {
	if !oneOf("linux", "windows", "linux") || oneOf("plan9", "windows", "linux") {
		t.Fatal("oneOf returned an unexpected value")
	}
}

func TestRejectsUnadvertisedTargetBeforeReadingFiles(t *testing.T) {
	err := run("missing", "windows", "arm64", "0.1.0", t.TempDir(), "missing")
	if err == nil || err.Error() != "unadvertised target windows/arm64" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyBinaryVersionMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "binary")
	if err := os.WriteFile(path, []byte("prefix gauge-allure-report-package-version:1.2.3 suffix"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifyBinaryVersion(path, "1.2.3"); err != nil {
		t.Fatal(err)
	}
	if err := verifyBinaryVersion(path, "1.2.4"); err == nil {
		t.Fatal("expected mismatched version error")
	}
}
