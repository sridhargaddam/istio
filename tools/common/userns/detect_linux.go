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

package userns

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// BuildProcFdPath builds a /proc path for the given file descriptor.
func BuildProcFdPath(fd int) string {
	return fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), fd)
}

// GetNsID retrieves the kernel namespace ID (inode number) for the given namespace path.
func GetNsID(nsPath string) (int, error) {
	fd, err := unix.Open(nsPath, unix.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer unix.Close(fd)

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return 0, err
	}

	return int(stat.Ino), nil
}

// GetParentUserNsByNsPath retrieves the parent user namespace fd for the given namespace path.
// The caller is responsible for closing the returned fd.
func GetParentUserNsByNsPath(nsPath string) (int, error) {
	fd, err := unix.Open(nsPath, unix.O_RDONLY, 0)
	if err != nil {
		return 0, err
	}
	defer unix.Close(fd)

	nsFd, err := unix.IoctlRetInt(fd, unix.NS_GET_USERNS)
	if err != nil {
		return 0, err
	}

	return nsFd, nil
}

// DetectUserNamespace checks whether the given network namespace path lives inside a
// non-root Kubernetes user namespace (hostUsers: false). It walks the namespace hierarchy
// via NS_GET_USERNS ioctl and compares parent vs grandparent user namespace IDs.
//
// Returns:
//   - parentUserNsFd: the file descriptor of the pod's parent user namespace (caller must close)
//   - detected: true if a user namespace relationship was detected
//   - err: any error that occurred during detection
func DetectUserNamespace(networkNamespace string) (parentUserNsFd int, detected bool, err error) {
	parentNs, errParent := GetParentUserNsByNsPath(networkNamespace)
	if errParent != nil {
		return 0, false, fmt.Errorf("failed to get parent user namespace: %w", errParent)
	}

	grandParentNs, errGrandParent := GetParentUserNsByNsPath(BuildProcFdPath(parentNs))
	if errGrandParent != nil {
		unix.Close(parentNs)
		return 0, false, fmt.Errorf("failed to get grandparent user namespace: %w", errGrandParent)
	}
	defer unix.Close(grandParentNs)

	parentNsID, errParentNsID := GetNsID(BuildProcFdPath(parentNs))
	grandParentNsID, errGrandParentNsID := GetNsID(BuildProcFdPath(grandParentNs))

	if errParentNsID != nil || errGrandParentNsID != nil {
		unix.Close(parentNs)
		return 0, false, fmt.Errorf("failed to get namespace IDs: parent=%v, grandparent=%v", errParentNsID, errGrandParentNsID)
	}

	if parentNsID != grandParentNsID {
		return parentNs, true, nil
	}

	unix.Close(parentNs)
	return 0, false, nil
}
