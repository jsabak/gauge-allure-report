// SPDX-License-Identifier: Apache-2.0

// Package app wires CLI modes and Gauge-launched execution mode.
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jsabak/gauge-allure-report/internal/attachments"
	"github.com/jsabak/gauge-allure-report/internal/collector"
	"github.com/jsabak/gauge-allure-report/internal/config"
	"github.com/jsabak/gauge-allure-report/internal/identity"
	"github.com/jsabak/gauge-allure-report/internal/logging"
	"github.com/jsabak/gauge-allure-report/internal/mapping"
	"github.com/jsabak/gauge-allure-report/internal/metadata"
	"github.com/jsabak/gauge-allure-report/internal/output"
	"github.com/jsabak/gauge-allure-report/internal/server"
	"github.com/jsabak/gauge-allure-report/internal/version"
)

const actionEnvironment = "allure-report_action"

// Run executes one process mode and returns the boundary exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "--version", "-version", "version":
			fmt.Fprintln(stdout, version.String())
			return 0
		case "validate-config":
			return validateConfig(args[1:], stdout, stderr)
		case "doctor":
			return doctor(stdout, stderr)
		case "help", "--help", "-h":
			usage(stdout)
			return 0
		default:
			fmt.Fprintf(stderr, "unknown command %q\n", args[0])
			usage(stderr)
			return 2
		}
	}
	if os.Getenv(actionEnvironment) != "execution" {
		usage(stderr)
		fmt.Fprintf(stderr, "\nGauge execution mode requires %s=execution\n", actionEnvironment)
		return 2
	}
	projectRoot, err := detectProjectRoot()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.Chdir(projectRoot); err != nil {
		fmt.Fprintf(stderr, "change to GAUGE_PROJECT_ROOT: %v\n", err)
		return 1
	}
	env := config.Environment()
	cfg, err := config.Load(projectRoot, env)
	if err != nil {
		fmt.Fprintf(stderr, "configuration error: %v\n", err)
		return 1
	}
	if !cfg.CleanResults {
		cfg.ResultsDir = filepath.Join(cfg.ResultsDir, time.Now().Format("2006-01-02_15.04.05.000"))
	}
	if err := output.Prepare(projectRoot, cfg.ResultsDir, cfg.CleanResults, cfg.AllowCleanNonOwned); err != nil {
		fmt.Fprintf(stderr, "prepare output: %v\n", err)
		return 1
	}
	logger := logging.New(stderr, cfg.LogLevel)
	writer := output.New(cfg.ResultsDir, cfg.MaxAttachmentBytes)
	uuids := identity.RandomUUID{}
	manager := attachments.New(writer, uuids, projectRoot, cfg.MaxAttachmentBytes, cfg.FollowSymlinks, cfg.AllowExternalAttachments, cfg.Redact).WithSearchRoots(env["gauge_screenshots_dir"])
	mapper := mapping.New(cfg, projectRoot, env, uuids, manager, nil)
	engine := collector.New(cfg, projectRoot, env, mapper, writer, logger)
	if err := server.Serve(ctx, engine, logger); err != nil && ctx.Err() == nil {
		logger.Error("reporter server stopped: %v", err)
		return 1
	}
	return 0
}

func validateConfig(args []string, stdout, stderr io.Writer) int {
	projectRoot, err := detectProjectRoot()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	env := config.Environment()
	if len(args) > 1 {
		fmt.Fprintln(stderr, "validate-config accepts at most one path")
		return 2
	}
	if len(args) == 1 {
		env["GAUGE_ALLURE_CONFIG"] = args[0]
	}
	cfg, err := config.Load(projectRoot, env)
	if err != nil {
		fmt.Fprintf(stderr, "invalid configuration: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "configuration valid: %s\nresults directory: %s\n", first(cfg.SourcePath, "built-in defaults + environment"), cfg.ResultsDir)
	return 0
}

func doctor(stdout, stderr io.Writer) int {
	projectRoot, err := detectProjectRoot()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	env := config.Environment()
	cfg, err := config.Load(projectRoot, env)
	if err != nil {
		fmt.Fprintf(stderr, "configuration: invalid: %v\n", err)
		return 1
	}
	writable, warning := probeWritable(cfg.ResultsDir)
	executor := "none"
	if value, ok := metadata.Executor(env, cfg.Executor); ok {
		executor = first(value.Name, value.Type)
	}
	diagnostic := map[string]any{
		"version": version.String(), "projectRoot": projectRoot, "resultsDirectory": cfg.ResultsDir,
		"configuration": first(cfg.SourcePath, "built-in defaults + environment"), "configurationValid": true,
		"gaugeActionPresent": os.Getenv(actionEnvironment) != "", "outputWritable": writable,
		"ciProvider": executor, "compatibility": cfg.Compatibility,
		"warnings": compactWarnings(warning, cleanWarning(cfg, projectRoot), protoWarning(cfg)),
	}
	data, _ := json.MarshalIndent(diagnostic, "", "  ")
	fmt.Fprintln(stdout, string(data))
	if !writable {
		return 1
	}
	return 0
}

func detectProjectRoot() (string, error) {
	root := os.Getenv("GAUGE_PROJECT_ROOT")
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("detect project root: %w", err)
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	return filepath.Clean(root), nil
}

func probeWritable(path string) (bool, string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return false, err.Error()
	}
	file, err := os.CreateTemp(path, ".doctor-")
	if err != nil {
		return false, err.Error()
	}
	name := file.Name()
	_ = file.Close()
	_ = os.Remove(name)
	return true, ""
}

func cleanWarning(cfg config.Config, projectRoot string) string {
	if cfg.AllowCleanNonOwned {
		return "allow_clean_non_owned weakens the default ownership guard"
	}
	if strings.EqualFold(filepath.Clean(cfg.ResultsDir), filepath.Clean(projectRoot)) {
		return "results directory resolves to project root and will be refused"
	}
	return ""
}
func protoWarning(cfg config.Config) string {
	if cfg.DiagnosticProtoDump {
		return "diagnostic_proto_dump may contain sensitive test data"
	}
	return ""
}
func compactWarnings(values ...string) []string {
	result := []string{}
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
func usage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: allure-report [version|validate-config [path]|doctor]\nGauge launches the command without arguments using allure-report_action=execution.")
}
func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
