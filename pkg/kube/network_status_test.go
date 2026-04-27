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

package kube

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetCUDNIPsFromPod(t *testing.T) {
	tests := []struct {
		name    string
		pod     *corev1.Pod
		wantIPs []string
	}{
		{
			name: "UDN interface is default with non-eth0 interface",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NetworkStatusAnnotation: `[
							{"name":"ovn-kubernetes","interface":"eth0","ips":["10.128.0.5"],"default":false},
							{"name":"cudn-network","interface":"ovn-udn1","ips":["10.10.0.5","fd00::5"],"default":true}
						]`,
					},
				},
			},
			wantIPs: []string{"10.10.0.5", "fd00::5"},
		},
		{
			name: "only eth0 as default returns nil",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NetworkStatusAnnotation: `[
							{"name":"ovn-kubernetes","interface":"eth0","ips":["10.128.0.5"],"default":true}
						]`,
					},
				},
			},
			wantIPs: nil,
		},
		{
			name: "no annotation returns nil",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			wantIPs: nil,
		},
		{
			name: "empty annotation returns nil",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NetworkStatusAnnotation: "",
					},
				},
			},
			wantIPs: nil,
		},
		{
			name: "invalid JSON returns nil",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NetworkStatusAnnotation: "not-json",
					},
				},
			},
			wantIPs: nil,
		},
		{
			name: "default interface with empty IPs returns nil",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NetworkStatusAnnotation: `[
							{"name":"cudn-network","interface":"ovn-udn1","ips":[],"default":true}
						]`,
					},
				},
			},
			wantIPs: nil,
		},
		{
			name: "non-default non-eth0 interface returns nil",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NetworkStatusAnnotation: `[
							{"name":"ovn-kubernetes","interface":"eth0","ips":["10.128.0.5"],"default":true},
							{"name":"secondary","interface":"net1","ips":["192.168.1.5"],"default":false}
						]`,
					},
				},
			},
			wantIPs: nil,
		},
		{
			name:    "nil pod annotations",
			pod:     &corev1.Pod{},
			wantIPs: nil,
		},
		{
			name: "multiple default interfaces picks first matching non-eth0",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NetworkStatusAnnotation: `[
							{"name":"cluster-net","interface":"eth0","ips":["10.128.0.5"],"default":false},
							{"name":"cudn-primary","interface":"ovn-udn1","ips":["10.10.0.10"],"default":true},
							{"name":"cudn-secondary","interface":"ovn-udn2","ips":["10.20.0.10"],"default":true}
						]`,
					},
				},
			},
			wantIPs: []string{"10.10.0.10"},
		},
		{
			name: "annotation with only non-default non-eth0 entries returns nil",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						NetworkStatusAnnotation: `[
							{"name":"cluster-net","interface":"eth0","ips":["10.128.0.5"],"default":true},
							{"name":"extra-net","interface":"ovn-udn1","ips":["10.10.0.5"],"default":false}
						]`,
					},
				},
			},
			wantIPs: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCUDNIPsFromPod(tt.pod)
			if len(got) != len(tt.wantIPs) {
				t.Fatalf("GetCUDNIPsFromPod() = %v, want %v", got, tt.wantIPs)
			}
			for i := range got {
				if got[i] != tt.wantIPs[i] {
					t.Errorf("GetCUDNIPsFromPod()[%d] = %q, want %q", i, got[i], tt.wantIPs[i])
				}
			}
		})
	}
}
