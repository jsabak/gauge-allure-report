// SPDX-License-Identifier: Apache-2.0

//lint:file-ignore SA1019 Gauge still populates these deprecated screenshot fields in supported legacy protocol paths.

// Package mapping converts Gauge protocol results into the native Allure model.
package mapping

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/allure-framework/allure-go/commons/model"
	gm "github.com/getgauge/gauge-proto/go/gauge_messages"
	"github.com/jsabak/gauge-allure-report/internal/attachments"
	"github.com/jsabak/gauge-allure-report/internal/config"
	"github.com/jsabak/gauge-allure-report/internal/identity"
	"github.com/jsabak/gauge-allure-report/internal/metadata"
	statusmap "github.com/jsabak/gauge-allure-report/internal/status"
	"github.com/jsabak/gauge-allure-report/internal/timing"
)

// Clock supplies deterministic fallbacks.
type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// LiveInfo enriches final results with lifecycle-only attempt data.
type LiveInfo struct {
	Range  timing.Range
	Stream int32
	Runner int32
	Retry  int32
}

// ResultArtifact associates a stable attempt key with a native Allure result.
type ResultArtifact struct {
	Key    string
	Result model.TestResult
}

// SuiteOutput is the deterministic result of suite reconciliation.
type SuiteOutput struct {
	Results    []ResultArtifact
	Containers []model.TestResultContainer
	Warnings   []string
}

// Mapper has no mutable run state and is safe to call concurrently.
type Mapper struct {
	cfg         config.Config
	projectRoot string
	env         map[string]string
	host        string
	uuids       identity.UUIDGenerator
	attachments *attachments.Manager
	clock       Clock
}

// New constructs a mapper behind injected boundaries.
func New(cfg config.Config, projectRoot string, env map[string]string, uuids identity.UUIDGenerator, manager *attachments.Manager, clock Clock) *Mapper {
	if clock == nil {
		clock = realClock{}
	}
	host, _ := os.Hostname()
	return &Mapper{cfg: cfg, projectRoot: projectRoot, env: env, host: host, uuids: uuids, attachments: manager, clock: clock}
}

// MapLiveScenario creates a promptly available result without copying final-only attachments.
func (m *Mapper) MapLiveScenario(ctx context.Context, request *gm.ScenarioExecutionEndingRequest, existingUUID string) (ResultArtifact, []string, error) {
	if request == nil || request.GetScenarioResult() == nil || request.GetScenarioResult().GetProtoItem() == nil {
		return ResultArtifact{}, nil, fmt.Errorf("map live scenario: missing scenario result")
	}
	info := request.GetCurrentExecutionInfo()
	specInfo := info.GetCurrentSpec()
	item := request.GetScenarioResult().GetProtoItem()
	retry := int32(0)
	if info.GetCurrentScenario() != nil && info.GetCurrentScenario().GetRetries() != nil {
		retry = info.GetCurrentScenario().GetRetries().GetCurrentRetry()
	}
	rangeValue := timing.Resolve(request.GetScenarioResult().GetTimestampISO(), request.GetScenarioResult().GetExecutionTime(), m.clock.Now())
	live := LiveInfo{Range: rangeValue, Stream: request.GetStream(), Runner: info.GetRunnerId(), Retry: retry}
	spec := &gm.ProtoSpec{SpecHeading: specInfo.GetName(), FileName: specInfo.GetFileName(), Tags: specInfo.GetTags(), Items: []*gm.ProtoItem{item}}
	artifact, warnings, err := m.mapScenario(ctx, spec, item, retry, live, existingUUID, false, nil)
	return artifact, warnings, err
}

// MapSuite reconciles the final suite with lifecycle UUID/timing state.
func (m *Mapper) MapSuite(ctx context.Context, suite *gm.ProtoSuiteResult, live map[string]LiveInfo, existing map[string]string) (SuiteOutput, error) {
	if suite == nil {
		result, err := m.synthetic("[Gauge] Reporter received an empty suite result", "protocol:nil-suite", "Gauge Core sent an empty SuiteExecutionResult", model.StatusBroken, m.clock.Now())
		return SuiteOutput{Results: []ResultArtifact{result}}, err
	}
	output := SuiteOutput{}
	suiteRange := timing.Resolve(suite.GetTimestampISO(), suite.GetExecutionTime(), m.clock.Now())
	allChildren := make([]string, 0)
	for specIndex, specResult := range suite.GetSpecResults() {
		if specResult == nil {
			artifact, err := m.synthetic("[Gauge] Invalid empty specification result", fmt.Sprintf("protocol:nil-spec:%d", specIndex), "Gauge supplied a nil specification result", model.StatusBroken, time.UnixMilli(suiteRange.Start))
			if err != nil {
				return output, err
			}
			output.Results = append(output.Results, artifact)
			allChildren = append(allChildren, artifact.Result.UUID)
			continue
		}
		spec := specResult.GetProtoSpec()
		if spec == nil {
			artifact, err := m.synthetic("[Gauge] Invalid specification result", fmt.Sprintf("protocol:missing-proto-spec:%d", specIndex), "Gauge supplied a specification result without a specification", model.StatusBroken, time.UnixMilli(suiteRange.Start))
			if err != nil {
				return output, err
			}
			output.Results = append(output.Results, artifact)
			allChildren = append(allChildren, artifact.Result.UUID)
			continue
		}
		for errorIndex, gaugeError := range specResult.GetErrors() {
			if gaugeError == nil {
				continue
			}
			kind := "Validation error"
			if gaugeError.GetType() == gm.Error_PARSE_ERROR {
				kind = "Parse error"
			}
			path := identity.CanonicalPath(m.projectRoot, first(gaugeError.GetFilename(), spec.GetFileName()))
			name := fmt.Sprintf("[Gauge] %s: %s:%d", kind, path, gaugeError.GetLineNumber())
			key := fmt.Sprintf("error:%s:%d:%d:%s", path, gaugeError.GetLineNumber(), errorIndex, gaugeError.GetMessage())
			artifact, err := m.synthetic(name, key, gaugeError.GetMessage(), model.StatusBroken, parseTime(specResult.GetTimestampISO(), suiteRange.Start))
			if err != nil {
				return output, err
			}
			output.Results = append(output.Results, artifact)
			allChildren = append(allChildren, artifact.Result.UUID)
		}
		specRange := timing.Resolve(specResult.GetTimestampISO(), specResult.GetExecutionTime(), time.UnixMilli(suiteRange.Start))
		cursor := specRange.Start
		specChildren := make([]string, 0)
		for _, item := range spec.GetItems() {
			if !isScenario(item) {
				continue
			}
			scenario, _ := scenarioFromItem(item)
			if scenario == nil {
				continue
			}
			retry := retryIndex(scenario.GetRetriesCount())
			canonical, _, specRow, scenarioRow := m.scenarioIdentity(spec, item, retry)
			key := identity.AttemptKey(canonical, specRow, scenarioRow, retry)
			liveInfo, found := live[key]
			if !found {
				duration := scenario.GetExecutionTime()
				liveInfo = LiveInfo{Range: timing.Range{Start: cursor, Stop: cursor + max64(duration, 0)}, Retry: retry}
				if liveInfo.Range.Stop > specRange.Stop {
					liveInfo.Range.Stop = specRange.Stop
				}
			}
			cursor = max64(cursor, liveInfo.Range.Stop)
			uuid := existing[key]
			artifact, warnings, err := m.mapScenario(ctx, spec, item, retry, liveInfo, uuid, true, findSpecTable(spec))
			if err != nil {
				if m.cfg.Strict {
					return output, err
				}
				output.Warnings = append(output.Warnings, err.Error())
				continue
			}
			output.Warnings = append(output.Warnings, warnings...)
			output.Results = append(output.Results, artifact)
			allChildren = append(allChildren, artifact.Result.UUID)
			specChildren = append(specChildren, artifact.Result.UUID)
			container, warnings, err := m.scenarioContainer(ctx, scenario, artifact.Result.UUID, liveInfo.Range)
			output.Warnings = append(output.Warnings, warnings...)
			if err != nil && m.cfg.Strict {
				return output, err
			}
			if container.UUID != "" {
				output.Containers = append(output.Containers, container)
			}
		}
		container, warnings, err := m.specContainer(ctx, spec, specChildren, specRange)
		output.Warnings = append(output.Warnings, warnings...)
		if err != nil && m.cfg.Strict {
			return output, err
		}
		if container.UUID != "" {
			output.Containers = append(output.Containers, container)
		}
	}
	if len(output.Results) == 0 {
		name, message, status := "[Gauge] No matching scenarios", "Gauge reported no executed or diagnosable scenarios", model.StatusSkipped
		if suite.GetFailed() {
			name, message, status = "[Gauge] Suite aborted before scenarios executed", "Gauge marked the empty suite as failed", model.StatusBroken
		}
		artifact, err := m.synthetic(name, "suite:empty", message, status, time.UnixMilli(suiteRange.Start))
		if err != nil {
			return output, err
		}
		output.Results = append(output.Results, artifact)
		allChildren = append(allChildren, artifact.Result.UUID)
	}
	suiteContainer, warnings, err := m.suiteContainer(ctx, suite, allChildren, suiteRange)
	output.Warnings = append(output.Warnings, warnings...)
	if err != nil && m.cfg.Strict {
		return output, err
	}
	if suiteContainer.UUID != "" {
		output.Containers = append(output.Containers, suiteContainer)
	}
	sort.SliceStable(output.Results, func(i, j int) bool {
		if output.Results[i].Result.Start == output.Results[j].Result.Start {
			return output.Results[i].Key < output.Results[j].Key
		}
		return output.Results[i].Result.Start < output.Results[j].Result.Start
	})
	sort.SliceStable(output.Containers, func(i, j int) bool { return output.Containers[i].UUID < output.Containers[j].UUID })
	return output, nil
}

func (m *Mapper) mapScenario(ctx context.Context, spec *gm.ProtoSpec, item *gm.ProtoItem, retry int32, live LiveInfo, existingUUID string, copyAttachments bool, specTable *gm.ProtoTable) (ResultArtifact, []string, error) {
	scenario, tableDriven := scenarioFromItem(item)
	if scenario == nil {
		return ResultArtifact{}, nil, fmt.Errorf("scenario item has no scenario")
	}
	canonical, path, specRow, scenarioRow := m.scenarioIdentity(spec, item, retry)
	key := identity.AttemptKey(canonical, specRow, scenarioRow, retry)
	uuid := existingUUID
	var err error
	if uuid == "" {
		uuid, err = m.uuids.New()
	}
	if err != nil {
		return ResultArtifact{}, nil, err
	}
	parameters, logical := m.testParameters(tableDriven, specTable)
	promoted, promotedLogical := m.promotedParameters(scenario)
	parameters = append(parameters, promoted...)
	logical = append(logical, promotedLogical...)
	if retry > 0 {
		parameters = append(parameters, model.Parameter{Name: "retry", Value: fmt.Sprint(retry), Excluded: true})
	}
	labels := []model.Label{{Name: "framework", Value: "Gauge"}}
	if language := m.env["test_language"]; language != "" {
		labels = append(labels, model.Label{Name: "language", Value: language})
	}
	if m.host != "" {
		labels = append(labels, model.Label{Name: "host", Value: m.host})
	}
	labels = append(labels,
		model.Label{Name: "thread", Value: fmt.Sprintf("stream-%d/runner-%d", live.Stream, live.Runner)},
		model.Label{Name: "parentSuite", Value: first(m.env["GAUGE_PROJECT_NAME"], projectFromCanonical(canonical))},
		model.Label{Name: "suite", Value: first(spec.GetSpecHeading(), filepath.Base(path))},
		model.Label{Name: "package", Value: strings.TrimSuffix(path, filepath.Ext(path))},
	)
	if retry > 0 {
		labels = append(labels, model.Label{Name: "gauge.retry", Value: fmt.Sprint(retry)})
	}
	tags := collectTags(spec, scenario)
	for _, tag := range tags {
		labels = append(labels, model.Label{Name: "tag", Value: tag})
	}
	parsed := metadata.ParseTags(tags, m.cfg)
	labels = append(labels, parsed.Labels...)
	if m.cfg.BehaviorFromSpec {
		labels = append(labels, model.Label{Name: "feature", Value: first(spec.GetSpecHeading(), filepath.Base(path))})
	}
	status, details := m.scenarioStatus(scenario)
	result := model.TestResult{
		UUID: uuid, Name: first(scenario.GetScenarioHeading(), "Unnamed Gauge scenario"), FullName: canonical,
		TestCaseID: identity.TestCaseID(canonical), Description: scenarioDescription(scenario),
		Status: status, StatusDetails: details, Stage: model.StageFinished,
		Labels: dedupeLabels(labels), Links: parsed.Links, Parameters: parameters,
		Start: live.Range.Start, Stop: live.Range.Stop,
	}
	result.HistoryID = identity.HistoryID(result.TestCaseID, logical)
	warnings := append([]string(nil), parsed.Warnings...)
	groups, attachmentWarnings, err := m.scenarioSteps(ctx, scenario, live.Range, copyAttachments)
	warnings = append(warnings, attachmentWarnings...)
	if err != nil {
		return ResultArtifact{}, warnings, err
	}
	result.Steps = groups
	if source := m.sourceLink(path, scenario.GetSpan()); source.URL != "" {
		result.Links = append(result.Links, source)
	}
	return ResultArtifact{Key: key, Result: result}, warnings, nil
}

func (m *Mapper) scenarioIdentity(spec *gm.ProtoSpec, item *gm.ProtoItem, retry int32) (string, string, int32, int32) {
	scenario, table := scenarioFromItem(item)
	path := identity.CanonicalPath(m.projectRoot, first(spec.GetFileName(), item.GetFileName()))
	span := scenario.GetSpan()
	project := first(m.env["GAUGE_PROJECT_NAME"], m.env["gauge_project_name"], filepath.Base(m.projectRoot))
	canonical := identity.Scenario(project, path, scenario.GetID(), scenario.GetScenarioHeading(), span.GetStart(), span.GetEnd())
	if table == nil {
		return canonical, path, 0, 0
	}
	return canonical, path, table.GetTableRowIndex(), table.GetScenarioTableRowIndex()
}

func (m *Mapper) scenarioStatus(scenario *gm.ProtoScenario) (model.Status, *model.StatusDetails) {
	if scenario.GetExecutionStatus() == gm.ExecutionStatus_SKIPPED || len(scenario.GetSkipErrors()) > 0 {
		return model.StatusSkipped, statusmap.FailureDetails(strings.Join(scenario.GetSkipErrors(), "\n"), "")
	}
	if scenario.GetPreHookFailure() != nil || scenario.GetPostHookFailure() != nil {
		failure := scenario.GetPreHookFailure()
		if failure == nil {
			failure = scenario.GetPostHookFailure()
		}
		return model.StatusBroken, statusmap.FailureDetails(failure.GetErrorMessage(), failure.GetStackTrace())
	}
	if scenario.GetExecutionStatus() == gm.ExecutionStatus_FAILED {
		execution := findStepFailure(scenario)
		message, trace := execution.GetErrorMessage(), execution.GetStackTrace()
		if message == "" {
			message = "Gauge scenario failed without an unambiguous diagnostic"
		}
		return statusmap.FromExecutionStatus(gm.ExecutionStatus_FAILED, "", statusmap.FailureKind(execution)), statusmap.FailureDetails(message, trace)
	}
	return statusmap.FromExecutionStatus(scenario.GetExecutionStatus(), "", statusmap.ErrorNone), nil
}

func (m *Mapper) scenarioSteps(ctx context.Context, scenario *gm.ProtoScenario, parent timing.Range, copyAttachments bool) ([]model.StepResult, []string, error) {
	groups := make([]model.StepResult, 0, 3)
	warnings := make([]string, 0)
	cursor := parent.Start
	sections := []struct {
		name, mode string
		items      []*gm.ProtoItem
	}{
		{"Context", m.cfg.ContextMode, scenario.GetContexts()},
		{"Steps", "grouped-steps", scenario.GetScenarioItems()},
		{"Teardown", m.cfg.TeardownMode, scenario.GetTearDownSteps()},
	}
	for _, section := range sections {
		if section.mode == "fixtures" {
			continue
		}
		steps, next, currentWarnings, err := m.mapItems(ctx, section.items, cursor, parent, 0, copyAttachments)
		warnings = append(warnings, currentWarnings...)
		if err != nil {
			return nil, warnings, err
		}
		cursor = next
		if len(steps) == 0 {
			continue
		}
		if section.mode == "flat" {
			groups = append(groups, steps...)
			continue
		}
		status := aggregateStepStatus(steps)
		group := model.StepResult{Name: section.name, Status: status, Stage: model.StageFinished, Steps: steps, Start: steps[0].Start, Stop: steps[len(steps)-1].Stop}
		groups = append(groups, group)
	}
	return groups, warnings, nil
}

func (m *Mapper) mapItems(ctx context.Context, items []*gm.ProtoItem, cursor int64, parent timing.Range, depth int, copyAttachments bool) ([]model.StepResult, int64, []string, error) {
	if depth > m.cfg.MaximumNestingDepth {
		return []model.StepResult{{Name: "[Gauge] Maximum concept nesting depth exceeded", Status: model.StatusBroken, Stage: model.StageFinished, Start: cursor, Stop: cursor}}, cursor, nil, nil
	}
	result := make([]model.StepResult, 0)
	warnings := make([]string, 0)
	for _, item := range items {
		if item == nil {
			continue
		}
		var step *gm.ProtoStep
		var children []*gm.ProtoItem
		var execution *gm.ProtoStepExecutionResult
		switch item.GetItemType() {
		case gm.ProtoItem_Step:
			step = item.GetStep()
			if step != nil {
				execution = step.GetStepExecutionResult()
			}
		case gm.ProtoItem_Concept:
			concept := item.GetConcept()
			if concept != nil {
				step, children, execution = concept.GetConceptStep(), concept.GetSteps(), concept.GetConceptExecutionResult()
			}
		default:
			continue
		}
		if step == nil {
			continue
		}
		duration := int64(0)
		if execution != nil && execution.GetExecutionResult() != nil {
			duration = execution.GetExecutionResult().GetExecutionTime()
		}
		stepRange := timing.Clamp(timing.Range{Start: cursor, Stop: cursor + max64(duration, 0)}, parent)
		if stepRange.Stop == stepRange.Start && parent.Stop > stepRange.Start {
			stepRange.Stop = min64(parent.Stop, stepRange.Start+1)
		}
		mappedStatus, details := statusmap.FromStep(execution)
		mapped := model.StepResult{Name: first(step.GetActualText(), step.GetParsedText(), "Unnamed Gauge step"), Status: mappedStatus, StatusDetails: details, Stage: model.StageFinished, Parameters: m.stepParameters(step), Start: stepRange.Start, Stop: stepRange.Stop}
		if len(children) > 0 {
			mapped.Steps, _, warnings, _ = m.mapItems(ctx, children, stepRange.Start, stepRange, depth+1, copyAttachments)
			if mapped.Status == model.StatusPassed {
				mapped.Status = aggregateStepStatus(mapped.Steps)
			}
		}
		if copyAttachments {
			currentWarnings, err := m.attachStep(ctx, &mapped, step)
			warnings = append(warnings, currentWarnings...)
			if err != nil {
				return result, cursor, warnings, err
			}
		}
		result = append(result, mapped)
		cursor = max64(cursor, stepRange.Stop)
	}
	return result, cursor, warnings, nil
}

func (m *Mapper) attachStep(ctx context.Context, target *model.StepResult, step *gm.ProtoStep) ([]string, error) {
	if m.attachments == nil {
		return nil, nil
	}
	warnings := make([]string, 0)
	if m.cfg.AttachMessages {
		for _, scope := range []struct {
			name   string
			values []string
		}{{"Before step messages", step.GetPreHookMessages()}, {"After step messages", step.GetPostHookMessages()}} {
			if len(scope.values) == 0 {
				continue
			}
			attachment, err := m.attachments.Messages(ctx, scope.name, scope.values)
			if err != nil {
				if m.cfg.Strict {
					return warnings, err
				}
				warnings = append(warnings, err.Error())
			} else {
				target.Attachments = append(target.Attachments, attachment)
			}
		}
		if execution := step.GetStepExecutionResult().GetExecutionResult(); execution != nil && len(execution.GetMessage()) > 0 {
			attachment, err := m.attachments.Messages(ctx, "Step messages", execution.GetMessage())
			if err != nil {
				if m.cfg.Strict {
					return warnings, err
				}
				warnings = append(warnings, err.Error())
			} else {
				target.Attachments = append(target.Attachments, attachment)
			}
		}
	}
	if m.cfg.AttachScreenshots {
		files := append([]string(nil), step.GetPreHookScreenshotFiles()...)
		files = append(files, step.GetPostHookScreenshotFiles()...)
		legacy := make([][]byte, 0)
		if len(step.GetPreHookScreenshotFiles()) == 0 {
			legacy = append(legacy, step.GetPreHookScreenshots()...)
		}
		if len(step.GetPostHookScreenshotFiles()) == 0 {
			legacy = append(legacy, step.GetPostHookScreenshots()...)
		}
		if execution := step.GetStepExecutionResult().GetExecutionResult(); execution != nil {
			if execution.GetFailureScreenshotFile() != "" {
				files = append(files, execution.GetFailureScreenshotFile())
			} else if len(execution.GetFailureScreenshot()) > 0 {
				legacy = append(legacy, execution.GetFailureScreenshot())
			} else if len(execution.GetScreenShot()) > 0 {
				legacy = append(legacy, execution.GetScreenShot())
			}
			files = append(files, execution.GetScreenshotFiles()...)
			if len(execution.GetScreenshotFiles()) == 0 {
				legacy = append(legacy, execution.GetScreenshots()...)
			}
		}
		for index, file := range files {
			attachment, err := m.attachments.Copy(ctx, screenshotName(index, file), file)
			if err != nil {
				if m.cfg.Strict {
					return warnings, err
				}
				warnings = append(warnings, err.Error())
			} else {
				target.Attachments = append(target.Attachments, attachment)
			}
		}
		for index, data := range legacy {
			attachment, err := m.attachments.Bytes(ctx, fmt.Sprintf("Screenshot %d (legacy)", index+1), "", ".png", data)
			if err != nil {
				if m.cfg.Strict {
					return warnings, err
				}
				warnings = append(warnings, err.Error())
			} else {
				target.Attachments = append(target.Attachments, attachment)
			}
		}
	}
	if m.cfg.AttachTables {
		for _, fragment := range step.GetFragments() {
			parameter := fragment.GetParameter()
			if fragment.GetFragmentType() != gm.Fragment_Parameter || parameter == nil {
				continue
			}
			name := first(parameter.GetName(), "step parameter")
			var mediaType, extension, value string
			switch parameter.GetParameterType() {
			case gm.Parameter_Table, gm.Parameter_Special_Table:
				mediaType, extension, value = "application/json", ".json", tableJSON(parameter.GetTable())
			case gm.Parameter_Multiline_String, gm.Parameter_Special_String:
				mediaType, extension, value = "text/markdown; charset=utf-8", ".md", parameter.GetValue()
			default:
				continue
			}
			attachment, err := m.attachments.Bytes(ctx, "Parameter: "+name, mediaType, extension, []byte(value))
			if err != nil {
				if m.cfg.Strict {
					return warnings, err
				}
				warnings = append(warnings, err.Error())
			} else {
				target.Attachments = append(target.Attachments, attachment)
			}
		}
	}
	return warnings, nil
}

func (m *Mapper) testParameters(table *gm.ProtoTableDrivenScenario, specTable *gm.ProtoTable) ([]model.Parameter, [][2]string) {
	if table == nil {
		return nil, nil
	}
	var headers, cells []string
	if table.GetIsScenarioTableDriven() {
		data := table.GetScenarioDataTable()
		row := table.GetScenarioTableRow()
		if data != nil && data.GetHeaders() != nil {
			headers = data.GetHeaders().GetCells()
		}
		if row != nil && len(row.GetRows()) > 0 {
			cells = row.GetRows()[0].GetCells()
		}
		if len(cells) == 0 && data != nil && int(table.GetScenarioTableRowIndex()) < len(data.GetRows()) {
			cells = data.GetRows()[table.GetScenarioTableRowIndex()].GetCells()
		}
	} else if specTable != nil {
		if specTable.GetHeaders() != nil {
			headers = specTable.GetHeaders().GetCells()
		}
		index := int(table.GetTableRowIndex())
		if index >= 0 && index < len(specTable.GetRows()) {
			cells = specTable.GetRows()[index].GetCells()
		}
	}
	parameters := make([]model.Parameter, 0, len(cells))
	logical := make([][2]string, 0, len(cells))
	names := map[string]int{}
	for index, value := range cells {
		name := fmt.Sprintf("column_%d", index+1)
		if index < len(headers) && strings.TrimSpace(headers[index]) != "" {
			name = headers[index]
		}
		names[name]++
		if names[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, names[name])
		}
		parameter, identityValue := m.privateParameter(name, value)
		parameters = append(parameters, parameter)
		if !parameter.Excluded {
			logical = append(logical, [2]string{name, identityValue})
		}
	}
	return parameters, logical
}

func (m *Mapper) stepParameters(step *gm.ProtoStep) []model.Parameter {
	parameters := make([]model.Parameter, 0)
	names := map[string]int{}
	for _, fragment := range step.GetFragments() {
		parameter := fragment.GetParameter()
		if fragment.GetFragmentType() != gm.Fragment_Parameter || parameter == nil {
			continue
		}
		name := strings.TrimSpace(parameter.GetName())
		if name == "" {
			name = fmt.Sprintf("arg_%d", len(parameters)+1)
		}
		names[name]++
		if names[name] > 1 {
			name = fmt.Sprintf("%s_%d", name, names[name])
		}
		value := parameter.GetValue()
		if parameter.GetTable() != nil {
			value = tableJSON(parameter.GetTable())
		}
		mapped, _ := m.privateParameter(name, value)
		parameters = append(parameters, mapped)
	}
	return parameters
}

func (m *Mapper) promotedParameters(scenario *gm.ProtoScenario) ([]model.Parameter, [][2]string) {
	if scenario == nil || len(m.cfg.PromoteParameters) == 0 {
		return nil, nil
	}
	parameters := make([]model.Parameter, 0)
	logical := make([][2]string, 0)
	names := map[string]int{}
	var visit func([]*gm.ProtoItem)
	visit = func(items []*gm.ProtoItem) {
		for _, item := range items {
			if item == nil {
				continue
			}
			if concept := item.GetConcept(); concept != nil {
				visit(concept.GetSteps())
			}
			step := item.GetStep()
			if step == nil && item.GetConcept() != nil {
				step = item.GetConcept().GetConceptStep()
			}
			if step == nil {
				continue
			}
			for _, fragment := range step.GetFragments() {
				value := fragment.GetParameter()
				if fragment.GetFragmentType() != gm.Fragment_Parameter || value == nil || !matches(value.GetName(), m.cfg.PromoteParameters) {
					continue
				}
				name := first(value.GetName(), fmt.Sprintf("arg_%d", len(parameters)+1))
				names[name]++
				if names[name] > 1 {
					name = fmt.Sprintf("%s_%d", name, names[name])
				}
				raw := value.GetValue()
				if value.GetTable() != nil {
					raw = tableJSON(value.GetTable())
				}
				parameter, identityValue := m.privateParameter(name, raw)
				parameters = append(parameters, parameter)
				if !parameter.Excluded {
					logical = append(logical, [2]string{name, identityValue})
				}
			}
		}
	}
	items := append([]*gm.ProtoItem(nil), scenario.GetContexts()...)
	items = append(items, scenario.GetScenarioItems()...)
	items = append(items, scenario.GetTearDownSteps()...)
	visit(items)
	return parameters, logical
}

func (m *Mapper) privateParameter(name, value string) (model.Parameter, string) {
	parameter := model.Parameter{Name: name, Value: value}
	identityValue := value
	if matches(name, m.cfg.MaskParameters) {
		parameter.Mode, parameter.Value = model.ParameterModeMasked, "[MASKED]"
		identityValue = privateDigest(value)
	}
	if matches(name, m.cfg.HideParameters) {
		parameter.Mode, parameter.Value = model.ParameterModeHidden, "[HIDDEN]"
		identityValue = privateDigest(value)
	}
	if matches(name, m.cfg.ExcludeHistoryParameters) {
		parameter.Excluded = true
	}
	return parameter, identityValue
}

func (m *Mapper) scenarioContainer(ctx context.Context, scenario *gm.ProtoScenario, child string, value timing.Range) (model.TestResultContainer, []string, error) {
	container, warnings, err := m.container(ctx, "Scenario hooks: "+scenario.GetScenarioHeading(), []string{child}, value,
		scenario.GetPreHookFailure(), scenario.GetPostHookFailure(), scenario.GetPreHookMessages(), scenario.GetPostHookMessages(), scenario.GetPreHookScreenshotFiles(), scenario.GetPostHookScreenshotFiles(), scenario.GetPreHookScreenshots(), scenario.GetPostHookScreenshots())
	if err != nil {
		return container, warnings, err
	}
	if m.cfg.ContextMode == "fixtures" {
		steps, _, current, mapErr := m.mapItems(ctx, scenario.GetContexts(), value.Start, value, 0, true)
		warnings = append(warnings, current...)
		if mapErr != nil {
			return container, warnings, mapErr
		}
		container.Befores[0].Steps = append(container.Befores[0].Steps, steps...)
	}
	if m.cfg.TeardownMode == "fixtures" {
		steps, _, current, mapErr := m.mapItems(ctx, scenario.GetTearDownSteps(), value.Start, value, 0, true)
		warnings = append(warnings, current...)
		if mapErr != nil {
			return container, warnings, mapErr
		}
		container.Afters[0].Steps = append(container.Afters[0].Steps, steps...)
	}
	return container, warnings, nil
}

func (m *Mapper) specContainer(ctx context.Context, spec *gm.ProtoSpec, children []string, value timing.Range) (model.TestResultContainer, []string, error) {
	if len(children) == 0 {
		return model.TestResultContainer{}, nil, nil
	}
	var before, after *gm.ProtoHookFailure
	if len(spec.GetPreHookFailures()) > 0 {
		before = spec.GetPreHookFailures()[0]
	}
	if len(spec.GetPostHookFailures()) > 0 {
		after = spec.GetPostHookFailures()[0]
	}
	return m.container(ctx, "Specification hooks: "+first(spec.GetSpecHeading(), filepath.Base(spec.GetFileName())), children, value,
		before, after, spec.GetPreHookMessages(), spec.GetPostHookMessages(), spec.GetPreHookScreenshotFiles(), spec.GetPostHookScreenshotFiles(), spec.GetPreHookScreenshots(), spec.GetPostHookScreenshots())
}

func (m *Mapper) suiteContainer(ctx context.Context, suite *gm.ProtoSuiteResult, children []string, value timing.Range) (model.TestResultContainer, []string, error) {
	return m.container(ctx, "Suite hooks: "+first(suite.GetProjectName(), "Gauge project"), children, value,
		suite.GetPreHookFailure(), suite.GetPostHookFailure(), suite.GetPreHookMessages(), suite.GetPostHookMessages(), suite.GetPreHookScreenshotFiles(), suite.GetPostHookScreenshotFiles(), suite.GetPreHookScreenshots(), suite.GetPostHookScreenshots())
}

func (m *Mapper) container(ctx context.Context, name string, children []string, value timing.Range, beforeFailure, afterFailure *gm.ProtoHookFailure, beforeMessages, afterMessages, beforeScreens, afterScreens []string, beforeLegacy, afterLegacy [][]byte) (model.TestResultContainer, []string, error) {
	if len(children) == 0 {
		return model.TestResultContainer{}, nil, nil
	}
	uuid, err := m.uuids.New()
	if err != nil {
		return model.TestResultContainer{}, nil, err
	}
	container := model.TestResultContainer{UUID: uuid, Name: name, Children: append([]string(nil), children...), Start: value.Start, Stop: value.Stop}
	warnings := make([]string, 0)
	before, currentWarnings, err := m.fixture(ctx, "Before "+strings.TrimPrefix(name, strings.Split(name, ":")[0]+":"), beforeFailure, beforeMessages, beforeScreens, beforeLegacy, value.Start)
	warnings = append(warnings, currentWarnings...)
	if err != nil {
		return container, warnings, err
	}
	after, currentWarnings, err := m.fixture(ctx, "After "+strings.TrimPrefix(name, strings.Split(name, ":")[0]+":"), afterFailure, afterMessages, afterScreens, afterLegacy, value.Stop)
	warnings = append(warnings, currentWarnings...)
	if err != nil {
		return container, warnings, err
	}
	container.Befores, container.Afters = []model.FixtureResult{before}, []model.FixtureResult{after}
	return container, warnings, nil
}

func (m *Mapper) fixture(ctx context.Context, name string, failure *gm.ProtoHookFailure, messages, screenshots []string, legacy [][]byte, at int64) (model.FixtureResult, []string, error) {
	fixture := model.FixtureResult{Name: strings.TrimSpace(name), Status: model.StatusPassed, Stage: model.StageFinished, Start: at, Stop: at}
	if failure != nil {
		fixture.Status = model.StatusBroken
		fixture.StatusDetails = statusmap.FailureDetails(failure.GetErrorMessage(), failure.GetStackTrace())
		if failure.GetFailureScreenshotFile() != "" {
			screenshots = append(screenshots, failure.GetFailureScreenshotFile())
		} else if len(failure.GetFailureScreenshot()) > 0 {
			legacy = append(legacy, failure.GetFailureScreenshot())
		} else if len(failure.GetScreenShot()) > 0 {
			legacy = append(legacy, failure.GetScreenShot())
		}
	}
	warnings := make([]string, 0)
	if m.attachments != nil && m.cfg.AttachMessages && len(messages) > 0 {
		attachment, err := m.attachments.Messages(ctx, fixture.Name+" messages", messages)
		if err != nil {
			if m.cfg.Strict {
				return fixture, warnings, err
			}
			warnings = append(warnings, err.Error())
		} else {
			fixture.Attachments = append(fixture.Attachments, attachment)
		}
	}
	if m.attachments != nil && m.cfg.AttachScreenshots {
		for index, source := range screenshots {
			attachment, err := m.attachments.Copy(ctx, screenshotName(index, source), source)
			if err != nil {
				if m.cfg.Strict {
					return fixture, warnings, err
				}
				warnings = append(warnings, err.Error())
			} else {
				fixture.Attachments = append(fixture.Attachments, attachment)
			}
		}
		for index, data := range legacy {
			attachment, err := m.attachments.Bytes(ctx, fmt.Sprintf("%s screenshot %d (legacy)", fixture.Name, index+1), "", ".png", data)
			if err != nil {
				if m.cfg.Strict {
					return fixture, warnings, err
				}
				warnings = append(warnings, err.Error())
			} else {
				fixture.Attachments = append(fixture.Attachments, attachment)
			}
		}
	}
	return fixture, warnings, nil
}

func (m *Mapper) synthetic(name, key, message string, status model.Status, start time.Time) (ResultArtifact, error) {
	uuid, err := m.uuids.New()
	if err != nil {
		return ResultArtifact{}, err
	}
	canonical := filepath.Base(m.projectRoot) + "::[synthetic]::" + key
	result := model.TestResult{UUID: uuid, Name: name, FullName: canonical, TestCaseID: identity.TestCaseID(canonical), Status: status, Stage: model.StageFinished, Start: start.UnixMilli(), Stop: start.UnixMilli(), Labels: []model.Label{{Name: "framework", Value: "Gauge"}, {Name: "parentSuite", Value: filepath.Base(m.projectRoot)}}}
	result.HistoryID = identity.HistoryID(result.TestCaseID, nil)
	result.StatusDetails = statusmap.FailureDetails(message, "")
	return ResultArtifact{Key: key, Result: result}, nil
}

func (m *Mapper) sourceLink(path string, span *gm.Span) model.Link {
	if m.cfg.SourceLinkTemplate == "" || span == nil {
		return model.Link{}
	}
	if strings.Contains(path, "..") || filepath.IsAbs(path) {
		return model.Link{}
	}
	address := strings.ReplaceAll(m.cfg.SourceLinkTemplate, "{path}", strings.ReplaceAll(url.PathEscape(path), "%2F", "/"))
	address = strings.ReplaceAll(address, "{line}", fmt.Sprint(span.GetStart()))
	parsed, err := url.Parse(address)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Host == "" {
		return model.Link{}
	}
	return model.Link{Name: "Gauge specification source", Type: "link", URL: address}
}

func scenarioFromItem(item *gm.ProtoItem) (*gm.ProtoScenario, *gm.ProtoTableDrivenScenario) {
	if item == nil {
		return nil, nil
	}
	if item.GetItemType() == gm.ProtoItem_TableDrivenScenario {
		table := item.GetTableDrivenScenario()
		if table != nil {
			return table.GetScenario(), table
		}
	}
	return item.GetScenario(), nil
}

func isScenario(item *gm.ProtoItem) bool {
	return item != nil && (item.GetItemType() == gm.ProtoItem_Scenario || item.GetItemType() == gm.ProtoItem_TableDrivenScenario)
}

func collectTags(spec *gm.ProtoSpec, scenario *gm.ProtoScenario) []string {
	values := append([]string(nil), spec.GetTags()...)
	values = append(values, scenario.GetTags()...)
	for _, item := range scenario.GetScenarioItems() {
		if item != nil && item.GetTags() != nil {
			values = append(values, item.GetTags().GetTags()...)
		}
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func scenarioDescription(scenario *gm.ProtoScenario) string {
	comments := make([]string, 0)
	for _, item := range scenario.GetScenarioItems() {
		if item != nil && item.GetComment() != nil && strings.TrimSpace(item.GetComment().GetText()) != "" {
			comments = append(comments, strings.TrimSpace(item.GetComment().GetText()))
		}
	}
	return strings.Join(comments, "\n\n")
}

func findStepFailure(scenario *gm.ProtoScenario) *gm.ProtoExecutionResult {
	var inspect func([]*gm.ProtoItem) *gm.ProtoExecutionResult
	inspect = func(items []*gm.ProtoItem) *gm.ProtoExecutionResult {
		for _, item := range items {
			if item == nil {
				continue
			}
			if step := item.GetStep(); step != nil && step.GetStepExecutionResult() != nil && step.GetStepExecutionResult().GetExecutionResult() != nil {
				value := step.GetStepExecutionResult().GetExecutionResult()
				if value.GetFailed() {
					return value
				}
			}
			if concept := item.GetConcept(); concept != nil {
				if value := inspect(concept.GetSteps()); value != nil {
					return value
				}
			}
		}
		return nil
	}
	items := append([]*gm.ProtoItem(nil), scenario.GetContexts()...)
	items = append(items, scenario.GetScenarioItems()...)
	items = append(items, scenario.GetTearDownSteps()...)
	return inspect(items)
}

func aggregateStepStatus(steps []model.StepResult) model.Status {
	value := model.StatusPassed
	for _, step := range steps {
		if step.Status == model.StatusBroken {
			return model.StatusBroken
		}
		if step.Status == model.StatusFailed {
			value = model.StatusFailed
		}
		if step.Status == model.StatusSkipped && value == model.StatusPassed {
			value = model.StatusSkipped
		}
	}
	return value
}

func dedupeLabels(values []model.Label) []model.Label {
	seen := map[string]bool{}
	result := make([]model.Label, 0, len(values))
	for _, value := range values {
		key := value.Name + "\x00" + value.Value
		if value.Name != "" && value.Value != "" && !seen[key] {
			seen[key] = true
			result = append(result, value)
		}
	}
	return result
}

func tableJSON(table *gm.ProtoTable) string {
	if table == nil {
		return ""
	}
	rows := make([][]string, 0, len(table.GetRows())+1)
	if table.GetHeaders() != nil {
		rows = append(rows, table.GetHeaders().GetCells())
	}
	for _, row := range table.GetRows() {
		if row != nil {
			rows = append(rows, row.GetCells())
		}
	}
	data, _ := json.Marshal(rows)
	return string(data)
}

func findSpecTable(spec *gm.ProtoSpec) *gm.ProtoTable {
	if spec == nil || !spec.GetIsTableDriven() {
		return nil
	}
	for _, item := range spec.GetItems() {
		if item != nil && item.GetItemType() == gm.ProtoItem_Table && item.GetTable() != nil {
			return item.GetTable()
		}
	}
	return nil
}

func matches(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.EqualFold(pattern, name) {
			return true
		}
		if matched, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(name)); err == nil && matched {
			return true
		}
	}
	return false
}

func privateDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
func screenshotName(index int, source string) string {
	if index == 0 && strings.Contains(strings.ToLower(filepath.Base(source)), "fail") {
		return "Failure screenshot"
	}
	return fmt.Sprintf("Screenshot %d", index+1)
}
func projectFromCanonical(value string) string {
	if project, _, ok := strings.Cut(value, "::"); ok {
		return project
	}
	return "Gauge project"
}
func parseTime(value string, fallback int64) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	return time.UnixMilli(fallback)
}
func first(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Gauge stores the number of attempts in ProtoScenario.retriesCount while
// lifecycle events expose a zero-based currentRetry index.
func retryIndex(attempts int64) int32 {
	if attempts <= 1 {
		return 0
	}
	if attempts > int64(^uint32(0)>>1)+1 {
		return int32(^uint32(0) >> 1)
	}
	return int32(attempts - 1)
}
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
