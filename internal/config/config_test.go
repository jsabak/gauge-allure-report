// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	root := t.TempDir()
	gauge := filepath.Join(root, ".gauge")
	if err := os.MkdirAll(gauge, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "results_dir: yaml-results\nstrict: false\nmax_attachment_bytes: 1024\ncontext_mode: flat\n"
	if err := os.WriteFile(filepath.Join(gauge, "allure-report.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"GAUGE_ALLURE_RESULTS_DIR": "env-results", "GAUGE_ALLURE_STRICT": "true", "GAUGE_ALLURE_MAX_ATTACHMENT_BYTES": "2048"}
	cfg, err := Load(root, env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ResultsDir != filepath.Join(root, "env-results") || !cfg.Strict || cfg.MaxAttachmentBytes != 2048 || cfg.ContextMode != "flat" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestResultsDirectoryFallbacks(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name   string
		env    map[string]string
		suffix string
	}{
		{"allure", map[string]string{"ALLURE_RESULTS_DIR": "allure"}, "allure"},
		{"gauge reports", map[string]string{"gauge_reports_dir": "custom"}, filepath.Join("custom", "allure-results")},
		{"default", map[string]string{}, filepath.Join("reports", "allure-results")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := Load(root, test.env)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.ResultsDir != filepath.Join(root, test.suffix) {
				t.Fatalf("got %s", cfg.ResultsDir)
			}
		})
	}
}

func TestLoadRejectsUnknownYAMLAndInvalidValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.yaml")
	if err := os.WriteFile(path, []byte("unknown_setting: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, map[string]string{"GAUGE_ALLURE_CONFIG": path}); err == nil {
		t.Fatal("expected unknown field error")
	}
	if _, err := Load(root, map[string]string{"GAUGE_ALLURE_COMPATIBILITY": "future"}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestAllEnvironmentOverrides(t *testing.T) {
	root := t.TempDir()
	env := map[string]string{
		"GAUGE_ALLURE_RESULTS_DIR": "custom", "GAUGE_ALLURE_COMPATIBILITY": "allure3", "GAUGE_ALLURE_WRITE_MODE": "suite-final",
		"GAUGE_ALLURE_LOG_LEVEL": "debug", "GAUGE_ALLURE_CONTEXT_MODE": "fixtures", "GAUGE_ALLURE_TEARDOWN_MODE": "flat",
		"GAUGE_ALLURE_HOOK_MODE": "fixtures", "GAUGE_ALLURE_CATEGORIES_FILE": "categories.json", "GAUGE_ALLURE_CATEGORIES_PROFILE": "gauge-default",
		"ALLURE_LINK_ISSUE_PATTERN": "https://issues/{}/", "ALLURE_LINK_TMS_PATTERN": "https://tms/{}/", "ALLURE_LINK_LINK_PATTERN": "https://links/{}/",
		"GAUGE_ALLURE_CLEAN_RESULTS": "false", "GAUGE_ALLURE_ALLOW_CLEAN_NON_OWNED": "true", "GAUGE_ALLURE_STRICT": "true",
		"GAUGE_ALLURE_ATTACH_SCREENSHOTS": "false", "GAUGE_ALLURE_ATTACH_MESSAGES": "false", "GAUGE_ALLURE_ATTACH_TABLES": "false",
		"GAUGE_ALLURE_FOLLOW_SYMLINKS": "true", "GAUGE_ALLURE_ALLOW_EXTERNAL_ATTACHMENTS": "true", "GAUGE_ALLURE_EXECUTOR_AUTO": "false",
		"GAUGE_ALLURE_DIAGNOSTIC_PROTO_DUMP": "true", "GAUGE_ALLURE_MAX_ATTACHMENT_BYTES": "4096", "GAUGE_ALLURE_MAX_NESTING_DEPTH": "17",
		"GAUGE_ALLURE_ENV_ALLOWLIST": "CI, BUILD_ID;REGION", "GAUGE_ALLURE_PROMOTE_PARAMETERS": "customer",
		"GAUGE_ALLURE_MASK_PARAMETERS": "password", "GAUGE_ALLURE_HIDE_PARAMETERS": "token", "GAUGE_ALLURE_EXCLUDE_HISTORY_PARAMETERS": "timestamp",
		"GAUGE_ALLURE_REDACT": "secret;credential",
	}
	cfg, err := Load(root, env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ResultsDir != filepath.Join(root, "custom") || cfg.Compatibility != "allure3" || cfg.WriteMode != "suite-final" || cfg.LogLevel != "debug" {
		t.Fatalf("string overrides not applied: %+v", cfg)
	}
	if cfg.CleanResults || !cfg.AllowCleanNonOwned || !cfg.Strict || cfg.AttachScreenshots || cfg.AttachMessages || cfg.AttachTables || !cfg.FollowSymlinks || !cfg.AllowExternalAttachments || cfg.ExecutorAuto || !cfg.DiagnosticProtoDump {
		t.Fatalf("boolean overrides not applied: %+v", cfg)
	}
	if cfg.MaxAttachmentBytes != 4096 || cfg.MaximumNestingDepth != 17 || len(cfg.EnvironmentAllowlist) != 3 || len(cfg.Redact) != 2 {
		t.Fatalf("numeric/list overrides not applied: %+v", cfg)
	}
}

func TestEnvironmentAndLegacyOverwrite(t *testing.T) {
	t.Setenv("GAUGE_ALLURE_TEST_SENTINEL", "present")
	if Environment()["GAUGE_ALLURE_TEST_SENTINEL"] != "present" {
		t.Fatal("environment snapshot omitted sentinel")
	}
	cfg, err := Load(t.TempDir(), map[string]string{"overwrite_reports": "false"})
	if err != nil || cfg.CleanResults {
		t.Fatalf("legacy overwrite override: cfg=%+v err=%v", cfg, err)
	}
	cfg, err = Load(t.TempDir(), map[string]string{"overwrite_reports": "not-a-bool"})
	if err != nil || !cfg.CleanResults {
		t.Fatalf("invalid legacy value should be ignored: cfg=%+v err=%v", cfg, err)
	}
}

func TestLoadFileErrors(t *testing.T) {
	root := t.TempDir()
	if _, err := Load(root, map[string]string{"GAUGE_ALLURE_CONFIG": "missing.yaml"}); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("missing explicit file error = %v", err)
	}
	directory := filepath.Join(root, "config-dir")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, map[string]string{"GAUGE_ALLURE_CONFIG": directory}); err == nil || !strings.Contains(err.Error(), "read configuration") {
		t.Fatalf("directory read error = %v", err)
	}
}

func TestInvalidEnvironmentValues(t *testing.T) {
	tests := []map[string]string{
		{"GAUGE_ALLURE_STRICT": "perhaps"},
		{"GAUGE_ALLURE_MAX_ATTACHMENT_BYTES": "large"},
		{"GAUGE_ALLURE_MAX_NESTING_DEPTH": "deep"},
	}
	for _, env := range tests {
		if _, err := Load(t.TempDir(), env); err == nil {
			t.Fatalf("expected error for %v", env)
		}
	}
}

func TestValidateEveryConstraint(t *testing.T) {
	base := Defaults()
	base.ResultsDir = "results"
	tests := []struct {
		name string
		edit func(*Config)
	}{
		{"empty project", func(*Config) {}},
		{"empty results", func(c *Config) { c.ResultsDir = "" }},
		{"compatibility", func(c *Config) { c.Compatibility = "invalid" }},
		{"write mode", func(c *Config) { c.WriteMode = "invalid" }},
		{"log level", func(c *Config) { c.LogLevel = "trace" }},
		{"context", func(c *Config) { c.ContextMode = "invalid" }},
		{"teardown", func(c *Config) { c.TeardownMode = "invalid" }},
		{"hooks", func(c *Config) { c.HookMode = "steps" }},
		{"attachment limit", func(c *Config) { c.MaxAttachmentBytes = 0 }},
		{"nesting low", func(c *Config) { c.MaximumNestingDepth = 0 }},
		{"nesting high", func(c *Config) { c.MaximumNestingDepth = 1001 }},
		{"categories", func(c *Config) { c.CategoriesProfile = "custom" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := base
			test.edit(&cfg)
			root := "/project"
			if test.name == "empty project" {
				root = " "
			}
			if err := cfg.Validate(root); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
