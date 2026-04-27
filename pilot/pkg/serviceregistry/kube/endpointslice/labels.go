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

package endpointslice

import (
	discovery "k8s.io/api/discovery/v1"

	"istio.io/istio/pilot/pkg/features"
)

const (
	// OVNKubernetesServiceLabel is the EndpointSlice label used by OVN-Kubernetes
	// to associate mirrored EndpointSlices with services in User-Defined Networks.
	OVNKubernetesServiceLabel = "k8s.ovn.org/service-name"
)

// GetServiceLabelKey returns the EndpointSlice label key used to identify the
// owning service. When PILOT_ENABLE_OVNK_UDN is enabled, OVN-Kubernetes mirrored
// EndpointSlices use a different label than the standard Kubernetes one.
func GetServiceLabelKey() string {
	if features.EnableOVNKubernetesUDN {
		return OVNKubernetesServiceLabel
	}
	return discovery.LabelServiceName
}

// GetServiceNameFromLabels extracts the service name from EndpointSlice labels
// using the appropriate label key based on the current configuration.
func GetServiceNameFromLabels(labels map[string]string) (string, bool) {
	name, found := labels[GetServiceLabelKey()]
	return name, found
}
