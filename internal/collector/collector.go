// SPDX-License-Identifier: Apache-2.0

// Package collector reconciles concurrent Gauge lifecycle callbacks with the final suite result.
package collector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/allure-framework/allure-go/commons/model"
	gm "github.com/getgauge/gauge-proto/go/gauge_messages"
	"github.com/jsabak/gauge-allure-report/internal/config"
	"github.com/jsabak/gauge-allure-report/internal/mapping"
	"github.com/jsabak/gauge-allure-report/internal/metadata"
	"github.com/jsabak/gauge-allure-report/internal/output"
	"github.com/jsabak/gauge-allure-report/internal/timing"
	"google.golang.org/protobuf/encoding/protojson"
)

// Logger keeps protocol stdout free for Gauge's startup handshake.
type Logger interface {
	Debug(string, ...any)
	Info(string, ...any)
	Warn(string, ...any)
	Error(string, ...any)
}

// Summary is emitted at finalization and exposed to tests.
type Summary struct {
	OutputPath  string
	Passed      int
	Failed      int
	Broken      int
	Skipped     int
	Retries     int
	Containers  int
	Attachments int
	Warnings    int
	Elapsed     time.Duration
}

// Engine owns all mutable run state behind a mutex.
type Engine struct {
	mu          sync.Mutex
	cfg         config.Config
	projectRoot string
	env         map[string]string
	mapper      *mapping.Mapper
	writer      output.Writer
	logger      Logger
	started     time.Time
	startEvents map[string]time.Time
	live        map[string]mapping.LiveInfo
	uuids       map[string]string
	results     map[string]model.TestResult
	warnings    []string
	finalized   bool
	project     string
	gaugeEnv    string
	streams     int32
	summary     Summary
}

// New creates a collector with injected mapper and writer boundaries.
func New(cfg config.Config, projectRoot string, env map[string]string, mapper *mapping.Mapper, writer output.Writer, logger Logger) *Engine {
	return &Engine{cfg: cfg, projectRoot: projectRoot, env: env, mapper: mapper, writer: writer, logger: logger,
		started: time.Now(), startEvents: make(map[string]time.Time), live: make(map[string]mapping.LiveInfo), uuids: make(map[string]string), results: make(map[string]model.TestResult)}
}

func (e *Engine) ExecutionStarting(request *gm.ExecutionStartingRequest) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if request == nil {
		e.warnLocked("received nil execution-start event")
		return
	}
	info := request.GetCurrentExecutionInfo()
	e.project = first(info.GetProjectName(), request.GetSuiteResult().GetProjectName(), e.project)
	e.gaugeEnv = first(request.GetSuiteResult().GetEnvironment(), e.gaugeEnv)
	e.streams = max32(e.streams, info.GetNumberOfExecutionStreams())
	e.startEvents[correlation("suite", request.GetStream(), info)] = time.Now()
}

func (e *Engine) ExecutionEnding(request *gm.ExecutionEndingRequest) {
	if request == nil {
		e.addWarning("received nil execution-end event")
		return
	}
	e.mu.Lock()
	delete(e.startEvents, correlation("suite", request.GetStream(), request.GetCurrentExecutionInfo()))
	e.mu.Unlock()
}

func (e *Engine) SpecStarting(request *gm.SpecExecutionStartingRequest) {
	if request == nil {
		e.addWarning("received nil specification-start event")
		return
	}
	e.mu.Lock()
	e.startEvents[correlation("spec", request.GetStream(), request.GetCurrentExecutionInfo())] = time.Now()
	e.mu.Unlock()
}

func (e *Engine) SpecEnding(request *gm.SpecExecutionEndingRequest) {
	if request == nil {
		e.addWarning("received nil specification-end event")
		return
	}
	e.mu.Lock()
	delete(e.startEvents, correlation("spec", request.GetStream(), request.GetCurrentExecutionInfo()))
	e.mu.Unlock()
}

func (e *Engine) ScenarioStarting(request *gm.ScenarioExecutionStartingRequest) {
	if request == nil {
		e.addWarning("received nil scenario-start event")
		return
	}
	e.mu.Lock()
	e.startEvents[correlation("scenario", request.GetStream(), request.GetCurrentExecutionInfo())] = time.Now()
	e.mu.Unlock()
}

// ScenarioEnding writes a streaming result and records stable reconciliation data.
func (e *Engine) ScenarioEnding(ctx context.Context, request *gm.ScenarioExecutionEndingRequest) error {
	if request == nil {
		return e.handleError(errors.New("received nil scenario-end event"))
	}
	artifact, warnings, err := e.mapper.MapLiveScenario(ctx, request, "")
	if err != nil {
		return e.handleError(err)
	}
	e.mu.Lock()
	if uuid := e.uuids[artifact.Key]; uuid != "" && uuid != artifact.Result.UUID {
		e.mu.Unlock()
		artifact, warnings, err = e.mapper.MapLiveScenario(ctx, request, uuid)
		if err != nil {
			return e.handleError(err)
		}
		e.mu.Lock()
	}
	startKey := correlation("scenario", request.GetStream(), request.GetCurrentExecutionInfo())
	if start, ok := e.startEvents[startKey]; ok {
		artifact.Result.Start = start.UnixMilli()
		if artifact.Result.Stop < artifact.Result.Start {
			artifact.Result.Stop = time.Now().UnixMilli()
		}
		delete(e.startEvents, startKey)
	}
	e.uuids[artifact.Key] = artifact.Result.UUID
	e.live[artifact.Key] = mapping.LiveInfo{Range: timing.Range{Start: artifact.Result.Start, Stop: artifact.Result.Stop}, Stream: request.GetStream(), Runner: request.GetCurrentExecutionInfo().GetRunnerId(), Retry: request.GetCurrentExecutionInfo().GetCurrentScenario().GetRetries().GetCurrentRetry()}
	e.warnings = append(e.warnings, warnings...)
	e.mu.Unlock()
	if e.cfg.WriteMode == "streaming" {
		if err := e.writer.WriteResult(ctx, artifact.Result); err != nil {
			return e.handleError(fmt.Errorf("stream scenario result: %w", err))
		}
	}
	e.mu.Lock()
	e.results[artifact.Result.UUID] = artifact.Result
	e.mu.Unlock()
	return nil
}

func (e *Engine) StepStarting(request *gm.StepExecutionStartingRequest) {
	e.recordStart("step", request.GetStream(), request.GetCurrentExecutionInfo())
}
func (e *Engine) StepEnding(request *gm.StepExecutionEndingRequest) {
	if request != nil {
		e.recordEnd("step", request.GetStream(), request.GetCurrentExecutionInfo())
	}
}
func (e *Engine) ConceptStarting(request *gm.ConceptExecutionStartingRequest) {
	e.recordStart("concept", request.GetStream(), request.GetCurrentExecutionInfo())
}
func (e *Engine) ConceptEnding(request *gm.ConceptExecutionEndingRequest) {
	if request != nil {
		e.recordEnd("concept", request.GetStream(), request.GetCurrentExecutionInfo())
	}
}

// Finalize reconciles the authoritative suite, rewrites stable attempt files,
// and emits containers plus run metadata.
func (e *Engine) Finalize(ctx context.Context, request *gm.SuiteExecutionResult) error {
	e.mu.Lock()
	if e.finalized {
		e.mu.Unlock()
		return nil
	}
	e.finalized = true
	live := copyLive(e.live)
	existing := copyStrings(e.uuids)
	e.mu.Unlock()
	var suite *gm.ProtoSuiteResult
	if request != nil {
		suite = request.GetSuiteResult()
	}
	outputValue, err := e.mapper.MapSuite(ctx, suite, live, existing)
	if err != nil {
		return e.handleError(err)
	}
	writeErrors := make([]error, 0)
	if e.cfg.DiagnosticProtoDump && suite != nil && len(outputValue.Results) > 0 {
		data, marshalErr := protojson.MarshalOptions{Multiline: true, Indent: "  ", UseProtoNames: true}.Marshal(suite)
		if marshalErr != nil {
			writeErrors = append(writeErrors, fmt.Errorf("marshal diagnostic proto dump: %w", marshalErr))
		} else {
			data = append(data, '\n')
			source := diagnosticSource(data)
			if writeErr := e.writer.WriteAttachment(ctx, source, bytes.NewReader(data), int64(len(data))); writeErr != nil {
				writeErrors = append(writeErrors, fmt.Errorf("write diagnostic proto dump: %w", writeErr))
			} else {
				outputValue.Results[0].Result.Attachments = append(outputValue.Results[0].Result.Attachments, model.Attachment{Name: "Gauge suite protocol diagnostic (sensitive)", Type: "application/json", Source: source})
			}
		}
	}
	for _, artifact := range outputValue.Results {
		if err := e.writer.WriteResult(ctx, artifact.Result); err != nil {
			writeErrors = append(writeErrors, fmt.Errorf("write result %s: %w", artifact.Key, err))
			continue
		}
		e.mu.Lock()
		e.uuids[artifact.Key] = artifact.Result.UUID
		e.results[artifact.Result.UUID] = artifact.Result
		e.mu.Unlock()
	}
	for _, container := range outputValue.Containers {
		if err := e.writer.WriteContainer(ctx, container); err != nil {
			writeErrors = append(writeErrors, fmt.Errorf("write container: %w", err))
		}
	}
	e.mu.Lock()
	e.warnings = append(e.warnings, outputValue.Warnings...)
	e.mu.Unlock()
	if err := e.writeMetadata(ctx, suite); err != nil {
		writeErrors = append(writeErrors, err)
	}
	if len(writeErrors) > 0 {
		combined := errors.Join(writeErrors...)
		if e.cfg.Strict {
			return combined
		}
		e.addWarning(combined.Error())
	}
	fixtureAttachments := 0
	for _, container := range outputValue.Containers {
		for _, fixture := range container.Befores {
			fixtureAttachments += len(fixture.Attachments) + countAttachments(fixture.Steps)
		}
		for _, fixture := range container.Afters {
			fixtureAttachments += len(fixture.Attachments) + countAttachments(fixture.Steps)
		}
	}
	e.finishSummary(len(outputValue.Containers), fixtureAttachments)
	return nil
}

// FinalizeInterrupted preserves partial streaming results or emits a visible broken result.
func (e *Engine) FinalizeInterrupted(ctx context.Context) error {
	e.mu.Lock()
	already := e.finalized
	count := len(e.results)
	e.mu.Unlock()
	if already {
		return nil
	}
	if count > 0 {
		e.mu.Lock()
		e.finalized = true
		e.mu.Unlock()
		e.finishSummary(0, 0)
		return nil
	}
	return e.Finalize(ctx, nil)
}

// Summary returns a snapshot after or during execution.
func (e *Engine) Summary() Summary { e.mu.Lock(); defer e.mu.Unlock(); return e.summary }

func (e *Engine) writeMetadata(ctx context.Context, suite *gm.ProtoSuiteResult) error {
	project, gaugeEnvironment, streams := e.project, e.gaugeEnv, e.streams
	if suite != nil {
		project = first(suite.GetProjectName(), project)
		gaugeEnvironment = first(suite.GetEnvironment(), gaugeEnvironment)
	}
	if err := e.writer.WriteEnvironment(ctx, metadata.Environment(e.env, e.cfg, project, gaugeEnvironment, streams)); err != nil {
		return fmt.Errorf("write environment metadata: %w", err)
	}
	if e.cfg.ExecutorAuto || e.cfg.Executor.Name != "" || e.cfg.Executor.Type != "" {
		if executor, ok := metadata.Executor(e.env, e.cfg.Executor); ok {
			if err := e.writer.WriteExecutor(ctx, executor); err != nil {
				return fmt.Errorf("write executor metadata: %w", err)
			}
		}
	}
	categories, ok, err := e.categories()
	if err != nil {
		return err
	}
	if ok {
		if err := e.writer.WriteCategories(ctx, categories); err != nil {
			return fmt.Errorf("write categories: %w", err)
		}
	}
	return nil
}

func (e *Engine) categories() ([]model.Category, bool, error) {
	if e.cfg.CategoriesFile != "" {
		path := e.cfg.CategoriesFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(e.projectRoot, path)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, false, fmt.Errorf("inspect categories file: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
			return nil, false, errors.New("categories file must be a regular non-symlink file no larger than 1 MiB")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, false, fmt.Errorf("read categories: %w", err)
		}
		var categories []model.Category
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&categories); err != nil {
			return nil, false, fmt.Errorf("decode categories: %w", err)
		}
		return categories, true, nil
	}
	if e.cfg.CategoriesProfile == "gauge-default" {
		return []model.Category{{Name: "Gauge infrastructure and integration errors", MatchedStatuses: []model.Status{model.StatusBroken}}, {Name: "Gauge assertion failures", MatchedStatuses: []model.Status{model.StatusFailed}}}, true, nil
	}
	return nil, false, nil
}

func (e *Engine) finishSummary(containers, fixtureAttachments int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	summary := Summary{OutputPath: e.cfg.ResultsDir, Containers: containers, Warnings: len(e.warnings), Elapsed: time.Since(e.started)}
	summary.Attachments = fixtureAttachments
	for _, result := range e.results {
		switch result.Status {
		case model.StatusPassed:
			summary.Passed++
		case model.StatusFailed:
			summary.Failed++
		case model.StatusBroken:
			summary.Broken++
		case model.StatusSkipped:
			summary.Skipped++
		}
		for _, parameter := range result.Parameters {
			if parameter.Name == "retry" {
				summary.Retries++
			}
		}
		summary.Attachments += countAttachments(result.Steps) + len(result.Attachments)
	}
	e.summary = summary
	if e.logger != nil {
		e.logger.Info("Allure results: path=%s passed=%d failed=%d broken=%d skipped=%d retries=%d containers=%d attachments=%d warnings=%d elapsed=%s", summary.OutputPath, summary.Passed, summary.Failed, summary.Broken, summary.Skipped, summary.Retries, summary.Containers, summary.Attachments, summary.Warnings, summary.Elapsed.Round(time.Millisecond))
	}
}

func (e *Engine) recordStart(scope string, stream int32, info *gm.ExecutionInfo) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.startEvents[correlation(scope, stream, info)] = time.Now()
}
func (e *Engine) recordEnd(scope string, stream int32, info *gm.ExecutionInfo) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.startEvents, correlation(scope, stream, info))
}
func (e *Engine) addWarning(value string) { e.mu.Lock(); defer e.mu.Unlock(); e.warnLocked(value) }
func (e *Engine) warnLocked(value string) {
	e.warnings = append(e.warnings, value)
	if e.logger != nil {
		e.logger.Warn("%s", value)
	}
}
func (e *Engine) handleError(err error) error {
	if err == nil {
		return nil
	}
	if e.cfg.Strict {
		return err
	}
	e.addWarning(err.Error())
	return nil
}

func correlation(scope string, stream int32, info *gm.ExecutionInfo) string {
	if info == nil {
		return fmt.Sprintf("%s:%d:nil", scope, stream)
	}
	spec, scenario, step := info.GetCurrentSpec(), info.GetCurrentScenario(), info.GetCurrentStep()
	retry := int32(0)
	if scenario.GetRetries() != nil {
		retry = scenario.GetRetries().GetCurrentRetry()
	}
	return fmt.Sprintf("%s:%d:%d:%s:%s:%s:%d", scope, stream, info.GetRunnerId(), spec.GetFileName(), scenario.GetName(), step.GetStep().GetParsedStepText(), retry)
}

func copyLive(source map[string]mapping.LiveInfo) map[string]mapping.LiveInfo {
	result := make(map[string]mapping.LiveInfo, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
func copyStrings(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
func countAttachments(steps []model.StepResult) int {
	count := 0
	for _, step := range steps {
		count += len(step.Attachments) + countAttachments(step.Steps)
	}
	return count
}
func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func diagnosticSource(data []byte) string {
	sum := sha256.Sum256(data)
	value := sum[:16]
	value[6] = (value[6] & 0x0f) | 0x50
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x-attachment.json", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16])
}
