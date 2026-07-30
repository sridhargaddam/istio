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

	"golang.org/x/sys/unix"

	"istio.io/istio/pkg/test/util/assert"
)

func TestBuildProcFdPath(t *testing.T) {
	path := BuildProcFdPath(42)
	expected := fmt.Sprintf("/proc/%d/fd/42", os.Getpid())
	assert.Equal(t, path, expected)
}

func TestGetNsID(t *testing.T) {
	id, err := GetNsID("/proc/self/ns/user")
	assert.NoError(t, err)
	if id == 0 {
		t.Fatal("expected non-zero namespace ID")
	}
}

func TestGetNsIDBadPath(t *testing.T) {
	_, err := GetNsID("/proc/self/ns/nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent namespace path")
	}
}

func TestGetParentUserNsByNsPath(t *testing.T) {
	fd, err := GetParentUserNsByNsPath("/proc/self/ns/net")
	if err != nil {
		t.Skipf("NS_GET_USERNS not supported: %v", err)
	}
	defer unix.Close(fd)
	if fd < 0 {
		t.Fatal("expected valid file descriptor")
	}
}

func TestDetectUserNamespaceHostReturnsNil(t *testing.T) {
	fd, detected, err := DetectUserNamespace("/proc/self/ns/net")
	if err != nil {
		t.Skipf("NS_GET_USERNS not supported in this environment: %v", err)
	}
	if detected {
		unix.Close(fd)
		t.Skip("running inside a user namespace, cannot test host detection")
	}
	assert.Equal(t, detected, false)
	assert.Equal(t, fd, 0)
}
