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

// This is a sample chained plugin that supports multiple CNI versions. It
// parses prevResult according to the cniVersion
package plugin

import (
	"fmt"

	"github.com/containernetworking/plugins/pkg/ns"
	"golang.org/x/sys/unix"
	"sigs.k8s.io/knftables"

	"istio.io/istio/pkg/log"
	"istio.io/istio/tools/common/config"
	"istio.io/istio/tools/common/userns"
	"istio.io/istio/tools/istio-nftables/pkg/builder"
	"istio.io/istio/tools/istio-nftables/pkg/capture"
	"istio.io/istio/tools/istio-nftables/pkg/nft"
)

// Program defines a method which programs nftables based on the parameters
// provided in Redirect.
func (n *nftables) Program(podName, netns string, rdrct *Redirect) error {
	cfg := config.DefaultConfig()
	cfg.HostFilesystemPodNetwork = true
	cfg.NetworkNamespace = netns
	cfg.ProxyPort = rdrct.targetPort
	cfg.ProxyUID = rdrct.noRedirectUID
	cfg.ProxyGID = rdrct.noRedirectGID
	cfg.InboundInterceptionMode = rdrct.redirectMode
	cfg.OutboundIPRangesInclude = rdrct.includeIPCidrs
	cfg.InboundPortsExclude = rdrct.excludeInboundPorts
	cfg.InboundPortsInclude = rdrct.includeInboundPorts
	cfg.ExcludeInterfaces = rdrct.excludeInterfaces
	cfg.OutboundPortsExclude = rdrct.excludeOutboundPorts
	cfg.OutboundPortsInclude = rdrct.includeOutboundPorts
	cfg.OutboundIPRangesExclude = rdrct.excludeIPCidrs
	cfg.RerouteVirtualInterfaces = rdrct.rerouteVirtualInterfaces
	cfg.RedirectDNS = rdrct.dnsRedirect
	cfg.CaptureAllDNS = rdrct.dnsRedirect
	cfg.DropInvalid = rdrct.invalidDrop
	cfg.DualStack = rdrct.dualStack

	netNs, err := getNs(netns)
	if err != nil {
		err = fmt.Errorf("failed to open netns %q: %s", netns, err)
		return err
	}
	defer netNs.Close()

	// Detect user namespace (hostUsers: false) before entering the network namespace.
	var nftProvider capture.NftProviderFunc
	parentUserNsFd, isUserNs, detectErr := userns.DetectUserNamespace(netns)
	if detectErr != nil {
		log.Debugf("User namespace detection returned error (proceeding without user ns support): %v", detectErr)
	}
	if isUserNs {
		defer unix.Close(parentUserNsFd)
		userNsPath := userns.BuildProcFdPath(parentUserNsFd)
		log.Infof("User namespace detected for pod %v, using nsenter-wrapped nft execution (--user=%s --net=%s)", podName, userNsPath, netns)
		nftProvider = func(family knftables.Family, table string) (builder.NftablesAPI, error) {
			return builder.NewNftUserNsImpl(family, table, userNsPath, netns)
		}
	}

	return netNs.Do(func(_ ns.NetNS) error {
		if err := cfg.FillConfigFromEnvironment(); err != nil {
			return err
		}
		log.Infof("============= Start nftables configuration for %v =============", podName)
		defer log.Infof("============= End nftables configuration for %v =============", podName)
		return nft.ProgramNftablesWithProvider(cfg, nftProvider)
	})
}
