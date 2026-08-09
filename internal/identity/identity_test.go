// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"regexp"
	"testing"
)

func TestRandomUUIDIsRFC4122Version4(t *testing.T) {
	value, err := (RandomUUID{}).New()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(value) {
		t.Fatalf("not an RFC 4122 version 4 UUID: %q", value)
	}
}

func TestCanonicalPath(t *testing.T) {
	tests := []struct{ name, root, path, want string }{
		{"windows absolute", `C:\work\demo`, `C:\work\demo\specs\refunds.spec`, "specs/refunds.spec"},
		{"unix absolute", "/work/demo", "/work/demo/specs/refunds.spec", "specs/refunds.spec"},
		{"relative", "/work/demo", `specs\Unicode ą.spec`, "specs/Unicode ą.spec"},
		{"traversal collapsed", "/work/demo", "../outside.spec", "outside.spec"},
		{"root itself", "/work/demo", "/work/demo", "."},
		{"absolute outside root", "/work/demo", "/tmp/outside.spec", "outside.spec"},
		{"empty", "/work/demo", " ./ ", "unknown.spec"},
		{"inner traversal", "/work/demo", "specs/nested/../refund.spec", "specs/refund.spec"},
		{"empty root", "", "./specs/a.spec", "specs/a.spec"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanonicalPath(test.root, test.path); got != test.want {
				t.Fatalf("CanonicalPath() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestScenarioFallbackIdentity(t *testing.T) {
	got := Scenario("  ", "specs/a.spec", "", "  repeated   spaces ", 12, 14)
	if got != "Gauge project::specs/a.spec::L12-L14:repeated spaces" {
		t.Fatalf("fallback identity = %q", got)
	}
}

func TestIdentityGoldenVectors(t *testing.T) {
	canonical := Scenario("payments", "specs/refunds.spec", "scenario-42", "Refund", 10, 20)
	if canonical != "payments::specs/refunds.spec::scenario-42" {
		t.Fatalf("canonical = %q", canonical)
	}
	if got, want := TestCaseID(canonical), "9452d5c66dd200aefaa5ebbedc9556005293002ef385265a733cc71d41b9329e"; got != want {
		t.Fatalf("testCaseId = %q, want %q", got, want)
	}
	parameters := [][2]string{{"currency", "PLN"}, {"amount", "100"}}
	if got, want := HistoryID(TestCaseID(canonical), parameters), "551bcab47bc12bbebc433420e9822c175f8cd89e222bf2de88e77ca4012f4bf9"; got != want {
		t.Fatalf("historyId = %q, want %q", got, want)
	}
	if HistoryID(TestCaseID(canonical), parameters) == HistoryID(TestCaseID(canonical), [][2]string{{"amount", "100"}, {"currency", "PLN"}}) {
		t.Fatal("parameter order must affect history")
	}
	if AttemptKey(canonical, 1, 2, 3) == AttemptKey(canonical, 1, 2, 4) {
		t.Fatal("retry attempts need distinct keys")
	}
}

func FuzzCanonicalPath(f *testing.F) {
	f.Add(`C:\project`, `C:\project\specs\a.spec`)
	f.Add("/project", "../../etc/passwd")
	f.Fuzz(func(t *testing.T, root, value string) {
		result := CanonicalPath(root, value)
		if result == "" {
			t.Fatal("canonical path is empty")
		}
		if len(result) > 1 && result[0] == '/' {
			t.Fatalf("absolute canonical path: %q", result)
		}
	})
}
