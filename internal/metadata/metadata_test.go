// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"

	"github.com/jsabak/gauge-allure-report/internal/config"
)

func TestParseTags(t *testing.T) {
	cfg := config.Defaults()
	cfg.IssueLinkPattern = "https://issues.example/{}"
	cfg.TMSLinkPattern = "https://tms.example/case/%s"
	parsed := ParseTags([]string{"allure.label.owner:alice", "allure.label.story:refund\\: partial", "allure.link.issue:PAY-1", "allure.link.tms:TC 2", "allure.link.link:javascript:alert(1)", "allure.id:42"}, cfg)
	if len(parsed.Labels) != 3 {
		t.Fatalf("labels: %+v", parsed.Labels)
	}
	if len(parsed.Links) != 2 {
		t.Fatalf("links: %+v", parsed.Links)
	}
	if len(parsed.Warnings) != 1 {
		t.Fatalf("warnings: %+v", parsed.Warnings)
	}
	if parsed.Links[0].URL != "https://issues.example/PAY-1" {
		t.Fatalf("issue URL: %s", parsed.Links[0].URL)
	}
}

func TestExecutorProviders(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"jenkins", map[string]string{"JENKINS_URL": "https://jenkins.example", "BUILD_URL": "https://jenkins.example/job/1", "JOB_NAME": "job"}, "jenkins"},
		{"github", map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_SERVER_URL": "https://github.com", "GITHUB_REPOSITORY": "o/r", "GITHUB_RUN_ID": "1"}, "github"},
		{"gitlab", map[string]string{"GITLAB_CI": "true", "CI_JOB_URL": "https://gitlab.example/job/1"}, "gitlab"},
		{"azure", map[string]string{"TF_BUILD": "True", "BUILD_BUILDID": "7"}, "azure"},
		{"teamcity", map[string]string{"TEAMCITY_VERSION": "2026.1"}, "teamcity"},
		{"circle", map[string]string{"CIRCLECI": "true"}, "circleci"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := Executor(test.env, config.ExecutorOverrides{})
			if !ok || got.Type != test.want {
				t.Fatalf("got %+v, %v", got, ok)
			}
		})
	}
}

func TestEnvironmentAllowlistRejectsSecretKeys(t *testing.T) {
	cfg := config.Defaults()
	cfg.EnvironmentAllowlist = []string{"REGION", "API_TOKEN"}
	cfg.Redact = []string{"internal"}
	values := Environment(map[string]string{"REGION": "internal-west", "API_TOKEN": "secret", "test_language": "python"}, cfg, "demo", "ci", 2)
	if values["REGION"] != "[REDACTED]-west" || values["API_TOKEN"] != "" {
		t.Fatalf("values: %+v", values)
	}
}

func FuzzParseTags(f *testing.F) {
	f.Add("allure.label.owner:alice")
	f.Add("allure.link.link:javascript:alert(1)")
	f.Fuzz(func(t *testing.T, tag string) { _ = ParseTags([]string{tag}, config.Defaults()) })
}
