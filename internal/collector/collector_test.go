// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/allure-framework/allure-go/commons/model"
	gm "github.com/getgauge/gauge-proto/go/gauge_messages"
	"github.com/jsabak/gauge-allure-report/internal/config"
	"github.com/jsabak/gauge-allure-report/internal/mapping"
)

type atomicUUID struct{ value atomic.Uint64 }

func (u *atomicUUID) New() (string, error) {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", u.value.Add(1)), nil
}

type collectorClock struct{ value time.Time }

func (c collectorClock) Now() time.Time { return c.value }

type collectorWriter struct {
	mu           sync.Mutex
	results      map[string]model.TestResult
	containers   map[string]model.TestResultContainer
	environments int
	executors    int
	categories   int
	attachments  int
}

func (w *collectorWriter) WriteResult(_ context.Context, value model.TestResult) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.results == nil {
		w.results = map[string]model.TestResult{}
	}
	w.results[value.UUID] = value
	return nil
}
func (w *collectorWriter) WriteContainer(_ context.Context, value model.TestResultContainer) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.containers == nil {
		w.containers = map[string]model.TestResultContainer{}
	}
	w.containers[value.UUID] = value
	return nil
}

func (w *collectorWriter) WriteAttachment(_ context.Context, _ string, reader io.Reader, _ int64) error {
	_, _ = io.Copy(io.Discard, reader)
	w.mu.Lock()
	w.attachments++
	w.mu.Unlock()
	return nil
}
func (w *collectorWriter) WriteEnvironment(context.Context, map[string]string) error {
	w.mu.Lock()
	w.environments++
	w.mu.Unlock()
	return nil
}
func (w *collectorWriter) WriteExecutor(context.Context, model.Executor) error {
	w.mu.Lock()
	w.executors++
	w.mu.Unlock()
	return nil
}
func (w *collectorWriter) WriteCategories(context.Context, []model.Category) error {
	w.mu.Lock()
	w.categories++
	w.mu.Unlock()
	return nil
}

type failingCollectorWriter struct {
	*collectorWriter
	fail string
}

func (w *failingCollectorWriter) failure(scope string) error {
	if w.fail == scope || w.fail == "all" {
		return errors.New(scope + " failed")
	}
	return nil
}
func (w *failingCollectorWriter) WriteResult(ctx context.Context, value model.TestResult) error {
	if err := w.failure("result"); err != nil {
		return err
	}
	return w.collectorWriter.WriteResult(ctx, value)
}
func (w *failingCollectorWriter) WriteContainer(ctx context.Context, value model.TestResultContainer) error {
	if err := w.failure("container"); err != nil {
		return err
	}
	return w.collectorWriter.WriteContainer(ctx, value)
}
func (w *failingCollectorWriter) WriteAttachment(ctx context.Context, name string, reader io.Reader, size int64) error {
	if err := w.failure("attachment"); err != nil {
		return err
	}
	return w.collectorWriter.WriteAttachment(ctx, name, reader, size)
}
func (w *failingCollectorWriter) WriteEnvironment(ctx context.Context, value map[string]string) error {
	if err := w.failure("environment"); err != nil {
		return err
	}
	return w.collectorWriter.WriteEnvironment(ctx, value)
}
func (w *failingCollectorWriter) WriteExecutor(ctx context.Context, value model.Executor) error {
	if err := w.failure("executor"); err != nil {
		return err
	}
	return w.collectorWriter.WriteExecutor(ctx, value)
}
func (w *failingCollectorWriter) WriteCategories(ctx context.Context, value []model.Category) error {
	if err := w.failure("categories"); err != nil {
		return err
	}
	return w.collectorWriter.WriteCategories(ctx, value)
}

type discardLogger struct{}

func (discardLogger) Debug(string, ...any) {}
func (discardLogger) Info(string, ...any)  {}
func (discardLogger) Warn(string, ...any)  {}
func (discardLogger) Error(string, ...any) {}

func TestConcurrentStreamingAndFinalReconciliation(t *testing.T) {
	cfg := config.Defaults()
	cfg.ResultsDir = t.TempDir()
	cfg.ExecutorAuto = false
	env := map[string]string{"GAUGE_PROJECT_NAME": "demo", "test_language": "python"}
	uuids := &atomicUUID{}
	writer := &collectorWriter{}
	mapper := mapping.New(cfg, `C:\project`, env, uuids, nil, collectorClock{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
	engine := New(cfg, `C:\project`, env, mapper, writer, discardLogger{})
	const scenarios = 8
	items := make([]*gm.ProtoItem, scenarios)
	var wait sync.WaitGroup
	for index := 0; index < scenarios; index++ {
		scenario := &gm.ProtoScenario{ID: fmt.Sprintf("s-%d", index), ScenarioHeading: fmt.Sprintf("scenario %d", index), ExecutionStatus: gm.ExecutionStatus_PASSED, ExecutionTime: 1, RetriesCount: 1,
			ScenarioItems: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Step, Step: &gm.ProtoStep{ActualText: "pass", StepExecutionResult: &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{ExecutionTime: 1}}}}}}
		items[index] = &gm.ProtoItem{ItemType: gm.ProtoItem_Scenario, Scenario: scenario, FileName: `C:\project\specs\parallel.spec`}
		request := liveRequest(index, items[index])
		engine.ScenarioStarting(&gm.ScenarioExecutionStartingRequest{CurrentExecutionInfo: request.CurrentExecutionInfo, ScenarioResult: request.ScenarioResult, Stream: request.Stream})
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := engine.ScenarioEnding(context.Background(), request); err != nil {
				t.Errorf("ScenarioEnding: %v", err)
			}
		}()
	}
	wait.Wait()
	suite := &gm.ProtoSuiteResult{ProjectName: "demo", TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: scenarios, SpecResults: []*gm.ProtoSpecResult{{TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: scenarios, ProtoSpec: &gm.ProtoSpec{SpecHeading: "Parallel", FileName: `C:\project\specs\parallel.spec`, Items: items}}}}
	if err := engine.Finalize(context.Background(), &gm.SuiteExecutionResult{SuiteResult: suite}); err != nil {
		t.Fatal(err)
	}
	summary := engine.Summary()
	if summary.Passed != scenarios || summary.Containers != scenarios+2 || summary.Warnings != 0 {
		t.Fatalf("summary: %+v", summary)
	}
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.results) != scenarios || len(writer.containers) != scenarios+2 || writer.environments != 1 {
		t.Fatalf("writer results=%d containers=%d env=%d", len(writer.results), len(writer.containers), writer.environments)
	}
}

func TestInterruptedRunCreatesBrokenResult(t *testing.T) {
	cfg := config.Defaults()
	cfg.ResultsDir = t.TempDir()
	cfg.ExecutorAuto = false
	writer := &collectorWriter{}
	uuids := &atomicUUID{}
	mapper := mapping.New(cfg, t.TempDir(), nil, uuids, nil, collectorClock{time.Unix(0, 0)})
	engine := New(cfg, t.TempDir(), nil, mapper, writer, discardLogger{})
	if err := engine.FinalizeInterrupted(context.Background()); err != nil {
		t.Fatal(err)
	}
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.results) != 1 {
		t.Fatalf("results=%d", len(writer.results))
	}
	for _, result := range writer.results {
		if result.Status != model.StatusBroken {
			t.Fatalf("status=%s", result.Status)
		}
	}
}

func TestLifecycleBookkeepingAndWarnings(t *testing.T) {
	cfg := config.Defaults()
	cfg.ResultsDir = t.TempDir()
	cfg.ExecutorAuto = false
	writer := &collectorWriter{}
	mapper := mapping.New(cfg, t.TempDir(), nil, &atomicUUID{}, nil, collectorClock{time.Unix(0, 0)})
	engine := New(cfg, t.TempDir(), nil, mapper, writer, discardLogger{})
	engine.ExecutionStarting(nil)
	engine.ExecutionEnding(nil)
	engine.SpecStarting(nil)
	engine.SpecEnding(nil)
	engine.ScenarioStarting(nil)
	info := &gm.ExecutionInfo{ProjectName: "project", RunnerId: 2, NumberOfExecutionStreams: 3, CurrentSpec: &gm.SpecInfo{FileName: "specs/a.spec"}, CurrentScenario: &gm.ScenarioInfo{Name: "scenario", Retries: &gm.ScenarioRetriesInfo{CurrentRetry: 1}}, CurrentStep: &gm.StepInfo{Step: &gm.ExecuteStepRequest{ParsedStepText: "step"}}}
	engine.ExecutionStarting(&gm.ExecutionStartingRequest{CurrentExecutionInfo: info, SuiteResult: &gm.ProtoSuiteResult{Environment: "ci"}, Stream: 1})
	engine.SpecStarting(&gm.SpecExecutionStartingRequest{CurrentExecutionInfo: info, Stream: 1})
	engine.ScenarioStarting(&gm.ScenarioExecutionStartingRequest{CurrentExecutionInfo: info, Stream: 1})
	engine.StepStarting(&gm.StepExecutionStartingRequest{CurrentExecutionInfo: info, Stream: 1})
	engine.StepEnding(&gm.StepExecutionEndingRequest{CurrentExecutionInfo: info, Stream: 1})
	engine.StepEnding(nil)
	engine.ConceptStarting(&gm.ConceptExecutionStartingRequest{CurrentExecutionInfo: info, Stream: 1})
	engine.ConceptEnding(&gm.ConceptExecutionEndingRequest{CurrentExecutionInfo: info, Stream: 1})
	engine.ConceptEnding(nil)
	engine.SpecEnding(&gm.SpecExecutionEndingRequest{CurrentExecutionInfo: info, Stream: 1})
	engine.ExecutionEnding(&gm.ExecutionEndingRequest{CurrentExecutionInfo: info, Stream: 1})
	if engine.project != "project" || engine.gaugeEnv != "ci" || engine.streams != 3 {
		t.Fatalf("execution metadata was not retained: project=%q env=%q streams=%d", engine.project, engine.gaugeEnv, engine.streams)
	}
	if len(engine.warnings) != 5 {
		t.Fatalf("nil lifecycle warnings = %v", engine.warnings)
	}
	if err := engine.ScenarioEnding(context.Background(), nil); err != nil {
		t.Fatalf("best-effort nil scenario should warn: %v", err)
	}
	if err := engine.handleError(nil); err != nil {
		t.Fatal(err)
	}
	if got := correlation("step", 1, info); !strings.Contains(got, "specs/a.spec:scenario:step:1") {
		t.Fatalf("correlation = %q", got)
	}
	if got := correlation("scope", 4, nil); got != "scope:4:nil" {
		t.Fatalf("nil correlation = %q", got)
	}
}

func TestStrictErrorsAndInterruptedExistingResults(t *testing.T) {
	cfg := config.Defaults()
	cfg.ResultsDir = t.TempDir()
	cfg.Strict = true
	cfg.ExecutorAuto = false
	mapper := mapping.New(cfg, t.TempDir(), nil, &atomicUUID{}, nil, collectorClock{time.Unix(0, 0)})
	engine := New(cfg, t.TempDir(), nil, mapper, &collectorWriter{}, discardLogger{})
	if err := engine.ScenarioEnding(context.Background(), nil); err == nil {
		t.Fatal("strict nil scenario must fail")
	}
	engine.results["partial"] = model.TestResult{UUID: "partial", Status: model.StatusSkipped, Parameters: []model.Parameter{{Name: "retry"}}, Attachments: []model.Attachment{{Source: "one"}}, Steps: []model.StepResult{{Attachments: []model.Attachment{{Source: "two"}}, Steps: []model.StepResult{{Attachments: []model.Attachment{{Source: "three"}}}}}}}
	if err := engine.FinalizeInterrupted(context.Background()); err != nil {
		t.Fatal(err)
	}
	if summary := engine.Summary(); summary.Skipped != 1 || summary.Retries != 1 || summary.Attachments != 3 {
		t.Fatalf("partial summary: %+v", summary)
	}
	if err := engine.FinalizeInterrupted(context.Background()); err != nil {
		t.Fatal("second interrupted finalize should be idempotent")
	}
}

func TestFinalizeDiagnosticsExecutorCategoriesAndIdempotence(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.ResultsDir = filepath.Join(root, "results")
	cfg.DiagnosticProtoDump = true
	cfg.CategoriesProfile = "gauge-default"
	env := map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_RUN_ID": "42", "GITHUB_REPOSITORY": "owner/repo", "GITHUB_SERVER_URL": "https://github.com"}
	writer := &collectorWriter{}
	mapper := mapping.New(cfg, root, env, &atomicUUID{}, nil, collectorClock{time.Unix(0, 0)})
	engine := New(cfg, root, env, mapper, writer, discardLogger{})
	scenario := &gm.ProtoScenario{ID: "one", ScenarioHeading: "one", ExecutionStatus: gm.ExecutionStatus_PASSED, ExecutionTime: 1, ScenarioItems: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Step, Step: &gm.ProtoStep{ActualText: "pass", StepExecutionResult: &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{ExecutionTime: 1}}}}}}
	suite := &gm.ProtoSuiteResult{ProjectName: "demo", Environment: "ci", TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 1, SpecResults: []*gm.ProtoSpecResult{{TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 1, ProtoSpec: &gm.ProtoSpec{FileName: "specs/a.spec", Items: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Scenario, Scenario: scenario}}}}}}
	request := &gm.SuiteExecutionResult{SuiteResult: suite}
	if err := engine.Finalize(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if err := engine.Finalize(context.Background(), request); err != nil {
		t.Fatal("second finalize should be idempotent")
	}
	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.attachments != 1 || writer.executors != 1 || writer.categories != 1 || writer.environments != 1 {
		t.Fatalf("metadata writes: attachments=%d executors=%d categories=%d env=%d", writer.attachments, writer.executors, writer.categories, writer.environments)
	}
	if !strings.HasSuffix(diagnosticSource([]byte("stable")), "-attachment.json") {
		t.Fatal("diagnostic source is not an Allure attachment")
	}
}

func TestCategoriesFileValidation(t *testing.T) {
	root := t.TempDir()
	cfg := config.Defaults()
	cfg.ResultsDir = filepath.Join(root, "results")
	cfg.CategoriesFile = "categories.json"
	engine := New(cfg, root, nil, nil, &collectorWriter{}, discardLogger{})
	path := filepath.Join(root, "categories.json")
	if _, _, err := engine.categories(); err == nil {
		t.Fatal("expected missing categories error")
	}
	if err := os.WriteFile(path, []byte(`[{"name":"Infrastructure","matchedStatuses":["broken"]}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	values, ok, err := engine.categories()
	if err != nil || !ok || len(values) != 1 {
		t.Fatalf("valid categories: values=%v ok=%v err=%v", values, ok, err)
	}
	if err := os.WriteFile(path, []byte(`[{"unknown":true}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.categories(); err == nil {
		t.Fatal("expected strict categories schema error")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := engine.categories(); err == nil {
		t.Fatal("expected non-regular categories error")
	}
}

func TestFinalizeWriterFailuresRespectMode(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(fmt.Sprintf("strict-%v", strict), func(t *testing.T) {
			cfg := config.Defaults()
			cfg.ResultsDir = t.TempDir()
			cfg.Strict = strict
			cfg.ExecutorAuto = false
			writer := &failingCollectorWriter{collectorWriter: &collectorWriter{}, fail: "result"}
			mapper := mapping.New(cfg, t.TempDir(), nil, &atomicUUID{}, nil, collectorClock{time.Unix(0, 0)})
			engine := New(cfg, t.TempDir(), nil, mapper, writer, discardLogger{})
			err := engine.Finalize(context.Background(), nil)
			if strict && err == nil {
				t.Fatal("strict mode must return writer failure")
			}
			if !strict && (err != nil || len(engine.warnings) == 0) {
				t.Fatalf("best effort should retain warning: err=%v warnings=%v", err, engine.warnings)
			}
		})
	}
}

func liveRequest(index int, item *gm.ProtoItem) *gm.ScenarioExecutionEndingRequest {
	return &gm.ScenarioExecutionEndingRequest{
		Stream:               int32(index % 2),
		CurrentExecutionInfo: &gm.ExecutionInfo{ProjectName: "demo", RunnerId: int32(index % 2), CurrentSpec: &gm.SpecInfo{Name: "Parallel", FileName: `C:\project\specs\parallel.spec`}, CurrentScenario: &gm.ScenarioInfo{Name: item.GetScenario().GetScenarioHeading(), Retries: &gm.ScenarioRetriesInfo{CurrentRetry: 0}}},
		ScenarioResult:       &gm.ProtoScenarioResult{ProtoItem: item, TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 1},
	}
}
