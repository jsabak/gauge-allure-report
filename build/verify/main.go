// SPDX-License-Identifier: Apache-2.0

// Command verify checks the exact contents and metadata of a Gauge plugin ZIP.
package main

import (
	"archive/zip"
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
	ID           string   `json:"id"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

func main() {
	archive := flag.String("archive", "", "archive to verify")
	version := flag.String("version", "0.1.0", "expected version")
	flag.Parse()
	if err := verify(*archive, *version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("verified %s\n", *archive)
}
func verify(path, version string) error {
	if path == "" {
		return errors.New("archive is required")
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer reader.Close()
	windows := strings.Contains(strings.ToLower(filepath.Base(path)), "windows")
	binary := "bin/allure-report"
	if windows {
		binary += ".exe"
	}
	want := map[string]bool{"plugin.json": false, binary: false}
	var pluginData []byte
	for _, file := range reader.File {
		if _, ok := want[file.Name]; !ok {
			return fmt.Errorf("unexpected archive entry %q", file.Name)
		}
		if want[file.Name] {
			return fmt.Errorf("duplicate archive entry %q", file.Name)
		}
		want[file.Name] = true
		if file.Name == "plugin.json" {
			source, openErr := file.Open()
			if openErr != nil {
				return openErr
			}
			pluginData, err = io.ReadAll(source)
			closeErr := source.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
		if file.Name == binary && !windows && file.Mode()&0o111 == 0 {
			return errors.New("unix binary is not executable")
		}
	}
	for name, found := range want {
		if !found {
			return fmt.Errorf("missing archive entry %q", name)
		}
	}
	var plugin descriptor
	if err := json.Unmarshal(pluginData, &plugin); err != nil {
		return err
	}
	if plugin.ID != "allure-report" || plugin.Version != version {
		return fmt.Errorf("descriptor mismatch: %+v", plugin)
	}
	grpc := false
	for _, capability := range plugin.Capabilities {
		if capability == "grpc_support" {
			grpc = true
		}
	}
	if !grpc {
		return errors.New("grpc_support capability is missing")
	}
	return nil
}
