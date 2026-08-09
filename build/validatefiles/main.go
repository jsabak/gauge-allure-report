// SPDX-License-Identifier: Apache-2.0

// Command validatefiles parses every repository-owned JSON and YAML file.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	count, err := validate(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("validated %d JSON/YAML files\n", count)
}

func validate(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != root && ignoredDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".json" && extension != ".yaml" && extension != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if extension == ".json" {
			err = decodeJSON(data)
		} else {
			err = decodeYAML(data)
		}
		if err != nil {
			return fmt.Errorf("validate %s: %w", path, err)
		}
		count++
		return nil
	})
	return count, err
}

func ignoredDirectory(name string) bool {
	switch name {
	case ".git", ".tools", "bin", "dist", "node_modules", "allure-results", "allure-report":
		return true
	default:
		return false
	}
}

func decodeJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if err := decoder.Decode(&value); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func decodeYAML(data []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var value any
		if err := decoder.Decode(&value); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}
