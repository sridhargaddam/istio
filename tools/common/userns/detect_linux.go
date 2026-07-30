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

	"istio.io/istio/pkg/log"
)

// BuildProcFdPath returns the /proc path for a file descriptor in the current process.
func BuildProcFdPath(fd int) string {
	return fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), fd)
}

// GetNsID returns the kernel namespace inode number for the namespace at nsPath.
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

// GetParentUserNsByNsPath retrieves the owning user namespace fd for the namespace
// at nsPath via the NS_GET_USERNS ioctl. The caller must close the returned fd.
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

// DetectUserNamespace checks whether the given network namespace path belongs to
// a pod running with Kubernetes user namespaces (hostUsers: false). It walks the
// user namespace hierarchy: if the parent and grandparent user namespaces differ,
// the network namespace is inside a non-root user namespace.
//
// On success, returns the parent user namespace fd (caller must close it) and true.
// Returns 0, false, nil if no user namespace is detected.
func DetectUserNamespace(networkNamespace string) (parentUserNsFd int, detected bool, err error) {
	parentNs, errParent := GetParentUserNsByNsPath(networkNamespace)
	if errParent != nil {
		return 0, false, errParent
	}

	parentNsPath := BuildProcFdPath(parentNs)
	grandParentNs, errGrandParent := GetParentUserNsByNsPath(parentNsPath)
	if errGrandParent != nil {
		unix.Close(parentNs)
		return 0, false, errGrandParent
	}
	defer unix.Close(grandParentNs)

	netNsID, errNsID := GetNsID(networkNamespace)
	parentNsID, errParentNsID := GetNsID(parentNsPath)
	grandParentNsID, errGrandParentNsID := GetNsID(BuildProcFdPath(grandParentNs))

	if errNsID != nil || errParentNsID != nil || errGrandParentNsID != nil {
		unix.Close(parentNs)
		return 0, false, fmt.Errorf("failed to get namespace IDs: netns=%v parent=%v grandparent=%v",
			errNsID, errParentNsID, errGrandParentNsID)
	}

	if parentNsID == grandParentNsID {
		unix.Close(parentNs)
		return 0, false, nil
	}

	log.Debugf("k8s user namespaces relationship detected: base %d -> %d -> %d",
		grandParentNsID, parentNsID, netNsID)

	return parentNs, true, nil
}
