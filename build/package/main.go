// SPDX-License-Identifier: Apache-2.0

// Command package creates one Gauge-compatible release archive.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type descriptor struct {
	Version string `json:"version"`
	ID      string `json:"id"`
}

func main() {
	binary := flag.String("binary", "", "compiled binary path")
	targetOS := flag.String("os", "", "target operating system")
	arch := flag.String("arch", "", "target Go architecture")
	version := flag.String("version", "0.1.0", "package semantic version without v")
	out := flag.String("out", "dist", "output directory")
	plugin := flag.String("plugin", "plugin.json", "plugin descriptor")
	flag.Parse()
	if err := run(*binary, *targetOS, *arch, *version, *out, *plugin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(binary, targetOS, arch, version, out, plugin string) error {
	if binary == "" || targetOS == "" || arch == "" {
		return errors.New("binary, os, and arch are required")
	}
	if !oneOf(targetOS, "windows", "linux", "darwin") || !oneOf(arch, "386", "amd64", "arm64") {
		return fmt.Errorf("unsupported target %s/%s", targetOS, arch)
	}
	if (targetOS == "darwin" && arch == "386") || (targetOS == "windows" && arch == "arm64") {
		return fmt.Errorf("unadvertised target %s/%s", targetOS, arch)
	}
	data, err := os.ReadFile(plugin)
	if err != nil {
		return fmt.Errorf("read plugin descriptor: %w", err)
	}
	var value descriptor
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("decode plugin descriptor: %w", err)
	}
	if value.ID != "allure-report" || value.Version != version {
		return fmt.Errorf("plugin.json id/version mismatch: %s %s", value.ID, value.Version)
	}
	info, err := os.Stat(binary)
	if err != nil {
		return fmt.Errorf("inspect binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("binary is not a regular file")
	}
	if err := verifyBinaryVersion(binary, version); err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	platformArch := map[string]string{"386": "x86", "amd64": "x86_64", "arm64": "arm64"}[arch]
	archiveName := fmt.Sprintf("allure-report-%s-%s.%s.zip", version, targetOS, platformArch)
	file, err := os.Create(filepath.Join(out, archiveName))
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(file.Name())
		}
	}()
	writer := zip.NewWriter(file)
	if err := addBytes(writer, "plugin.json", data, 0o644); err != nil {
		return err
	}
	binaryName := "bin/allure-report"
	if targetOS == "windows" {
		binaryName += ".exe"
	}
	if err := addFile(writer, binaryName, binary, 0o755); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	keep = true
	fmt.Println(filepath.Join(out, archiveName))
	return nil
}

func verifyBinaryVersion(path, expected string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read binary version marker: %w", err)
	}
	marker := []byte("gauge-allure-report-package-version:" + expected)
	if bytes.Contains(data, marker) {
		return nil
	}
	return fmt.Errorf("binary does not contain reporter package version marker %s", expected)
}

func addBytes(writer *zip.Writer, name string, data []byte, mode os.FileMode) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	header.SetMode(mode)
	target, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = target.Write(data)
	return err
}
func addFile(writer *zip.Writer, name, source string, mode os.FileMode) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	header := &zip.FileHeader{Name: strings.ReplaceAll(name, "\\", "/"), Method: zip.Deflate}
	header.SetMode(mode)
	target, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(target, file)
	return err
}
func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
