// SPDX-License-Identifier: Apache-2.0

package version

import (
	"strings"
	"testing"
)

func TestStringDetectsMarkerMismatch(t *testing.T) {
	originalVersion, originalMarker := Version, PackageMarker
	t.Cleanup(func() { Version, PackageMarker = originalVersion, originalMarker })
	Version = "1.2.3"
	PackageMarker = packageMarkerPrefix + Version
	if strings.Contains(String(), "mismatch") {
		t.Fatal("matching marker reported an error")
	}
	PackageMarker = packageMarkerPrefix + "other"
	if !strings.Contains(String(), "package marker mismatch") {
		t.Fatal("marker mismatch was not exposed")
	}
}
