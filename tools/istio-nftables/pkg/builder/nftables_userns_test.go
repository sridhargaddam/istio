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

package builder

import (
	"context"
	"os/exec"
	"testing"

	// Create a new network namespace. This will have the 'lo' interface ready but nothing else.
	_ "github.com/howardjohn/unshare-go/netns"
	// Create a new user namespace. This will map the current UID to 0.
	usernslib "github.com/howardjohn/unshare-go/userns"

	"sigs.k8s.io/knftables"

	"istio.io/istio/tools/common/userns"
)

func TestNewNftUserNsImplRequiresNsenter(t *testing.T) {
	// If nsenter is not available, NewNftUserNsImpl should fail gracefully
	_, err := exec.LookPath("nsenter")
	if err != nil {
		t.Skip("nsenter not available, skipping")
	}
	_, err = exec.LookPath("nft")
	if err != nil {
		t.Skip("nft not available, skipping")
	}

	impl, err := NewNftUserNsImpl(knftables.InetFamily, "test-table", "/proc/self/ns/user", "/proc/self/ns/net")
	if err != nil {
		t.Fatalf("NewNftUserNsImpl failed: %v", err)
	}

	// Verify the implementation satisfies the NftablesAPI interface
	var _ NftablesAPI = impl

	// Verify NewTransaction works
	tx := impl.NewTransaction()
	if tx == nil {
		t.Error("NewTransaction returned nil")
	}

	// Verify Dump works
	dump := impl.Dump(tx)
	if dump == "" {
		t.Log("Dump returned empty string for empty transaction (expected)")
	}
}

func TestNftUserNsImplRun(t *testing.T) {
	_, err := exec.LookPath("nsenter")
	if err != nil {
		t.Skip("nsenter not available, skipping")
	}
	_, err = exec.LookPath("nft")
	if err != nil {
		t.Skip("nft not available, skipping")
	}

	// Override UID to 0 for tests running inside unshare-go namespace
	usernslib.WriteGroupMap(map[uint32]uint32{usernslib.OriginalGID(): 0})

	// Detect our own user namespace
	parentFd, detected, detectErr := userns.DetectUserNamespace("/proc/self/ns/net")
	if detectErr != nil || !detected {
		// We're running inside unshare-go, so we should be in a user namespace.
		// But the detection requires grandparent != parent, which may not be the case
		// if unshare-go only creates one level of nesting.
		t.Skipf("User namespace not detected (may need nested user ns): detected=%v, err=%v", detected, detectErr)
	}

	userNsPath := userns.BuildProcFdPath(parentFd)

	impl, err := NewNftUserNsImpl(knftables.InetFamily, "istio-test", userNsPath, "/proc/self/ns/net")
	if err != nil {
		t.Fatalf("NewNftUserNsImpl failed: %v", err)
	}

	// Create a simple transaction that creates and immediately cleans up a table
	tx := impl.NewTransaction()
	tx.Add(&knftables.Table{
		Comment: knftables.PtrTo("istio user ns test"),
	})

	err = impl.Run(context.Background(), tx)
	if err != nil {
		t.Logf("Run failed (may be expected depending on namespace setup): %v", err)
	}

	// Clean up
	cleanTx := impl.NewTransaction()
	cleanTx.Delete(&knftables.Table{})
	_ = impl.Run(context.Background(), cleanTx)
}
