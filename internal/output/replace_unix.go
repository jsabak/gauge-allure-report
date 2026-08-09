// SPDX-License-Identifier: Apache-2.0
//go:build !windows

package output

import "os"

func atomicReplace(source, target string) error { return os.Rename(source, target) }
