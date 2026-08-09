// SPDX-License-Identifier: Apache-2.0

// Package status centralizes Gauge-to-Allure status decisions.
package status

import (
	"strings"

	"github.com/allure-framework/allure-go/commons/model"
	gm "github.com/getgauge/gauge-proto/go/gauge_messages"
)

// ErrorKind classifies failures whose origin is not captured by Gauge's status enum.
type ErrorKind string

const (
	ErrorNone         ErrorKind = "none"
	ErrorAssertion    ErrorKind = "assertion"
	ErrorVerification ErrorKind = "verification"
	ErrorUnexpected   ErrorKind = "unexpected"
	ErrorHook         ErrorKind = "hook"
	ErrorParse        ErrorKind = "parse"
	ErrorValidation   ErrorKind = "validation"
	ErrorProtocol     ErrorKind = "protocol"
	ErrorInternal     ErrorKind = "internal"
)

// FromExecutionStatus maps every Gauge enum value. Unknown and not-executed
// values are broken unless the caller supplies an explicit skip reason.
func FromExecutionStatus(value gm.ExecutionStatus, skipReason string, kind ErrorKind) model.Status {
	if strings.TrimSpace(skipReason) != "" || value == gm.ExecutionStatus_SKIPPED {
		return model.StatusSkipped
	}
	switch value {
	case gm.ExecutionStatus_PASSED:
		return model.StatusPassed
	case gm.ExecutionStatus_FAILED:
		if kind == ErrorAssertion || kind == ErrorVerification {
			return model.StatusFailed
		}
		return model.StatusBroken
	case gm.ExecutionStatus_NOTEXECUTED:
		return model.StatusBroken
	default:
		return model.StatusBroken
	}
}

// FromStep maps a nil-safe Gauge step execution result.
func FromStep(value *gm.ProtoStepExecutionResult) (model.Status, *model.StatusDetails) {
	if value == nil {
		return model.StatusBroken, &model.StatusDetails{Message: "Gauge did not provide a step execution result"}
	}
	if value.GetSkipped() {
		return model.StatusSkipped, details(value.GetSkippedReason(), "")
	}
	if value.GetPreHookFailure() != nil || value.GetPostHookFailure() != nil {
		failure := value.GetPreHookFailure()
		if failure == nil {
			failure = value.GetPostHookFailure()
		}
		return model.StatusBroken, details(failure.GetErrorMessage(), failure.GetStackTrace())
	}
	execution := value.GetExecutionResult()
	if execution == nil {
		return model.StatusBroken, &model.StatusDetails{Message: "Gauge did not provide an execution result"}
	}
	if execution.GetSkipScenario() {
		return model.StatusSkipped, details(execution.GetErrorMessage(), execution.GetStackTrace())
	}
	if !execution.GetFailed() {
		return model.StatusPassed, nil
	}
	return FromExecutionStatus(gm.ExecutionStatus_FAILED, "", FailureKind(execution)), details(execution.GetErrorMessage(), execution.GetStackTrace())
}

// FailureKind compensates for runner implementations which leave errorType at
// its protobuf default (ASSERTION) for every exception. Explicit verification
// and recoverable failures win; otherwise assertion diagnostics are recognized
// conservatively and an ambiguous exception is treated as infrastructure/code
// breakage rather than a test assertion.
func FailureKind(value *gm.ProtoExecutionResult) ErrorKind {
	if value == nil {
		return ErrorProtocol
	}
	if value.GetRecoverableError() || value.GetErrorType() == gm.ProtoExecutionResult_VERIFICATION {
		return ErrorVerification
	}
	diagnostic := strings.ToLower(value.GetErrorMessage() + "\n" + value.GetStackTrace())
	for _, marker := range []string{
		"assertionerror", "assertion failed", "assert failed", "[err_assertion]",
		"assertionfailederror", "comparisonfailure", "comparison failure",
		"expected:", "expected ", " but got ", " but was ", "should equal",
		"should be", "verification failed", "\ne   assert ",
	} {
		if strings.Contains(diagnostic, marker) {
			return ErrorAssertion
		}
	}
	return ErrorUnexpected
}

// FailureDetails truncates pathological protocol payloads while retaining useful diagnostics.
func FailureDetails(message, trace string) *model.StatusDetails { return details(message, trace) }

func details(message, trace string) *model.StatusDetails {
	message = truncate(strings.TrimSpace(message), 16*1024)
	trace = truncate(strings.TrimSpace(trace), 256*1024)
	if message == "" && trace == "" {
		return nil
	}
	return &model.StatusDetails{Message: message, Trace: trace}
}

func truncate(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	return value[:maximum] + "\n… truncated by allure-report"
}
