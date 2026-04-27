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

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeutil "istio.io/istio/pkg/kube"
	"istio.io/istio/pkg/test/framework/components/cluster"
	"istio.io/istio/pkg/test/framework/resource"
	"istio.io/istio/pkg/test/scopes"
)

// SuiteSetup registers UDN hooks on the test framework settings and creates
// the ClusterUserDefinedNetwork CR. Call this from TestMain's Setup chain
// when EnableCUDN is true.
//
// It performs the following:
//   - Registers NamespaceLabelHook to inject the CUDN primary network label.
//   - Registers WorkloadAddressHook to resolve CUDN IPs from pod annotations.
//   - Creates the ClusterUserDefinedNetwork CR on all clusters.
func SuiteSetup(ctx resource.Context) error {
	s := ctx.Settings()
	if !s.EnableCUDN {
		return nil
	}

	networkName := s.CUDNNetworkName
	scopes.Framework.Infof("UDN plugin: setting up CUDN %q", networkName)

	s.NamespaceLabelHook = AddNamespaceLabels(networkName)
	s.WorkloadAddressHook = workloadAddressFromCUDN

	for _, c := range ctx.AllClusters() {
		if err := applyCUDNCR(ctx, c, networkName, s.CUDNSelector); err != nil {
			return fmt.Errorf("failed to create CUDN CR on cluster %s: %w", c.Name(), err)
		}
	}
	return nil
}

// workloadAddressFromCUDN returns CUDN IPs from the pod's CNI
// network-status annotation, falling back to nil (so the default
// Pod.Status.PodIP is used).
func workloadAddressFromCUDN(pod *corev1.Pod) []string {
	ips := kubeutil.GetCUDNIPsFromPod(pod)
	if len(ips) > 0 {
		return ips
	}
	return nil
}

// preInstallCreateZtunnelNamespace returns a PreInstallHook that creates the
// istio-system namespace (used by ztunnel) with the CUDN primary network
// label. OVN-K requires this label at namespace creation time.
func preInstallCreateZtunnelNamespace(networkName string) func(ctx resource.Context) error {
	return func(ctx resource.Context) error {
		for _, c := range ctx.AllClusters() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "istio-system",
					Labels: map[string]string{
						CUDNPrimaryNetworkLabel: networkName,
					},
				},
			}
			_, err := c.Kube().CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
			if err != nil {
				scopes.Framework.Infof("UDN plugin: namespace istio-system already exists or error: %v", err)
			}
		}
		return nil
	}
}

// applyCUDNCR creates the ClusterUserDefinedNetwork custom resource.
func applyCUDNCR(ctx resource.Context, c cluster.Cluster, name, selector string) error {
	matchExpr := ""
	if selector != "" {
		matchExpr = fmt.Sprintf(`
      matchExpressions:
        - key: %s
          operator: Exists`, selector)
	}

	yaml := fmt.Sprintf(`
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: %s
spec:
  namespaceSelector:%s
  network:
    topology: Layer3
    layer3:
      role: Primary
      subnets:
        - cidr: 10.10.0.0/16
          hostSubnet: 24
        - cidr: 2014:100:200::0/60
          hostSubnet: 64
`, name, matchExpr)

	return ctx.ConfigKube(c).YAML("", yaml).Apply()
}
