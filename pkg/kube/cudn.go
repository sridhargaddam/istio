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

	"istio.io/istio/pkg/log"
)

const (
	// NetworkStatusAnnotation is the OVN-K annotation containing network interface information
	NetworkStatusAnnotation = "k8s.v1.cni.cncf.io/network-status"
)

// NetworkStatus represents the OVN-K network status annotation structure
type NetworkStatus struct {
	Name      string   `json:"name"`
	Interface string   `json:"interface"`
	IPs       []string `json:"ips"`
	Mac       string   `json:"mac"`
	Default   bool     `json:"default"`
}

// GetCUDNIPsFromPod extracts CUDN (Cluster User-Defined Network) IPs from pod annotations.
// OVN-K stores network information in the k8s.v1.cni.cncf.io/network-status annotation.
// For pods in CUDN, there will be multiple network interfaces, and the Primary CUDN interface
// will have "default": true
// Returns the list of IP addresses from the CUDN network interface, or nil if not found.
func GetCUDNIPsFromPod(pod *corev1.Pod) []string {
	if pod == nil || pod.Annotations == nil {
		return nil
	}

	netStatusJSON, found := pod.Annotations[NetworkStatusAnnotation]
	if !found {
		return nil
	}

	var netStatuses []NetworkStatus
	if err := json.Unmarshal([]byte(netStatusJSON), &netStatuses); err != nil {
		log.Debugf("Failed to parse network status annotation for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return nil
	}

	// Look for the CUDN network interface marked as default and is not eth0
	for _, ns := range netStatuses {
		if ns.Default && len(ns.IPs) > 0 && ns.Interface != "eth0" {
			log.Debugf("Found CUDN IPs for pod %s/%s: %v (interface: %s)",
				pod.Namespace, pod.Name, ns.IPs, ns.Interface)
			return ns.IPs
		}
	}

	return nil
}
