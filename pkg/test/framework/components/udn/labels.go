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

package udn

const (
	// CUDNPrimaryNetworkLabel is the OVN-Kubernetes label that must be
	// present on a namespace at creation time to associate it with a
	// ClusterUserDefinedNetwork as the primary network.
	CUDNPrimaryNetworkLabel = "k8s.ovn.org/primary-user-defined-network"
)

// AddNamespaceLabels injects the CUDN primary network label into a label map.
// This function is intended to be used as a NamespaceLabelHook.
func AddNamespaceLabels(networkName string) func(labels map[string]string) {
	return func(labels map[string]string) {
		labels[CUDNPrimaryNetworkLabel] = networkName
	}
}
