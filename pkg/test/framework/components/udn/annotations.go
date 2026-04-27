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
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"istio.io/istio/pkg/test/framework/components/cluster"
)

// OVNOpenPortsAnnotation is the annotation key used by OVN-Kubernetes to
// declare additional ports that should be open on the UDN gateway.
const OVNOpenPortsAnnotation = "k8s.ovn.org/open-default-ports"

// AnnotateWaypointForCUDN patches a waypoint Gateway with the
// OVN-K open-default-ports annotation so UDN traffic can reach the proxy.
func AnnotateWaypointForCUDN(cls cluster.Cluster, namespace, name string) error {
	portsJSON, err := json.Marshal(defaultWaypointPorts())
	if err != nil {
		return fmt.Errorf("failed to marshal OVN ports annotation: %w", err)
	}

	patch := fmt.Sprintf(`{"metadata":{"annotations":{%q:%q}}}`,
		OVNOpenPortsAnnotation, string(portsJSON))

	gwClient := cls.GatewayAPI().GatewayV1().Gateways(namespace)
	_, err = gwClient.Patch(
		context.TODO(),
		name,
		types.MergePatchType,
		[]byte(patch),
		metav1.PatchOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to annotate waypoint %s/%s with OVN ports: %w", namespace, name, err)
	}
	return nil
}
