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
	"encoding/json"
	"fmt"
)

// ovnOpenPort represents a single port entry for the OVN open-default-ports annotation.
type ovnOpenPort struct {
	Protocol string `json:"protocol"`
	Port     int    `json:"port"`
}

// defaultWaypointPorts returns the set of ports that Istio components need
// opened on the OVN-K UDN gateway for proper traffic flow.
func defaultWaypointPorts() []ovnOpenPort {
	return []ovnOpenPort{
		{Protocol: "tcp", Port: 15001},
		{Protocol: "tcp", Port: 15006},
		{Protocol: "tcp", Port: 15008},
		{Protocol: "tcp", Port: 15009},
		{Protocol: "tcp", Port: 15010},
		{Protocol: "tcp", Port: 15021},
		{Protocol: "tcp", Port: 15090},
	}
}

// BuildControlPlaneValues returns Helm override YAML to configure istiod
// for OVN-K UDN environments. This enables the PILOT_ENABLE_OVNK_UDN
// flag and adds the open-default-ports annotation to the istiod pod template.
func BuildControlPlaneValues() string {
	portsJSON, _ := json.Marshal(defaultWaypointPorts())
	return fmt.Sprintf(`
values:
  pilot:
    env:
      PILOT_ENABLE_OVNK_UDN: "true"
    podAnnotations:
      k8s.ovn.org/open-default-ports: '%s'
`, string(portsJSON))
}

// BuildAmbientControlPlaneValues returns Helm override YAML to configure
// istiod for OVN-K UDN environments in ambient mode. In addition to the
// pilot configuration, it enables the UDN feature flag and configures
// the ztunnel pod template with the open-default-ports annotation.
func BuildAmbientControlPlaneValues() string {
	portsJSON, _ := json.Marshal(defaultWaypointPorts())
	return fmt.Sprintf(`
values:
  pilot:
    env:
      PILOT_ENABLE_OVNK_UDN: "true"
    podAnnotations:
      k8s.ovn.org/open-default-ports: '%s'
  ztunnel:
    podAnnotations:
      k8s.ovn.org/open-default-ports: '%s'
  cni:
    ambient:
      dnsCapture: "false"
`, string(portsJSON), string(portsJSON))
}
