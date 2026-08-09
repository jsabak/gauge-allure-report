// SPDX-License-Identifier: Apache-2.0

package status

import (
	"testing"

	"github.com/allure-framework/allure-go/commons/model"
	gm "github.com/getgauge/gauge-proto/go/gauge_messages"
)

func TestFromExecutionStatus(t *testing.T) {
	tests := []struct {
		name   string
		gauge  gm.ExecutionStatus
		reason string
		kind   ErrorKind
		want   model.Status
	}{
		{"passed", gm.ExecutionStatus_PASSED, "", ErrorNone, model.StatusPassed},
		{"assertion", gm.ExecutionStatus_FAILED, "", ErrorAssertion, model.StatusFailed},
		{"verification", gm.ExecutionStatus_FAILED, "", ErrorVerification, model.StatusFailed},
		{"ambiguous", gm.ExecutionStatus_FAILED, "", ErrorNone, model.StatusBroken},
		{"unexpected", gm.ExecutionStatus_FAILED, "", ErrorUnexpected, model.StatusBroken},
		{"hook", gm.ExecutionStatus_FAILED, "", ErrorHook, model.StatusBroken},
		{"parse", gm.ExecutionStatus_FAILED, "", ErrorParse, model.StatusBroken},
		{"skipped enum", gm.ExecutionStatus_SKIPPED, "", ErrorNone, model.StatusSkipped},
		{"skipped reason", gm.ExecutionStatus_NOTEXECUTED, "filtered", ErrorNone, model.StatusSkipped},
		{"not executed unknown", gm.ExecutionStatus_NOTEXECUTED, "", ErrorNone, model.StatusBroken},
		{"unknown", gm.ExecutionStatus(99), "", ErrorNone, model.StatusBroken},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FromExecutionStatus(test.gauge, test.reason, test.kind); got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestFromStep(t *testing.T) {
	tests := []struct {
		name  string
		value *gm.ProtoStepExecutionResult
		want  model.Status
	}{
		{"nil", nil, model.StatusBroken},
		{"passed", &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{}}, model.StatusPassed},
		{"assertion", &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{Failed: true, ErrorMessage: "AssertionError: no"}}, model.StatusFailed},
		{"unexpected", &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{Failed: true, ErrorMessage: "RuntimeError: no"}}, model.StatusBroken},
		{"verification", &gm.ProtoStepExecutionResult{ExecutionResult: &gm.ProtoExecutionResult{Failed: true, RecoverableError: true, ErrorMessage: "no"}}, model.StatusFailed},
		{"skipped", &gm.ProtoStepExecutionResult{Skipped: true, SkippedReason: "later"}, model.StatusSkipped},
		{"hook", &gm.ProtoStepExecutionResult{PreHookFailure: &gm.ProtoHookFailure{ErrorMessage: "setup"}}, model.StatusBroken},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := FromStep(test.value)
			if got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestFailureKind(t *testing.T) {
	tests := []struct {
		name  string
		value *gm.ProtoExecutionResult
		want  ErrorKind
	}{
		{"nil", nil, ErrorProtocol},
		{"recoverable", &gm.ProtoExecutionResult{RecoverableError: true}, ErrorVerification},
		{"verification enum", &gm.ProtoExecutionResult{ErrorType: gm.ProtoExecutionResult_VERIFICATION}, ErrorVerification},
		{"python assertion", &gm.ProtoExecutionResult{ErrorMessage: "AssertionError: mismatch"}, ErrorAssertion},
		{"node assertion", &gm.ProtoExecutionResult{StackTrace: "AssertionError [ERR_ASSERTION]: values differ"}, ErrorAssertion},
		{"runtime exception", &gm.ProtoExecutionResult{ErrorMessage: "RuntimeError: database unavailable"}, ErrorUnexpected},
		{"ambiguous default enum", &gm.ProtoExecutionResult{ErrorType: gm.ProtoExecutionResult_ASSERTION, ErrorMessage: "boom"}, ErrorUnexpected},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := FailureKind(test.value); got != test.want {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}
