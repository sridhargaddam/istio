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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ns"
	"golang.org/x/sys/unix"

	"istio.io/istio/pkg/log"
	"istio.io/istio/tools/common/config"
	"istio.io/istio/tools/common/userns"
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

	parentNsFd, isUserNs, _ := userns.DetectUserNamespace(netns)
	if isUserNs {
		defer unix.Close(parentNsFd)
	}

	return netNs.Do(func(_ ns.NetNS) error {
		if err := cfg.FillConfigFromEnvironment(); err != nil {
			return err
		}
		log.Infof("============= SGM:: ---> Start nftables configuration for %v =============", podName)
		defer log.Infof("============= End nftables configuration for %v =============", podName)

		if isUserNs {
			log.Infof("user namespace detected for pod %s, using sandbox for nftables", podName)
			return runNftInUserNsSandbox(parentNsFd, func() error {
				return nft.ProgramNftables(cfg)
			})
		}
		return nft.ProgramNftables(cfg)
	})
}

// runNftInUserNsSandbox builds a sandbox similar to the iptables runInSandbox, but
// instead of fixing lock files and NSS, it bind-mounts a wrapper script over the
// real nft binary so that knftables' internal nft invocations transparently go
// through nsenter to enter the pod's user namespace.
//
// This is necessary because the knftables library's exec interface is private and
// cannot be overridden. By replacing the nft binary (in a private mount namespace)
// with a wrapper that calls nsenter, we achieve user namespace entry without any
// changes to the knftables library.
func runNftInUserNsSandbox(parentUserNsFd int, f func() error) error {
	chErr := make(chan error, 1)
	currentNS, nerr := ns.GetCurrentNS()
	if nerr != nil {
		return fmt.Errorf("failed to get current namespace: %v", nerr)
	}

	executed := false
	go func() {
		chErr <- func() error {
			runtime.LockOSThread()

			if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
				return fmt.Errorf("failed to unshare mount ns: %v", err)
			}
			if err := currentNS.Set(); err != nil {
				return fmt.Errorf("failed to restore net ns: %v", err)
			}
			if err := unix.Mount("", "/", "", unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
				if slaveErr := unix.Mount("", "/", "", unix.MS_SLAVE|unix.MS_REC, ""); slaveErr != nil {
					return fmt.Errorf("failed to remount /: (MS_PRIVATE returned: %v; MS_SLAVE returned: %v)", err, slaveErr)
				}
			}

			realNftPath, err := exec.LookPath("nft")
			if err != nil {
				return fmt.Errorf("nft binary not found: %v", err)
			}

			// Mount a tmpfs for our wrapper files. We need our own tmpfs because:
			// 1. os.MkdirTemp creates 0700 dirs, but after nsenter --user switches to the
			//    pod's user namespace, host UID 0 becomes unmapped (nobody) and can't
			//    traverse 0700 directories.
			// 2. The host's /tmp may have noexec or restrictive SELinux labels.
			// The tmpfs is in our private mount namespace so it won't affect the host.
			sandboxDir := "/tmp/istio-nft-sandbox"
			if err := os.MkdirAll(sandboxDir, 0o755); err != nil {
				return fmt.Errorf("failed to create sandbox dir: %v", err)
			}
			if err := syscall.Mount("tmpfs", sandboxDir, "tmpfs", 0, "mode=0755,size=10m"); err != nil {
				return fmt.Errorf("failed to mount tmpfs for sandbox: %v", err)
			}
			defer func() {
				syscall.Unmount(sandboxDir, 0)
				os.RemoveAll(sandboxDir)
			}()

			realNftCopy := filepath.Join(sandboxDir, "nft.real")
			if err := copyFile(realNftPath, realNftCopy); err != nil {
				return fmt.Errorf("failed to copy nft binary: %v", err)
			}

			wrapperPath := filepath.Join(sandboxDir, "nft.wrapper")
			// "cd /" prevents "shell-init: error retrieving current directory" when
			// the cwd is inaccessible inside the sandboxed mount namespace.
			wrapperContent := fmt.Sprintf(
				"#!/bin/sh\ncd /\nexec nsenter --user=%s %s \"$@\"\n",
				userns.BuildProcFdPath(parentUserNsFd),
				realNftCopy,
			)
			if err := os.WriteFile(wrapperPath, []byte(wrapperContent), 0o755); err != nil {
				return fmt.Errorf("failed to write nft wrapper script: %v", err)
			}

			if err := syscall.Mount(wrapperPath, realNftPath, "", syscall.MS_BIND, ""); err != nil {
				return fmt.Errorf("failed to bind mount nft wrapper over %s: %v", realNftPath, err)
			}

			executed = true
			return f()
		}()
	}()

	err := <-chErr
	if err != nil && !executed {
		log.Warnf("failed to setup user ns sandbox for nftables, attempting without: %v", err)
		return f()
	}
	return err
}

// copyFile copies the file at src to dst, preserving executable permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
