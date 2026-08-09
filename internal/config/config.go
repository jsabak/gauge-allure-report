// SPDX-License-Identifier: Apache-2.0

// Package config loads and validates the reporter's typed YAML/environment configuration.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultRelativeResults = "reports/allure-results"

// Config is the complete first-release reporter configuration.
type Config struct {
	ResultsDir               string            `yaml:"results_dir" json:"results_dir"`
	CleanResults             bool              `yaml:"clean_results" json:"clean_results"`
	AllowCleanNonOwned       bool              `yaml:"allow_clean_non_owned" json:"allow_clean_non_owned"`
	Compatibility            string            `yaml:"compatibility" json:"compatibility"`
	WriteMode                string            `yaml:"write_mode" json:"write_mode"`
	Strict                   bool              `yaml:"strict" json:"strict"`
	LogLevel                 string            `yaml:"log_level" json:"log_level"`
	AttachScreenshots        bool              `yaml:"attach_screenshots" json:"attach_screenshots"`
	AttachMessages           bool              `yaml:"attach_messages" json:"attach_messages"`
	AttachTables             bool              `yaml:"attach_tables" json:"attach_tables"`
	MaxAttachmentBytes       int64             `yaml:"max_attachment_bytes" json:"max_attachment_bytes"`
	FollowSymlinks           bool              `yaml:"follow_symlinks" json:"follow_symlinks"`
	AllowExternalAttachments bool              `yaml:"allow_external_attachments" json:"allow_external_attachments"`
	ContextMode              string            `yaml:"context_mode" json:"context_mode"`
	TeardownMode             string            `yaml:"teardown_mode" json:"teardown_mode"`
	HookMode                 string            `yaml:"hook_mode" json:"hook_mode"`
	BehaviorFromSpec         bool              `yaml:"behavior_from_spec" json:"behavior_from_spec"`
	IssueLinkPattern         string            `yaml:"issue_link_pattern" json:"issue_link_pattern"`
	TMSLinkPattern           string            `yaml:"tms_link_pattern" json:"tms_link_pattern"`
	GenericLinkPattern       string            `yaml:"generic_link_pattern" json:"generic_link_pattern"`
	EnvironmentAllowlist     []string          `yaml:"environment_allowlist" json:"environment_allowlist"`
	PromoteParameters        []string          `yaml:"promote_parameters" json:"promote_parameters"`
	MaskParameters           []string          `yaml:"mask_parameters" json:"mask_parameters"`
	HideParameters           []string          `yaml:"hide_parameters" json:"hide_parameters"`
	ExcludeHistoryParameters []string          `yaml:"exclude_history_parameters" json:"exclude_history_parameters"`
	Redact                   []string          `yaml:"redact" json:"redact"`
	ExecutorAuto             bool              `yaml:"executor_auto" json:"executor_auto"`
	Executor                 ExecutorOverrides `yaml:"executor" json:"executor"`
	CategoriesFile           string            `yaml:"categories_file" json:"categories_file"`
	CategoriesProfile        string            `yaml:"categories_profile" json:"categories_profile"`
	SourceLinkTemplate       string            `yaml:"source_link_template" json:"source_link_template"`
	MaximumNestingDepth      int               `yaml:"maximum_nesting_depth" json:"maximum_nesting_depth"`
	DiagnosticProtoDump      bool              `yaml:"diagnostic_proto_dump" json:"diagnostic_proto_dump"`
	SourcePath               string            `yaml:"-" json:"-"`
}

// ExecutorOverrides contains explicitly safe executor.json overrides.
type ExecutorOverrides struct {
	Name       string `yaml:"name" json:"name"`
	Type       string `yaml:"type" json:"type"`
	BuildName  string `yaml:"build_name" json:"build_name"`
	BuildURL   string `yaml:"build_url" json:"build_url"`
	ReportName string `yaml:"report_name" json:"report_name"`
	ReportURL  string `yaml:"report_url" json:"report_url"`
}

// Defaults returns conservative cross-version defaults.
func Defaults() Config {
	return Config{
		CleanResults:        true,
		Compatibility:       "allure2-and-3",
		WriteMode:           "streaming",
		LogLevel:            "info",
		AttachScreenshots:   true,
		AttachMessages:      true,
		AttachTables:        true,
		MaxAttachmentBytes:  100 * 1024 * 1024,
		ContextMode:         "grouped-steps",
		TeardownMode:        "grouped-steps",
		HookMode:            "fixtures",
		ExecutorAuto:        true,
		CategoriesProfile:   "none",
		MaximumNestingDepth: 100,
	}
}

// Load reads defaults, an optional YAML file, and process-environment overrides.
func Load(projectRoot string, env map[string]string) (Config, error) {
	if env == nil {
		env = Environment()
	}
	cfg := Defaults()
	path := strings.TrimSpace(env["GAUGE_ALLURE_CONFIG"])
	if path == "" {
		path = filepath.Join(projectRoot, ".gauge", "allure-report.yaml")
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(projectRoot, path)
	}
	if data, err := os.ReadFile(path); err == nil {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("decode configuration %s: %w", path, err)
		}
		cfg.SourcePath = path
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read configuration %s: %w", path, err)
	} else if env["GAUGE_ALLURE_CONFIG"] != "" {
		return Config{}, fmt.Errorf("configured file does not exist: %s", path)
	}
	if err := applyEnvironment(&cfg, env); err != nil {
		return Config{}, err
	}
	if cfg.ResultsDir == "" {
		switch {
		case env["GAUGE_ALLURE_RESULTS_DIR"] != "":
			cfg.ResultsDir = env["GAUGE_ALLURE_RESULTS_DIR"]
		case env["ALLURE_RESULTS_DIR"] != "":
			cfg.ResultsDir = env["ALLURE_RESULTS_DIR"]
		case env["gauge_reports_dir"] != "":
			cfg.ResultsDir = filepath.Join(env["gauge_reports_dir"], "allure-results")
		default:
			cfg.ResultsDir = defaultRelativeResults
		}
	}
	if !filepath.IsAbs(cfg.ResultsDir) {
		cfg.ResultsDir = filepath.Join(projectRoot, cfg.ResultsDir)
	}
	cfg.ResultsDir = filepath.Clean(cfg.ResultsDir)
	if err := cfg.Validate(projectRoot); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Validate checks the configuration without mutating the filesystem.
func (c Config) Validate(projectRoot string) error {
	if strings.TrimSpace(projectRoot) == "" {
		return errors.New("GAUGE project root is empty")
	}
	if c.ResultsDir == "" {
		return errors.New("results_dir is empty")
	}
	if !oneOf(c.Compatibility, "allure2-and-3", "allure3") {
		return fmt.Errorf("compatibility must be allure2-and-3 or allure3, got %q", c.Compatibility)
	}
	if !oneOf(c.WriteMode, "streaming", "suite-final") {
		return fmt.Errorf("write_mode must be streaming or suite-final, got %q", c.WriteMode)
	}
	if !oneOf(c.LogLevel, "debug", "info", "warn", "error") {
		return fmt.Errorf("unsupported log_level %q", c.LogLevel)
	}
	if !oneOf(c.ContextMode, "grouped-steps", "flat", "fixtures") {
		return fmt.Errorf("unsupported context_mode %q", c.ContextMode)
	}
	if !oneOf(c.TeardownMode, "grouped-steps", "flat", "fixtures") {
		return fmt.Errorf("unsupported teardown_mode %q", c.TeardownMode)
	}
	if c.HookMode != "fixtures" {
		return fmt.Errorf("hook_mode must be fixtures in this release, got %q", c.HookMode)
	}
	if c.MaxAttachmentBytes <= 0 {
		return errors.New("max_attachment_bytes must be positive")
	}
	if c.MaximumNestingDepth < 1 || c.MaximumNestingDepth > 1000 {
		return errors.New("maximum_nesting_depth must be between 1 and 1000")
	}
	if !oneOf(c.CategoriesProfile, "none", "gauge-default") {
		return fmt.Errorf("unsupported categories_profile %q", c.CategoriesProfile)
	}
	return nil
}

// Environment snapshots process environment without retaining duplicate keys.
func Environment() map[string]string {
	values := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, found := strings.Cut(item, "=")
		if found {
			values[key] = value
		}
	}
	return values
}

func applyEnvironment(cfg *Config, env map[string]string) error {
	stringsMap := map[string]*string{
		"GAUGE_ALLURE_RESULTS_DIR":        &cfg.ResultsDir,
		"GAUGE_ALLURE_COMPATIBILITY":      &cfg.Compatibility,
		"GAUGE_ALLURE_WRITE_MODE":         &cfg.WriteMode,
		"GAUGE_ALLURE_LOG_LEVEL":          &cfg.LogLevel,
		"GAUGE_ALLURE_CONTEXT_MODE":       &cfg.ContextMode,
		"GAUGE_ALLURE_TEARDOWN_MODE":      &cfg.TeardownMode,
		"GAUGE_ALLURE_HOOK_MODE":          &cfg.HookMode,
		"GAUGE_ALLURE_CATEGORIES_FILE":    &cfg.CategoriesFile,
		"GAUGE_ALLURE_CATEGORIES_PROFILE": &cfg.CategoriesProfile,
		"ALLURE_LINK_ISSUE_PATTERN":       &cfg.IssueLinkPattern,
		"ALLURE_LINK_TMS_PATTERN":         &cfg.TMSLinkPattern,
		"ALLURE_LINK_LINK_PATTERN":        &cfg.GenericLinkPattern,
	}
	for key, target := range stringsMap {
		if value, ok := env[key]; ok && value != "" {
			*target = value
		}
	}
	bools := map[string]*bool{
		"GAUGE_ALLURE_CLEAN_RESULTS":              &cfg.CleanResults,
		"GAUGE_ALLURE_ALLOW_CLEAN_NON_OWNED":      &cfg.AllowCleanNonOwned,
		"GAUGE_ALLURE_STRICT":                     &cfg.Strict,
		"GAUGE_ALLURE_ATTACH_SCREENSHOTS":         &cfg.AttachScreenshots,
		"GAUGE_ALLURE_ATTACH_MESSAGES":            &cfg.AttachMessages,
		"GAUGE_ALLURE_ATTACH_TABLES":              &cfg.AttachTables,
		"GAUGE_ALLURE_FOLLOW_SYMLINKS":            &cfg.FollowSymlinks,
		"GAUGE_ALLURE_ALLOW_EXTERNAL_ATTACHMENTS": &cfg.AllowExternalAttachments,
		"GAUGE_ALLURE_EXECUTOR_AUTO":              &cfg.ExecutorAuto,
		"GAUGE_ALLURE_DIAGNOSTIC_PROTO_DUMP":      &cfg.DiagnosticProtoDump,
	}
	for key, target := range bools {
		if raw, ok := env[key]; ok && raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				return fmt.Errorf("parse %s: %w", key, err)
			}
			*target = value
		}
	}
	if raw := env["GAUGE_ALLURE_MAX_ATTACHMENT_BYTES"]; raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("parse GAUGE_ALLURE_MAX_ATTACHMENT_BYTES: %w", err)
		}
		cfg.MaxAttachmentBytes = value
	}
	if raw := env["GAUGE_ALLURE_MAX_NESTING_DEPTH"]; raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("parse GAUGE_ALLURE_MAX_NESTING_DEPTH: %w", err)
		}
		cfg.MaximumNestingDepth = value
	}
	listValues := map[string]*[]string{
		"GAUGE_ALLURE_ENV_ALLOWLIST":              &cfg.EnvironmentAllowlist,
		"GAUGE_ALLURE_PROMOTE_PARAMETERS":         &cfg.PromoteParameters,
		"GAUGE_ALLURE_MASK_PARAMETERS":            &cfg.MaskParameters,
		"GAUGE_ALLURE_HIDE_PARAMETERS":            &cfg.HideParameters,
		"GAUGE_ALLURE_EXCLUDE_HISTORY_PARAMETERS": &cfg.ExcludeHistoryParameters,
		"GAUGE_ALLURE_REDACT":                     &cfg.Redact,
	}
	for key, target := range listValues {
		if raw, ok := env[key]; ok {
			*target = splitList(raw)
		}
	}
	if raw, ok := env["overwrite_reports"]; ok && env["GAUGE_ALLURE_CLEAN_RESULTS"] == "" {
		value, err := strconv.ParseBool(raw)
		if err == nil {
			cfg.CleanResults = value
		}
	}
	return nil
}

func splitList(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ';' })
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
