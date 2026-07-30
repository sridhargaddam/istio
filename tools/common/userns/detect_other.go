// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !linux

package userns

import "fmt"

// BuildProcFdPath builds a /proc path for the given file descriptor.
func BuildProcFdPath(fd int) string {
	return fmt.Sprintf("/proc/%d/fd/%d", 0, fd)
}

// GetNsID is not supported on non-Linux platforms.
func GetNsID(_ string) (int, error) {
	return 0, fmt.Errorf("user namespace detection is not supported on this platform")
}

// GetParentUserNsByNsPath is not supported on non-Linux platforms.
func GetParentUserNsByNsPath(_ string) (int, error) {
	return 0, fmt.Errorf("user namespace detection is not supported on this platform")
}

// DetectUserNamespace is not supported on non-Linux platforms.
func DetectUserNamespace(_ string) (int, bool, error) {
	return 0, false, nil
}
