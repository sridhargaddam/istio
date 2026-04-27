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
	"testing"

	discovery "k8s.io/api/discovery/v1"

	"istio.io/istio/pilot/pkg/features"
	"istio.io/istio/pkg/test"
)

func TestGetServiceLabelKey(t *testing.T) {
	t.Run("default returns kubernetes label", func(t *testing.T) {
		test.SetForTest(t, &features.EnableOVNKubernetesUDN, false)
		got := GetServiceLabelKey()
		if got != discovery.LabelServiceName {
			t.Errorf("GetServiceLabelKey() = %q, want %q", got, discovery.LabelServiceName)
		}
	})

	t.Run("ovnk udn returns ovn label", func(t *testing.T) {
		test.SetForTest(t, &features.EnableOVNKubernetesUDN, true)
		got := GetServiceLabelKey()
		if got != OVNKubernetesServiceLabel {
			t.Errorf("GetServiceLabelKey() = %q, want %q", got, OVNKubernetesServiceLabel)
		}
	})
}

func TestGetServiceNameFromLabels(t *testing.T) {
	tests := []struct {
		name      string
		ovnkUDN   bool
		labels    map[string]string
		wantName  string
		wantFound bool
	}{
		{
			name:      "standard k8s label found",
			ovnkUDN:   false,
			labels:    map[string]string{discovery.LabelServiceName: "my-svc"},
			wantName:  "my-svc",
			wantFound: true,
		},
		{
			name:      "standard k8s label not found",
			ovnkUDN:   false,
			labels:    map[string]string{"other": "value"},
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "ovnk label found when enabled",
			ovnkUDN:   true,
			labels:    map[string]string{OVNKubernetesServiceLabel: "ovn-svc"},
			wantName:  "ovn-svc",
			wantFound: true,
		},
		{
			name:      "ovnk label not found when enabled",
			ovnkUDN:   true,
			labels:    map[string]string{discovery.LabelServiceName: "k8s-svc"},
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "both labels present uses standard when disabled",
			ovnkUDN:   false,
			labels:    map[string]string{discovery.LabelServiceName: "k8s-svc", OVNKubernetesServiceLabel: "ovn-svc"},
			wantName:  "k8s-svc",
			wantFound: true,
		},
		{
			name:      "both labels present uses ovnk when enabled",
			ovnkUDN:   true,
			labels:    map[string]string{discovery.LabelServiceName: "k8s-svc", OVNKubernetesServiceLabel: "ovn-svc"},
			wantName:  "ovn-svc",
			wantFound: true,
		},
		{
			name:      "nil labels",
			ovnkUDN:   false,
			labels:    nil,
			wantName:  "",
			wantFound: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			test.SetForTest(t, &features.EnableOVNKubernetesUDN, tt.ovnkUDN)
			gotName, gotFound := GetServiceNameFromLabels(tt.labels)
			if gotName != tt.wantName || gotFound != tt.wantFound {
				t.Errorf("GetServiceNameFromLabels() = (%q, %v), want (%q, %v)",
					gotName, gotFound, tt.wantName, tt.wantFound)
			}
		})
	}
}
