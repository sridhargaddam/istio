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

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	// Create a new network namespace. This will have the 'lo' interface ready but nothing else.
	_ "github.com/howardjohn/unshare-go/netns"
	// Create a new user namespace. This will map the current UID to 0.
	_ "github.com/howardjohn/unshare-go/userns"
	"github.com/vishvananda/netns"

	"istio.io/istio/pkg/test/util/assert"
)

func TestRunNftInUserNsSandboxMountIsolation(t *testing.T) {
	// Create a known file path that we'll use to verify the sandbox doesn't leak mounts
	tmpDir := t.TempDir()
	markerFile := filepath.Join(tmpDir, "marker")
	assert.NoError(t, os.WriteFile(markerFile, []byte("before"), 0o644))

	originalNetNS, err := netns.Get()
	assert.NoError(t, err)
	var sandboxedNetNS netns.NsHandle

	// Use fd 0 (stdin) as a dummy fd; the sandbox setup itself will work,
	// but the wrapper script won't actually enter a user ns since fd 0 isn't
	// a user namespace fd. That's fine -- we're testing mount isolation, not nsenter.
	assert.NoError(t, runNftInUserNsSandbox(0, func() error {
		sandboxedNetNS, err = netns.Get()
		assert.NoError(t, err)
		return nil
	}))

	// Network namespace should be preserved across the sandbox
	assert.Equal(t, originalNetNS.Equal(sandboxedNetNS), true)

	// The marker file should still have its original content (sandbox mounts don't leak)
	after, err := os.ReadFile(markerFile)
	assert.NoError(t, err)
	assert.Equal(t, string(after), "before")
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")

	content := []byte("hello nft")
	assert.NoError(t, os.WriteFile(src, content, 0o755))
	assert.NoError(t, copyFile(src, dst))

	got, err := os.ReadFile(dst)
	assert.NoError(t, err)
	assert.Equal(t, string(got), string(content))

	info, err := os.Stat(dst)
	assert.NoError(t, err)
	assert.Equal(t, info.Mode().Perm()&0o755 != 0, true)
}
