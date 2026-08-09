// SPDX-License-Identifier: Apache-2.0

// Package metadata maps Gauge tags, environment, and CI contracts to Allure metadata.
package metadata

import (
	"fmt"
	"net/url"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/allure-framework/allure-go/commons/model"
	"github.com/jsabak/gauge-allure-report/internal/config"
	"github.com/jsabak/gauge-allure-report/internal/version"
)

// ParsedTags contains additive metadata; callers must still preserve every original tag.
type ParsedTags struct {
	Labels   []model.Label
	Links    []model.Link
	Warnings []string
}

// ParseTags recognizes the documented allure.label.*, allure.link.*, and allure.id syntax.
func ParseTags(tags []string, cfg config.Config) ParsedTags {
	result := ParsedTags{}
	seenLabels, seenLinks := map[string]bool{}, map[string]bool{}
	for _, tag := range tags {
		key, value, ok := splitTag(tag)
		if !ok {
			continue
		}
		switch {
		case key == "allure.id":
			addLabel(&result.Labels, seenLabels, "ALLURE_ID", value)
		case strings.HasPrefix(key, "allure.label."):
			name := strings.TrimPrefix(key, "allure.label.")
			if !validLabelName(name) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("ignored invalid Allure label name %q", name))
				continue
			}
			addLabel(&result.Labels, seenLabels, name, value)
		case strings.HasPrefix(key, "allure.link."):
			linkType := strings.TrimPrefix(key, "allure.link.")
			pattern := cfg.GenericLinkPattern
			if linkType == "issue" {
				pattern = cfg.IssueLinkPattern
			} else if linkType == "tms" {
				pattern = cfg.TMSLinkPattern
			} else if linkType != "link" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("ignored unsupported Allure link type %q", linkType))
				continue
			}
			address := expandLink(pattern, value)
			if address == "" && linkType == "link" {
				address = value
			}
			if !safeURL(address) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("ignored unsafe or incomplete %s link", linkType))
				continue
			}
			identity := linkType + "\x00" + address + "\x00" + value
			if !seenLinks[identity] {
				seenLinks[identity] = true
				result.Links = append(result.Links, model.Link{Type: linkType, Name: value, URL: address})
			}
		}
	}
	return result
}

// Executor detects supported CI providers from official environment contracts.
func Executor(env map[string]string, overrides config.ExecutorOverrides) (model.Executor, bool) {
	var result model.Executor
	switch {
	case env["JENKINS_URL"] != "" || env["BUILD_URL"] != "" && env["JENKINS_HOME"] != "":
		result = model.Executor{Name: "Jenkins", Type: "jenkins", BuildName: first(env["JOB_NAME"], env["BUILD_TAG"]), BuildURL: env["BUILD_URL"]}
		result.BuildOrder, _ = strconv.ParseInt(env["BUILD_NUMBER"], 10, 64)
	case env["GITHUB_ACTIONS"] == "true":
		result = model.Executor{Name: "GitHub Actions", Type: "github", BuildName: first(env["GITHUB_WORKFLOW"], env["GITHUB_JOB"])}
		result.BuildOrder, _ = strconv.ParseInt(env["GITHUB_RUN_NUMBER"], 10, 64)
		if env["GITHUB_SERVER_URL"] != "" && env["GITHUB_REPOSITORY"] != "" && env["GITHUB_RUN_ID"] != "" {
			result.BuildURL = strings.TrimSuffix(env["GITHUB_SERVER_URL"], "/") + "/" + env["GITHUB_REPOSITORY"] + "/actions/runs/" + env["GITHUB_RUN_ID"]
		}
	case env["GITLAB_CI"] == "true":
		result = model.Executor{Name: "GitLab CI", Type: "gitlab", BuildName: first(env["CI_JOB_NAME"], env["CI_PIPELINE_NAME"]), BuildURL: first(env["CI_JOB_URL"], env["CI_PIPELINE_URL"])}
		result.BuildOrder, _ = strconv.ParseInt(env["CI_PIPELINE_IID"], 10, 64)
	case env["TF_BUILD"] == "True" || env["TF_BUILD"] == "true":
		result = model.Executor{Name: "Azure Pipelines", Type: "azure", BuildName: env["BUILD_BUILDNUMBER"]}
		result.BuildOrder, _ = strconv.ParseInt(env["BUILD_BUILDID"], 10, 64)
		if env["SYSTEM_TEAMFOUNDATIONCOLLECTIONURI"] != "" && env["SYSTEM_TEAMPROJECT"] != "" && env["BUILD_BUILDID"] != "" {
			result.BuildURL = strings.TrimSuffix(env["SYSTEM_TEAMFOUNDATIONCOLLECTIONURI"], "/") + "/" + url.PathEscape(env["SYSTEM_TEAMPROJECT"]) + "/_build/results?buildId=" + url.QueryEscape(env["BUILD_BUILDID"])
		}
	case env["TEAMCITY_VERSION"] != "":
		result = model.Executor{Name: "TeamCity", Type: "teamcity", BuildName: env["TEAMCITY_BUILDCONF_NAME"], BuildURL: env["BUILD_URL"]}
		result.BuildOrder, _ = strconv.ParseInt(env["BUILD_NUMBER"], 10, 64)
	case env["CIRCLECI"] == "true":
		result = model.Executor{Name: "CircleCI", Type: "circleci", BuildName: env["CIRCLE_JOB"], BuildURL: env["CIRCLE_BUILD_URL"]}
		result.BuildOrder, _ = strconv.ParseInt(env["CIRCLE_BUILD_NUM"], 10, 64)
	case env["CI"] != "":
		result = model.Executor{Name: "CI", Type: "generic", BuildName: first(env["CI_JOB_NAME"], env["BUILD_NAME"]), BuildURL: first(env["CI_JOB_URL"], env["BUILD_URL"])}
	default:
		if overrides.Name == "" && overrides.Type == "" {
			return model.Executor{}, false
		}
	}
	applyExecutorOverrides(&result, overrides)
	result.BuildURL = keepSafeURL(result.BuildURL)
	result.ReportURL = keepSafeURL(result.ReportURL)
	return result, true
}

// Environment returns a strict, sorted-at-write-time metadata allowlist.
func Environment(env map[string]string, cfg config.Config, project, gaugeEnvironment string, streams int32) map[string]string {
	values := map[string]string{
		"allure-report.version": version.Version,
		"os":                    runtime.GOOS + "/" + runtime.GOARCH,
	}
	if project != "" {
		values["gauge.project"] = project
	}
	if gaugeEnvironment != "" {
		values["gauge.environment"] = gaugeEnvironment
	}
	if language := env["test_language"]; language != "" {
		values["gauge.language"] = language
	}
	if gaugeVersion := env["GAUGE_VERSION"]; gaugeVersion != "" {
		values["gauge.version"] = gaugeVersion
	}
	if streams > 0 {
		values["gauge.parallel_streams"] = strconv.Itoa(int(streams))
	}
	allow := append([]string(nil), cfg.EnvironmentAllowlist...)
	sort.Strings(allow)
	for _, key := range allow {
		if safeEnvironmentKey(key) {
			if value, ok := env[key]; ok {
				values[key] = redact(value, cfg.Redact)
			}
		}
	}
	return values
}

func splitTag(tag string) (string, string, bool) {
	escaped := false
	for index, r := range tag {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ':' {
			key := strings.TrimSpace(unescape(tag[:index]))
			value := strings.TrimSpace(unescape(tag[index+1:]))
			return key, value, key != "" && value != ""
		}
	}
	return "", "", false
}

func unescape(value string) string {
	var builder strings.Builder
	escaped := false
	for _, r := range value {
		if escaped {
			builder.WriteRune(r)
			escaped = false
		} else if r == '\\' {
			escaped = true
		} else {
			builder.WriteRune(r)
		}
	}
	if escaped {
		builder.WriteByte('\\')
	}
	return builder.String()
}

func validLabelName(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r == '_' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func addLabel(labels *[]model.Label, seen map[string]bool, name, value string) {
	key := name + "\x00" + value
	if !seen[key] {
		seen[key] = true
		*labels = append(*labels, model.Label{Name: name, Value: value})
	}
}

func expandLink(pattern, value string) string {
	if pattern == "" {
		return ""
	}
	if strings.Contains(pattern, "{}") {
		return strings.ReplaceAll(pattern, "{}", url.PathEscape(value))
	}
	if strings.Contains(pattern, "%s") {
		return strings.ReplaceAll(pattern, "%s", url.PathEscape(value))
	}
	return strings.TrimSuffix(pattern, "/") + "/" + url.PathEscape(value)
}

func safeURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func keepSafeURL(value string) string {
	if safeURL(value) {
		return value
	}
	return ""
}

func safeEnvironmentKey(value string) bool {
	lower := strings.ToLower(value)
	for _, fragment := range []string{"token", "secret", "password", "credential", "private", "key"} {
		if strings.Contains(lower, fragment) {
			return false
		}
	}
	return value != "" && len(value) <= 128
}

func redact(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	return value
}

func applyExecutorOverrides(result *model.Executor, overrides config.ExecutorOverrides) {
	if overrides.Name != "" {
		result.Name = overrides.Name
	}
	if overrides.Type != "" {
		result.Type = overrides.Type
	}
	if overrides.BuildName != "" {
		result.BuildName = overrides.BuildName
	}
	if overrides.BuildURL != "" {
		result.BuildURL = overrides.BuildURL
	}
	if overrides.ReportName != "" {
		result.ReportName = overrides.ReportName
	}
	if overrides.ReportURL != "" {
		result.ReportURL = overrides.ReportURL
	}
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
