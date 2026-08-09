// SPDX-License-Identifier: Apache-2.0

// Package version exposes reproducibly injected build metadata.
package version

import "fmt"

const packageMarkerPrefix = "gauge-allure-report-package-version:"

var (
	Version       = "0.1.0-dev"
	Commit        = "unknown"
	BuildDate     = "unknown"
	Dirty         = "unknown"
	PackageMarker = "gauge-allure-report-package-version:0.1.0-dev"
)

// String returns a concise operator-facing version string.
func String() string {
	value := fmt.Sprintf("allure-report %s (commit=%s, built=%s, dirty=%s)", Version, Commit, BuildDate, Dirty)
	if PackageMarker != packageMarkerPrefix+Version {
		return value + " [package marker mismatch]"
	}
	return value
}
