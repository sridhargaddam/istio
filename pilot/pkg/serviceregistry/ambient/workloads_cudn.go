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

package ambient

import (
	"fmt"
	"net/netip"

	v1 "k8s.io/api/core/v1"
	discovery "k8s.io/api/discovery/v1"

	"istio.io/istio/pilot/pkg/features"
	"istio.io/istio/pilot/pkg/serviceregistry/kube/endpointslice"
	kubeutil "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/kube/krt"
	"istio.io/istio/pkg/log"
	"istio.io/istio/pkg/slices"
)

// resolveWorkloadIPs returns the routable IPs for a pod as [][]byte.
//
// When PILOT_ENABLE_OVNK_UDN is enabled, it resolves IPs exclusively from
// CUDN sources (mirrored EndpointSlices, then the CNI network-status
// annotation). If neither source yields an IP, it returns an error rather
// than falling back to Pod.Status.PodIPs, because the standard PodIPs are
// not routable on the User-Defined Network.
//
// When the flag is disabled (default), it uses the standard Pod.Status.PodIPs.
func resolveWorkloadIPs(
	ctx krt.HandlerContext,
	p *v1.Pod,
	endpointSlicesAddressIndex krt.Index[TargetRef, *discovery.EndpointSlice],
) ([][]byte, error) {
	if features.EnableOVNKubernetesUDN {
		if ips := getCUDNIPsFromEndpointSlices(ctx, p, endpointSlicesAddressIndex); len(ips) > 0 {
			return ips, nil
		}
		if ips := getCUDNIPsFromPodAnnotation(p); len(ips) > 0 {
			return ips, nil
		}
		return nil, fmt.Errorf("OVNK UDN enabled but no CUDN IPs found for pod %s/%s from EndpointSlices or network-status annotation",
			p.Namespace, p.Name)
	}

	k8sPodIPs := getPodIPs(p)
	if len(k8sPodIPs) == 0 {
		return nil, nil
	}
	return slices.MapErr(k8sPodIPs, func(e v1.PodIP) ([]byte, error) {
		n, err := netip.ParseAddr(e.IP)
		if err != nil {
			return nil, err
		}
		return n.AsSlice(), nil
	})
}

// getCUDNIPsFromEndpointSlices retrieves CUDN IPs from OVN-Kubernetes mirrored
// EndpointSlices. These are EndpointSlices that reference the pod via a TargetRef
// and carry the k8s.ovn.org/service-name label.
func getCUDNIPsFromEndpointSlices(
	ctx krt.HandlerContext,
	p *v1.Pod,
	addressIndex krt.Index[TargetRef, *discovery.EndpointSlice],
) [][]byte {
	tr := TargetRef{
		Namespace: p.Namespace,
		Name:      p.Name,
		UID:       p.UID,
	}
	matchedSlices := addressIndex.Fetch(ctx, tr)
	for _, es := range matchedSlices {
		if _, found := endpointslice.GetServiceNameFromLabels(es.Labels); !found {
			continue
		}
		for _, ep := range es.Endpoints {
			if ep.TargetRef == nil || ep.TargetRef.UID != p.UID {
				continue
			}
			if len(ep.Addresses) == 0 {
				continue
			}
			ips, err := slices.MapErr(ep.Addresses, func(addr string) ([]byte, error) {
				n, err := netip.ParseAddr(addr)
				if err != nil {
					return nil, err
				}
				return n.AsSlice(), nil
			})
			if err != nil {
				log.Debugf("failed to parse CUDN EndpointSlice addresses for pod %s/%s: %v", p.Namespace, p.Name, err)
				continue
			}
			log.Debugf("resolved CUDN IPs from EndpointSlice for pod %s/%s: %v", p.Namespace, p.Name, ep.Addresses)
			return ips
		}
	}
	return nil
}

// getCUDNIPsFromPodAnnotation extracts CUDN IPs from the pod's CNI
// network-status annotation (k8s.v1.cni.cncf.io/network-status).
func getCUDNIPsFromPodAnnotation(p *v1.Pod) [][]byte {
	cudnIPs := kubeutil.GetCUDNIPsFromPod(p)
	if len(cudnIPs) == 0 {
		return nil
	}
	ips, err := slices.MapErr(cudnIPs, func(ip string) ([]byte, error) {
		n, err := netip.ParseAddr(ip)
		if err != nil {
			return nil, err
		}
		return n.AsSlice(), nil
	})
	if err != nil {
		log.Debugf("failed to parse CUDN annotation IPs for pod %s/%s: %v", p.Namespace, p.Name, err)
		return nil
	}
	log.Debugf("resolved CUDN IPs from pod annotation for pod %s/%s: %v", p.Namespace, p.Name, cudnIPs)
	return ips
}
