// SPDX-License-Identifier: Apache-2.0

package mapping

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/allure-framework/allure-go/commons/model"
	gm "github.com/getgauge/gauge-proto/go/gauge_messages"
	"github.com/jsabak/gauge-allure-report/internal/config"
)

type sequenceUUID struct {
	values []string
	index  int
}

func (s *sequenceUUID) New() (string, error) {
	value := s.values[s.index]
	s.index++
	return value, nil
}

type fixedClock struct{ value time.Time }

func (f fixedClock) Now() time.Time { return f.value }

func TestMapPassingScenarioGolden(t *testing.T) {
	mapper := testMapper()
	scenario := &gm.ProtoScenario{
		ID: "s1", ScenarioHeading: "passes", ExecutionStatus: gm.ExecutionStatus_PASSED, ExecutionTime: 10,
		Tags:          []string{"allure.label.owner:alice"},
		ScenarioItems: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Step, Step: passedStep("a passing step", 10)}},
	}
	spec := &gm.ProtoSpec{SpecHeading: "Passing", FileName: `C:\project\specs\pass.spec`, Tags: []string{"smoke"}, Items: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Scenario, Scenario: scenario}}}
	suite := &gm.ProtoSuiteResult{ProjectName: "demo", TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 10, SpecResults: []*gm.ProtoSpecResult{{TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 10, ProtoSpec: spec}}}
	got, err := mapper.MapSuite(context.Background(), suite, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || len(got.Containers) != 3 {
		t.Fatalf("results=%d containers=%d warnings=%v", len(got.Results), len(got.Containers), got.Warnings)
	}
	payload, err := json.MarshalIndent(got.Results[0].Result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, '\n')
	want, err := os.ReadFile(filepath.Join("testdata", "passing-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != string(want) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", payload, want)
	}
}

func TestMapTableRetryConceptFailureSkipAndErrors(t *testing.T) {
	mapper := testMapper()
	table := &gm.ProtoTable{Headers: &gm.ProtoTableRow{Cells: []string{"amount", "currency"}}, Rows: []*gm.ProtoTableRow{{Cells: []string{"100", "PLN"}}}}
	failed := &gm.ProtoScenario{ID: "refund", ScenarioHeading: "refund", ExecutionStatus: gm.ExecutionStatus_FAILED, ExecutionTime: 5, RetriesCount: 2,
		ScenarioItems: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Concept, Concept: &gm.ProtoConcept{ConceptStep: &gm.ProtoStep{ActualText: "refund concept"}, ConceptExecutionResult: passedExecution(5), Steps: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Step, Step: failedStep("refund fails")}}}}}}
	skipped := &gm.ProtoScenario{ID: "later", ScenarioHeading: "later", ExecutionStatus: gm.ExecutionStatus_SKIPPED, SkipErrors: []string{"not available"}}
	spec := &gm.ProtoSpec{SpecHeading: "Refunds", FileName: "/project/specs/refunds.spec", IsTableDriven: true, Items: []*gm.ProtoItem{
		{ItemType: gm.ProtoItem_Table, Table: table},
		{ItemType: gm.ProtoItem_TableDrivenScenario, TableDrivenScenario: &gm.ProtoTableDrivenScenario{Scenario: failed, IsSpecTableDriven: true, TableRowIndex: 0}},
		{ItemType: gm.ProtoItem_Scenario, Scenario: skipped},
	}}
	suite := &gm.ProtoSuiteResult{TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 10, SpecResults: []*gm.ProtoSpecResult{{ProtoSpec: spec, TimestampISO: "2026-01-01T00:00:00Z", ExecutionTime: 10, Errors: []*gm.Error{{Type: gm.Error_VALIDATION_ERROR, Filename: spec.FileName, LineNumber: 7, Message: "missing step"}}}}}
	got, err := mapper.MapSuite(context.Background(), suite, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[model.Status]int{}
	for _, result := range got.Results {
		statuses[result.Result.Status]++
	}
	if statuses[model.StatusFailed] != 1 || statuses[model.StatusSkipped] != 1 || statuses[model.StatusBroken] != 1 {
		t.Fatalf("statuses: %+v", statuses)
	}
	var retry model.TestResult
	for _, result := range got.Results {
		if result.Result.Name == "refund" {
			retry = result.Result
		}
	}
	if len(retry.Parameters) != 3 || retry.Parameters[2].Name != "retry" || !retry.Parameters[2].Excluded {
		t.Fatalf("parameters: %+v", retry.Parameters)
	}
	if len(retry.Steps) != 1 || len(retry.Steps[0].Steps) != 1 || len(retry.Steps[0].Steps[0].Steps) != 1 {
		t.Fatalf("concept tree: %+v", retry.Steps)
	}
}

func TestRetryIndexNormalizesGaugeAttemptCount(t *testing.T) {
	tests := []struct {
		attempts int64
		want     int32
	}{{0, 0}, {1, 0}, {2, 1}, {4, 3}}
	for _, test := range tests {
		if got := retryIndex(test.attempts); got != test.want {
			t.Fatalf("retryIndex(%d)=%d, want %d", test.attempts, got, test.want)
		}
	}
}

func TestMapNilAndEmptySuite(t *testing.T) {
	mapper := testMapper()
	nilOutput, err := mapper.MapSuite(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if nilOutput.Results[0].Result.Status != model.StatusBroken {
		t.Fatal("nil suite must be broken")
	}
	empty, err := mapper.MapSuite(context.Background(), &gm.ProtoSuiteResult{TimestampISO: "2026-01-01T00:00:00Z"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Results[0].Result.Status != model.StatusSkipped {
		t.Fatal("empty successful suite must be skipped")
	}
}

func TestScenarioFailureClassification(t *testing.T) {
	mapper := testMapper()
	tests := []struct {
		name    string
		message string
		trace   string
		want    model.Status
	}{
		{"assertion", "AssertionError: expected true", "", model.StatusFailed},
		{"unexpected", "RuntimeError: service unavailable", "Traceback", model.StatusBroken},
		{"ambiguous", "boom", "", model.StatusBroken},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scenario := &gm.ProtoScenario{ExecutionStatus: gm.ExecutionStatus_FAILED, ScenarioItems: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Step, Step: &gm.ProtoStep{StepExecutionResult: &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{Failed: true, ErrorMessage: test.message, StackTrace: test.trace}}}}}}
			got, _ := mapper.scenarioStatus(scenario)
			if got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestChunkFlagDoesNotDiscardGrpcSuitePayload(t *testing.T) {
	mapper := testMapper()
	scenario := &gm.ProtoScenario{ID: "chunked", ScenarioHeading: "chunked compatibility", ExecutionStatus: gm.ExecutionStatus_PASSED, ScenarioItems: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Step, Step: passedStep("pass", 1)}}}
	suite := &gm.ProtoSuiteResult{Chunked: true, ChunkSize: 4, TimestampISO: "2026-01-01T00:00:00Z", SpecResults: []*gm.ProtoSpecResult{{ProtoSpec: &gm.ProtoSpec{FileName: "specs/chunk.spec", Items: []*gm.ProtoItem{{ItemType: gm.ProtoItem_Scenario, Scenario: scenario}}}}}}
	got, err := mapper.MapSuite(context.Background(), suite, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].Result.Name != "chunked compatibility" {
		t.Fatalf("results: %+v", got.Results)
	}
}

func TestPrivateParameterPolicy(t *testing.T) {
	mapper := testMapper()
	mapper.cfg.MaskParameters = []string{"password"}
	mapper.cfg.HideParameters = []string{"token"}
	mapper.cfg.ExcludeHistoryParameters = []string{"volatile"}
	masked, maskedIdentity := mapper.privateParameter("password", "secret")
	hidden, hiddenIdentity := mapper.privateParameter("token", "abc")
	excluded, _ := mapper.privateParameter("volatile", "now")
	if masked.Value != "[MASKED]" || hidden.Value != "[HIDDEN]" || maskedIdentity == "secret" || hiddenIdentity == "abc" || !excluded.Excluded {
		t.Fatalf("privacy failure: %+v %+v %+v", masked, hidden, excluded)
	}
}

func TestTableJSONDeterministic(t *testing.T) {
	table := &gm.ProtoTable{Headers: &gm.ProtoTableRow{Cells: []string{"a"}}, Rows: []*gm.ProtoTableRow{{Cells: []string{"ą"}}}}
	if got, want := tableJSON(table), `[["a"],["ą"]]`; got != want {
		t.Fatalf("got %q", got)
	}
}

func FuzzTableJSON(f *testing.F) {
	f.Add("header", "value")
	f.Add(string([]byte{0xff}), string([]byte{0xfe}))
	f.Fuzz(func(t *testing.T, header, value string) {
		got := tableJSON(&gm.ProtoTable{Headers: &gm.ProtoTableRow{Cells: []string{header}}, Rows: []*gm.ProtoTableRow{{Cells: []string{value}}}})
		var decoded [][]string
		if err := json.Unmarshal([]byte(got), &decoded); err != nil {
			t.Fatal(err)
		}
		if utf8.ValidString(header) && utf8.ValidString(value) && !reflect.DeepEqual(decoded, [][]string{{header}, {value}}) {
			t.Fatalf("decoded: %#v", decoded)
		}
		if len(decoded) != 2 || len(decoded[0]) != 1 || len(decoded[1]) != 1 || !utf8.ValidString(decoded[0][0]) || !utf8.ValidString(decoded[1][0]) {
			t.Fatalf("JSON did not normalize strings safely: %#v", decoded)
		}
	})
}

func testMapper() *Mapper {
	cfg := config.Defaults()
	values := make([]string, 40)
	for index := range values {
		values[index] = "00000000-0000-4000-8000-" + leftPad(index+1, 12)
	}
	return &Mapper{cfg: cfg, projectRoot: `C:\project`, env: map[string]string{"GAUGE_PROJECT_NAME": "demo", "test_language": "python"}, host: "test-host", uuids: &sequenceUUID{values: values}, clock: fixedClock{value: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}}
}

func leftPad(value, width int) string {
	result := ""
	for len(result)+len(stringValue(value)) < width {
		result += "0"
	}
	return result + stringValue(value)
}
func stringValue(value int) string {
	if value == 0 {
		return "0"
	}
	digits := ""
	for value > 0 {
		digits = string(rune('0'+value%10)) + digits
		value /= 10
	}
	return digits
}
func passedStep(name string, duration int64) *gm.ProtoStep {
	return &gm.ProtoStep{ActualText: name, StepExecutionResult: passedExecution(duration)}
}
func passedExecution(duration int64) *gm.ProtoStepExecutionResult {
	return &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{ExecutionTime: duration}}
}
func failedStep(name string) *gm.ProtoStep {
	return &gm.ProtoStep{ActualText: name, StepExecutionResult: &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{Failed: true, ErrorMessage: "expected 2 but got 1", StackTrace: "AssertionError: mismatch", ExecutionTime: 1}}}
}
