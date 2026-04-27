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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
)

const (
	// NetworkStatusAnnotation is the standard CNI annotation containing network
	// interface information, as defined by the CNCF Network Plumbing Working Group.
	// See https://github.com/k8snetworkplumbingwg/network-attachment-definition-client
	NetworkStatusAnnotation = "k8s.v1.cni.cncf.io/network-status"
)

// networkStatus represents a single network interface entry in the
// k8s.v1.cni.cncf.io/network-status annotation.
type networkStatus struct {
	Name      string   `json:"name"`
	Interface string   `json:"interface"`
	IPs       []string `json:"ips"`
	Default   bool     `json:"default"`
}

// GetCUDNIPsFromPod extracts the primary User-Defined Network (UDN) IPs from
// a pod's network-status annotation. It returns the IPs from the default
// network interface that is NOT eth0, which in OVN-Kubernetes UDN environments
// represents the UDN overlay interface (e.g., ovn-udn1).
//
// The heuristic (default=true, interface!=eth0) is specific to OVN-Kubernetes
// UDN, where the default route is switched to the UDN interface while eth0
// retains the cluster default network IP.
//
// Returns nil if the annotation is missing, unparseable, or no matching
// interface is found.
func GetCUDNIPsFromPod(pod *corev1.Pod) []string {
	netStatusJSON, found := pod.Annotations[NetworkStatusAnnotation]
	if !found || netStatusJSON == "" {
		return nil
	}

	var netStatuses []networkStatus
	if err := json.Unmarshal([]byte(netStatusJSON), &netStatuses); err != nil {
		return nil
	}

	for _, ns := range netStatuses {
		if ns.Default && len(ns.IPs) > 0 && ns.Interface != "eth0" {
			return ns.IPs
		}
	}
	return nil
}
