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

package util

import (
	"fmt"
	"net"
	"net/netip"
)

// GetInterfaceIPv4 returns the IPv4 address of the specified network interface
func GetInterfaceIPv4(interfaceName string) (netip.Addr, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("interface %s not found: %w", interfaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to get addresses for %s: %w", interfaceName, err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			if ipv4 := ipNet.IP.To4(); ipv4 != nil {
				ip, _ := netip.AddrFromSlice(ipv4)
				return ip, nil
			}
		}
	}

	return netip.Addr{}, fmt.Errorf("no IPv4 address found on %s", interfaceName)
}

// GetInterfaceIPv6 returns the IPv6 address of the specified network interface
func GetInterfaceIPv6(interfaceName string) (netip.Addr, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("interface %s not found: %w", interfaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return netip.Addr{}, fmt.Errorf("failed to get addresses for %s: %w", interfaceName, err)
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			// Check if it's IPv6 and not IPv4
			if ipNet.IP.To4() == nil && len(ipNet.IP) == net.IPv6len {
				ip, _ := netip.AddrFromSlice(ipNet.IP)
				return ip, nil
			}
		}
	}

	return netip.Addr{}, fmt.Errorf("no IPv6 address found on %s", interfaceName)
}
