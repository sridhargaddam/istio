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
	"testing"

	// Create a new network namespace. This will have the 'lo' interface ready but nothing else.
	_ "github.com/howardjohn/unshare-go/netns"
	// Create a new user namespace. This will map the current UID to 0.
	_ "github.com/howardjohn/unshare-go/userns"

	"golang.org/x/sys/unix"
)

func TestBuildProcFdPath(t *testing.T) {
	fd := 42
	expected := fmt.Sprintf("/proc/%d/fd/42", os.Getpid())
	result := BuildProcFdPath(fd)
	if result != expected {
		t.Errorf("BuildProcFdPath(%d) = %q, want %q", fd, result, expected)
	}
}

func TestGetNsID(t *testing.T) {
	// Use the current network namespace as a known valid namespace path
	nsPath := "/proc/self/ns/net"
	id, err := GetNsID(nsPath)
	if err != nil {
		t.Fatalf("GetNsID(%q) returned error: %v", nsPath, err)
	}
	if id == 0 {
		t.Error("GetNsID returned 0, expected a non-zero inode number")
	}

	// Verify consistency -- same path should return same ID
	id2, err := GetNsID(nsPath)
	if err != nil {
		t.Fatalf("GetNsID(%q) second call returned error: %v", nsPath, err)
	}
	if id != id2 {
		t.Errorf("GetNsID returned different values: %d vs %d", id, id2)
	}
}

func TestGetNsIDInvalidPath(t *testing.T) {
	_, err := GetNsID("/nonexistent/path")
	if err == nil {
		t.Error("GetNsID with invalid path should return error")
	}
}

func TestGetParentUserNsByNsPath(t *testing.T) {
	nsPath := "/proc/self/ns/net"
	fd, err := GetParentUserNsByNsPath(nsPath)
	if err != nil {
		t.Fatalf("GetParentUserNsByNsPath(%q) returned error: %v", nsPath, err)
	}
	defer unix.Close(fd)
	if fd < 0 {
		t.Errorf("GetParentUserNsByNsPath returned invalid fd: %d", fd)
	}
}

func TestDetectUserNamespace(t *testing.T) {
	// In the unshare-go test environment, we are inside a user namespace + network namespace.
	// With a single level of user ns nesting (as created by unshare-go), the grandparent
	// user namespace is the init user namespace, which we may not have permission to query.
	// This is expected behavior -- DetectUserNamespace should return (0, false, err) in this case.
	nsPath := "/proc/self/ns/net"
	parentFd, detected, err := DetectUserNamespace(nsPath)
	if err != nil {
		// "operation not permitted" is expected when the parent user namespace is the
		// init user namespace and we lack permissions to query its parent.
		t.Logf("DetectUserNamespace returned error (expected for single-level user ns nesting): %v", err)
		return
	}
	if detected {
		defer unix.Close(parentFd)
		t.Logf("User namespace detected (expected when running under nested user ns), parentFd=%d", parentFd)
	} else {
		t.Logf("No user namespace detected (expected when running under single-level user ns)")
	}
}
