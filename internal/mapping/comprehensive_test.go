// SPDX-License-Identifier: Apache-2.0

package mapping

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/allure-framework/allure-go/commons/model"
	gm "github.com/getgauge/gauge-proto/go/gauge_messages"
	"github.com/jsabak/gauge-allure-report/internal/attachments"
	"github.com/jsabak/gauge-allure-report/internal/config"
	"github.com/jsabak/gauge-allure-report/internal/timing"
)

type countingUUID struct{ value int }

func (u *countingUUID) New() (string, error) {
	u.value++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", u.value), nil
}

type attachmentWriter struct{ values map[string][]byte }

func (w *attachmentWriter) WriteResult(context.Context, model.TestResult) error { return nil }
func (w *attachmentWriter) WriteContainer(context.Context, model.TestResultContainer) error {
	return nil
}
func (w *attachmentWriter) WriteAttachment(_ context.Context, name string, reader io.Reader, _ int64) error {
	if w.values == nil {
		w.values = map[string][]byte{}
	}
	data, err := io.ReadAll(reader)
	if err == nil {
		w.values[name] = data
	}
	return err
}
func (w *attachmentWriter) WriteEnvironment(context.Context, map[string]string) error { return nil }
func (w *attachmentWriter) WriteExecutor(context.Context, model.Executor) error       { return nil }
func (w *attachmentWriter) WriteCategories(context.Context, []model.Category) error   { return nil }

func parameterFragment(name, value string, kind gm.Parameter_ParameterType, table *gm.ProtoTable) *gm.Fragment {
	return &gm.Fragment{FragmentType: gm.Fragment_Parameter, Parameter: &gm.Parameter{Name: name, Value: value, ParameterType: kind, Table: table}}
}

func TestComprehensiveMappingWithAttachmentsAndFixtures(t *testing.T) {
	root := t.TempDir()
	image := filepath.Join(root, "failure.png")
	if err := os.WriteFile(image, []byte("\x89PNG\r\n\x1a\nimage"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.ContextMode = "fixtures"
	cfg.TeardownMode = "fixtures"
	cfg.BehaviorFromSpec = true
	cfg.PromoteParameters = []string{"customer*"}
	cfg.SourceLinkTemplate = "https://github.example/repository/blob/main/{path}#L{line}"
	cfg.Redact = []string{"secret"}
	ids := &countingUUID{}
	writer := &attachmentWriter{}
	manager := attachments.New(writer, ids, root, 1024*1024, false, false, cfg.Redact)
	mapper := New(cfg, root, map[string]string{"GAUGE_PROJECT_NAME": "demo", "test_language": "python"}, ids, manager, fixedClock{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})

	table := &gm.ProtoTable{Headers: &gm.ProtoTableRow{Cells: []string{"a"}}, Rows: []*gm.ProtoTableRow{{Cells: []string{"b"}}}}
	execution := &gm.ProtoExecutionResult{ExecutionTime: 3, Message: []string{"step secret message"}, FailureScreenshotFile: image, ScreenshotFiles: []string{image}, Screenshots: [][]byte{[]byte("legacy-unused")}}
	step := &gm.ProtoStep{
		ActualText: "complex step", ParsedText: "complex <customer>",
		Fragments: []*gm.Fragment{
			parameterFragment("customer", "alice", gm.Parameter_Static, nil),
			parameterFragment("", "document", gm.Parameter_Multiline_String, nil),
			parameterFragment("payload", "", gm.Parameter_Table, table),
		},
		StepExecutionResult:    &gm.ProtoStepExecutionResult{ExecutionResult: execution},
		PreHookMessages:        []string{"before secret"},
		PostHookMessages:       []string{"after"},
		PreHookScreenshotFiles: []string{image},
		PostHookScreenshots:    [][]byte{[]byte("legacy-post")},
	}
	contextStep := &gm.ProtoItem{ItemType: gm.ProtoItem_Step, Step: passedStep("context", 1)}
	teardownStep := &gm.ProtoItem{ItemType: gm.ProtoItem_Step, Step: passedStep("teardown", 1)}
	scenario := &gm.ProtoScenario{
		ID: "scenario-id", ScenarioHeading: "Comprehensive", Span: &gm.Span{Start: 12, End: 20}, ExecutionStatus: gm.ExecutionStatus_PASSED, ExecutionTime: 5,
		Tags: []string{"allure.label.owner:team", "allure.issue:GA-1"}, Contexts: []*gm.ProtoItem{contextStep}, TearDownSteps: []*gm.ProtoItem{teardownStep},
		ScenarioItems:   []*gm.ProtoItem{{ItemType: gm.ProtoItem_Comment, Comment: &gm.ProtoComment{Text: "scenario description"}}, {ItemType: gm.ProtoItem_Step, Step: step}},
		PreHookFailure:  &gm.ProtoHookFailure{ErrorMessage: "hook exploded", StackTrace: "trace", FailureScreenshotFile: image},
		PreHookMessages: []string{"scenario before"}, PostHookMessages: []string{"scenario after"}, PostHookScreenshots: [][]byte{[]byte("scenario legacy")},
	}
	scenarioTable := &gm.ProtoTable{Headers: &gm.ProtoTableRow{Cells: []string{"account", "account"}}, Rows: []*gm.ProtoTableRow{{Cells: []string{"A", "B"}}}}
	item := &gm.ProtoItem{ItemType: gm.ProtoItem_TableDrivenScenario, FileName: filepath.Join(root, "specs", "complex.spec"), TableDrivenScenario: &gm.ProtoTableDrivenScenario{Scenario: scenario, IsScenarioTableDriven: true, ScenarioDataTable: scenarioTable, ScenarioTableRowIndex: 0}}
	spec := &gm.ProtoSpec{
		SpecHeading: "Complex specification", FileName: filepath.Join(root, "specs", "complex.spec"), Tags: []string{"component"}, Items: []*gm.ProtoItem{item},
		PreHookFailures:  []*gm.ProtoHookFailure{{ErrorMessage: "spec before", FailureScreenshot: []byte("spec legacy")}},
		PostHookFailures: []*gm.ProtoHookFailure{{ErrorMessage: "spec after", ScreenShot: []byte("spec old")}},
		PreHookMessages:  []string{"spec message"}, PostHookScreenshotFiles: []string{image},
	}
	suite := &gm.ProtoSuiteResult{
		ProjectName: "demo", TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 10, SpecResults: []*gm.ProtoSpecResult{{TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 10, ProtoSpec: spec}},
		PreHookFailure: &gm.ProtoHookFailure{ErrorMessage: "suite before", FailureScreenshot: []byte("suite legacy")}, PostHookFailure: &gm.ProtoHookFailure{ErrorMessage: "suite after", ScreenShot: []byte("suite old")},
		PreHookMessages: []string{"suite message"}, PostHookScreenshotFiles: []string{image},
	}

	got, err := mapper.MapSuite(context.Background(), suite, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || len(got.Containers) != 3 || len(writer.values) < 12 {
		t.Fatalf("results=%d containers=%d attachments=%d warnings=%v", len(got.Results), len(got.Containers), len(writer.values), got.Warnings)
	}
	result := got.Results[0].Result
	if result.Status != model.StatusBroken || result.Description != "scenario description" || len(result.Parameters) < 3 {
		t.Fatalf("mapped result: %+v", result)
	}
	if !strings.Contains(result.Links[len(result.Links)-1].URL, "specs/complex.spec#L12") {
		t.Fatalf("source link: %+v", result.Links)
	}
	if len(got.Containers[0].Befores)+len(got.Containers[0].Afters) == 0 {
		t.Fatal("fixtures were not emitted")
	}
	for _, data := range writer.values {
		if strings.Contains(string(data), "secret") {
			t.Fatalf("redaction failed in attachment: %q", data)
		}
	}
}

func TestMapLiveScenarioValidationAndExistingUUID(t *testing.T) {
	mapper := testMapper()
	if _, _, err := mapper.MapLiveScenario(context.Background(), nil, ""); err == nil {
		t.Fatal("expected invalid live request error")
	}
	scenario := &gm.ProtoScenario{ID: "live", ScenarioHeading: "Live", ExecutionStatus: gm.ExecutionStatus_PASSED, ScenarioItems: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Step, Step: passedStep("pass", 1)}}}
	item := &gm.ProtoItem{ItemType: gm.ProtoItem_Scenario, Scenario: scenario, FileName: "specs/live.spec"}
	request := &gm.ScenarioExecutionEndingRequest{Stream: 3, CurrentExecutionInfo: &gm.ExecutionInfo{RunnerId: 7, CurrentSpec: &gm.SpecInfo{Name: "Live spec", FileName: "specs/live.spec", Tags: []string{"smoke"}}, CurrentScenario: &gm.ScenarioInfo{Retries: &gm.ScenarioRetriesInfo{CurrentRetry: 2}}}, ScenarioResult: &gm.ProtoScenarioResult{ProtoItem: item, TimestampISO: "invalid", ExecutionTime: 2}}
	artifact, _, err := mapper.MapLiveScenario(context.Background(), request, "existing-uuid")
	if err != nil {
		t.Fatal(err)
	}
	if artifact.Result.UUID != "existing-uuid" || !strings.Contains(artifact.Key, "retry=2") {
		t.Fatalf("live artifact: %+v", artifact)
	}
}

func TestMalformedSuitesAndHelpers(t *testing.T) {
	mapper := testMapper()
	for _, suite := range []*gm.ProtoSuiteResult{
		{TimestampISO: "invalid", Failed: true},
		{TimestampISO: "2026-01-01T00:00:00Z", SpecResults: []*gm.ProtoSpecResult{nil, {}}},
	} {
		got, err := mapper.MapSuite(context.Background(), suite, nil, nil)
		if err != nil || len(got.Results) == 0 {
			t.Fatalf("malformed suite was lost: results=%v err=%v", got.Results, err)
		}
	}
	if _, _, err := mapper.mapScenario(context.Background(), &gm.ProtoSpec{}, &gm.ProtoItem{ItemType: gm.ProtoItem_Scenario}, 0, LiveInfo{}, "", false, nil); err == nil {
		t.Fatal("expected missing scenario error")
	}
	if link := mapper.sourceLink("specs/a.spec", &gm.Span{Start: 1}); link.URL != "" {
		t.Fatal("source link should be disabled without a template")
	}
	mapper.cfg.SourceLinkTemplate = "file:///{path}"
	if link := mapper.sourceLink("specs/a.spec", &gm.Span{Start: 1}); link.URL != "" {
		t.Fatal("unsafe source scheme accepted")
	}
	mapper.cfg.MaximumNestingDepth = 0
	steps, _, _, err := mapper.mapItems(context.Background(), nil, 5, timing.Range{Start: 5, Stop: 10}, 1, false)
	if err != nil || len(steps) != 1 || steps[0].Status != model.StatusBroken {
		t.Fatalf("nesting guard: steps=%v err=%v", steps, err)
	}
	if scenario, table := scenarioFromItem(nil); scenario != nil || table != nil || isScenario(nil) {
		t.Fatal("nil item helper mismatch")
	}
	if got := screenshotName(0, "failure.png"); got != "Failure screenshot" {
		t.Fatalf("screenshotName = %q", got)
	}
	if got := screenshotName(1, "other.png"); got != "Screenshot 2" {
		t.Fatalf("screenshotName = %q", got)
	}
	if projectFromCanonical("without-delimiter") != "Gauge project" || parseTime("invalid", 123).UnixMilli() != 123 || first(" ", "value") != "value" || min64(1, 2) != 1 {
		t.Fatal("fallback helper mismatch")
	}
	if retryIndex(int64(^uint32(0)>>1)+2) != int32(^uint32(0)>>1) {
		t.Fatal("retry index did not saturate")
	}
}

func TestParameterShapesAndStatusHelpers(t *testing.T) {
	mapper := testMapper()
	table := &gm.ProtoTable{Headers: &gm.ProtoTableRow{Cells: []string{"name", ""}}, Rows: []*gm.ProtoTableRow{{Cells: []string{"one", "two"}}}}
	driven := &gm.ProtoTableDrivenScenario{IsSpecTableDriven: true, TableRowIndex: 0}
	params, logical := mapper.testParameters(driven, table)
	if len(params) != 2 || params[1].Name != "column_2" || len(logical) != 2 {
		t.Fatalf("spec parameters: %v %v", params, logical)
	}
	step := &gm.ProtoStep{Fragments: []*gm.Fragment{parameterFragment("", "a", gm.Parameter_Static, nil), parameterFragment("dup", "b", gm.Parameter_Static, nil), parameterFragment("dup", "", gm.Parameter_Table, table)}}
	if got := mapper.stepParameters(step); len(got) != 3 || got[0].Name != "arg_1" || got[2].Name != "dup_2" {
		t.Fatalf("step parameters: %+v", got)
	}
	if got := aggregateStepStatus([]model.StepResult{{Status: model.StatusSkipped}, {Status: model.StatusFailed}}); got != model.StatusFailed {
		t.Fatalf("aggregate = %s", got)
	}
	if got := aggregateStepStatus([]model.StepResult{{Status: model.StatusBroken}}); got != model.StatusBroken {
		t.Fatalf("aggregate = %s", got)
	}
	if tableJSON(nil) != "" || findSpecTable(nil) != nil || findSpecTable(&gm.ProtoSpec{}) != nil || matches("name", []string{"["}) {
		t.Fatal("nil/invalid helper mismatch")
	}
}
