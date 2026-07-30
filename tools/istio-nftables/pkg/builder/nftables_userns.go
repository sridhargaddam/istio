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
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"sigs.k8s.io/knftables"

	"istio.io/istio/pkg/log"
)

// NftUserNsImpl is an NftablesAPI implementation that wraps nft execution with nsenter
// to enter the pod's User Namespace before applying rules. This is required for
// hostUsers: false pods where skuid/skgid rules need to be programmed from within
// the pod's User Namespace context.
type NftUserNsImpl struct {
	inner          *NftImpl
	userNsPath     string
	networkNsPath  string
	nsenterPath    string
	nftPath        string
}

// NewNftUserNsImpl creates a new NftUserNsImpl that wraps nft commands with nsenter.
// userNsPath is the /proc path to the pod's parent user namespace fd.
// networkNsPath is the path to the pod's network namespace.
func NewNftUserNsImpl(family knftables.Family, table string, userNsPath, networkNsPath string) (*NftUserNsImpl, error) {
	inner, err := NewNftImpl(family, table)
	if err != nil {
		return nil, err
	}

	nsenterPath, err := exec.LookPath("nsenter")
	if err != nil {
		return nil, fmt.Errorf("k8s user namespace detected, but nsenter binary not found: %w", err)
	}

	nftPath, err := exec.LookPath("nft")
	if err != nil {
		return nil, fmt.Errorf("nft binary not found: %w", err)
	}

	return &NftUserNsImpl{
		inner:         inner,
		userNsPath:    userNsPath,
		networkNsPath: networkNsPath,
		nsenterPath:   nsenterPath,
		nftPath:       nftPath,
	}, nil
}

// NewTransaction starts a new transaction using the inner knftables backend.
func (n *NftUserNsImpl) NewTransaction() *knftables.Transaction {
	return n.inner.NewTransaction()
}

// Dump is used for logging purposes.
func (n *NftUserNsImpl) Dump(tx *knftables.Transaction) string {
	return n.inner.Dump(tx)
}

// ListElements returns a list of elements using nsenter-wrapped nft invocation.
// For listing operations, we delegate to the inner implementation since listing
// does not involve skuid/skgid validation by the kernel.
func (n *NftUserNsImpl) ListElements(ctx context.Context, objectType, name string) ([]*knftables.Element, error) {
	return n.inner.ListElements(ctx, objectType, name)
}

// Run applies a transaction by serializing it and executing through nsenter.
// This ensures the nft binary runs inside the pod's User Namespace, allowing
// skuid/skgid rules to be programmed correctly.
func (n *NftUserNsImpl) Run(ctx context.Context, tx *knftables.Transaction) error {
	txData := tx.String()
	if txData == "" {
		return nil
	}

	log.Debugf("Running nft via nsenter --user=%s --net=%s for user namespace support", n.userNsPath, n.networkNsPath)

	args := []string{
		fmt.Sprintf("--user=%s", n.userNsPath),
		fmt.Sprintf("--net=%s", n.networkNsPath),
		n.nftPath, "-f", "-",
	}

	cmd := exec.CommandContext(ctx, n.nsenterPath, args...)
	cmd.Stdin = strings.NewReader(txData)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nsenter nft run failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}
