/*
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
*/

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils"
	"github.com/j-keck/arping"
	"github.com/vishvananda/netlink"
)

// NOTE: portions of the code in this file are copied from
// github.om/containernetworking/plugins

const defaultBridgeName = "k-vswitch0"

type NetConf struct {
	types.NetConf

	BridgeName string `json:"bridge"`

	// copied from bridge plugin, clean up later
	IsGW         bool `json:"isGateway`
	IsDefaultGW  bool `json:"isDefaultGateway"`
	ForceAddress bool `json:"forceAddress"`
	IPMasq       bool `json:"ipMasq"`
	MTU          int  `json:"mtu"`
	HairpinMode  bool `json:"hairpinMode"`
	PromiscMode  bool `json:"promiscMode"`
}

type gwInfo struct {
	gws               []net.IPNet
	family            int
	defaultRouteFound bool
}

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func loadNetConf(bytes []byte) (*NetConf, string, error) {
	n := &NetConf{
		BridgeName: defaultBridgeName,
	}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, "", fmt.Errorf("failed to load netconf: %v", err)
	}
	return n, n.CNIVersion, nil
}

func bridgeByName(name string) (netlink.Link, error) {
	br, err := netlink.LinkByName(name)
	if err != nil {
		return nil, fmt.Errorf("could not lookup %q: %v", name, err)
	}

	return br, nil
}

func getBridgeInterface() (*current.Interface, error) {
	br, err := bridgeByName(defaultBridgeName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bridge %q: %v", defaultBridgeName, err)
	}

	return &current.Interface{
		Name: br.Attrs().Name,
		Mac:  br.Attrs().HardwareAddr.String(),
	}, nil
}

// getPodInfo parses the values in CNI_ARGS and checks for values
// K8S_POD_NAMESPACE and K8S_POD_NAME which should be set by the kubelet
func getPodInfo() (string, string, error) {
	cniArgs := os.Getenv("CNI_ARGS")
	if cniArgs == "" {
		return "", "", errors.New("env var CNI_ARGS was empty")
	}

	var podNamespace, podName string

	args := strings.Split(cniArgs, ";")
	for _, arg := range args {
		if strings.Contains(arg, "K8S_POD_NAMESPACE") {
			podNamespace = strings.TrimPrefix(arg, "K8S_POD_NAMESPACE=")
			continue
		}

		if strings.Contains(arg, "K8S_POD_NAME") {
			podName = strings.TrimPrefix(arg, "K8S_POD_NAME=")
			continue
		}
	}

	return podNamespace, podName, nil
}

// addPort adds port to a bridge, also adding the container ID
// in the external-ids column of the ports table
func addPort(bridgeName, port, containerMac, netNS, podNamespace, podName string) error {
	commands := []string{
		"--may-exist", "add-port", bridgeName, port,
		"--", "set", "port", port,
		fmt.Sprintf("external-ids:netns=%s", normalizedNetNS(netNS)),
		fmt.Sprintf("external-ids:k8s_pod_namespace=%s", podNamespace),
		fmt.Sprintf("external-ids:k8s_pod_name=%s", podName),
		"--", "set", "interface", port,
		fmt.Sprintf("external-ids:netns=%s", normalizedNetNS(netNS)),
		fmt.Sprintf("external-ids:k8s_pod_namespace=%s", podNamespace),
		fmt.Sprintf("external-ids:k8s_pod_name=%s", podName),
		"--", "set", "port", port, fmt.Sprintf("mac=\"%s\"", containerMac),
	}

	_, err := exec.Command("ovs-vsctl", commands...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add OVS port %q to bridge %q, err: %v",
			port, bridgeName, err)
	}

	return nil
}

func delPort(bridge, port string) error {
	commands := []string{
		"--if-exists", "del-port", bridge, port,
	}

	out, err := exec.Command("ovs-vsctl", commands...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete OVS port %q on bridge %q, err: %v, output: %q",
			port, bridge, err, string(out))
	}

	return nil
}

// findPort finds the OVS port name by network namespace
// TODO: CNI delete can be called multiple times so this method will often
// return errors even though the port was successfully deleted.
func findPort(bridge, netNS, podNamespace, podName string) (string, error) {
	commands := []string{
		"--format=json", "--column=name", "find",
		"port", fmt.Sprintf("external-ids:netns=%s", normalizedNetNS(netNS)),
		fmt.Sprintf("external-ids:k8s_pod_namespace=%s", podNamespace),
		fmt.Sprintf("external-ids:k8s_pod_name=%s", podName),
	}

	out, err := exec.Command("ovs-vsctl", commands...).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get OVS port with net namespace %q from bridge %q, err: %v",
			netNS, bridge, err)
	}

	dbData := struct {
		Data [][]string
	}{}
	if err = json.Unmarshal(out, &dbData); err != nil {
		return "", err
	}

	if len(dbData.Data) == 0 {
		// TODO: might make more sense to not return an error here since
		// CNI delete can be called multiple times.
		return "", fmt.Errorf("OVS port for %s/%s was not found, OVS DB data: %v, output: %q",
			podNamespace, podName, dbData.Data, string(out))
	}

	portName := dbData.Data[0][0]
	return portName, nil
}

// normalizedNetNS takes the NetNS (typically a path to a network namespace) and normalizes it
// so it can be added to the OVS Port DB.
func normalizedNetNS(netns string) string {
	return strings.Replace(netns, "/", "", -1)
}

func setupVeth(netns ns.NetNS, ifName string) (*current.Interface, *current.Interface, error) {
	contIface := &current.Interface{}
	hostIface := &current.Interface{}

	err := netns.Do(func(hostNS ns.NetNS) error {
		// create the veth pair in the container and move host end into host netns
		// TODO don't hardcode the MTU
		hostVeth, containerVeth, err := setupVethPair(ifName, 1500, hostNS)
		if err != nil {
			return err
		}
		contIface.Name = containerVeth.Name
		contIface.Mac = containerVeth.HardwareAddr.String()
		contIface.Sandbox = netns.Path()
		hostIface.Name = hostVeth.Name
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// need to lookup hostVeth again as its index has changed during ns move
	hostVeth, err := netlink.LinkByName(hostIface.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup %q: %v", hostIface.Name, err)
	}
	hostIface.Mac = hostVeth.Attrs().HardwareAddr.String()

	// set hairpin mode, do we need this?
	// if err = netlink.LinkSetHairpin(hostVeth, hairpinMode); err != nil {
	//	return nil, nil, fmt.Errorf("failed to setup hairpin mode for %v: %v", hostVeth.Attrs().Name, err)
	// }

	return hostIface, contIface, nil
}

// calcGateways processes the results from the IPAM plugin and does the
// following for each IP family:
//    - Calculates and compiles a list of gateway addresses
//    - Adds a default route if needed
func calcGateways(result *current.Result, n *NetConf) (*gwInfo, *gwInfo, error) {

	gwsV4 := &gwInfo{}
	gwsV6 := &gwInfo{}

	for _, ipc := range result.IPs {

		// Determine if this config is IPv4 or IPv6
		var gws *gwInfo
		defaultNet := &net.IPNet{}
		switch {
		case ipc.Address.IP.To4() != nil:
			gws = gwsV4
			gws.family = netlink.FAMILY_V4
			defaultNet.IP = net.IPv4zero
		case len(ipc.Address.IP) == net.IPv6len:
			gws = gwsV6
			gws.family = netlink.FAMILY_V6
			defaultNet.IP = net.IPv6zero
		default:
			return nil, nil, fmt.Errorf("Unknown IP object: %v", ipc)
		}
		defaultNet.Mask = net.IPMask(defaultNet.IP)

		// All IPs currently refer to the container interface
		ipc.Interface = current.Int(2)

		// If not provided, calculate the gateway address corresponding
		// to the selected IP address
		if ipc.Gateway == nil && n.IsGW {
			ipc.Gateway = calcGatewayIP(&ipc.Address)
		}

		// Add a default route for this family using the current
		// gateway address if necessary.
		if n.IsDefaultGW && !gws.defaultRouteFound {
			for _, route := range result.Routes {
				if route.GW != nil && defaultNet.String() == route.Dst.String() {
					gws.defaultRouteFound = true
					break
				}
			}
			if !gws.defaultRouteFound {
				result.Routes = append(
					result.Routes,
					&types.Route{Dst: *defaultNet, GW: ipc.Gateway},
				)
				gws.defaultRouteFound = true
			}
		}

		// Append this gateway address to the list of gateways
		if n.IsGW {
			gw := net.IPNet{
				IP:   ipc.Gateway,
				Mask: ipc.Address.Mask,
			}
			gws.gws = append(gws.gws, gw)
		}
	}
	return gwsV4, gwsV6, nil
}

func calcGatewayIP(ipn *net.IPNet) net.IP {
	nid := ipn.IP.Mask(ipn.Mask)
	return ip.NextIP(nid)
}

// disableIPV6DAD disables IPv6 Duplicate Address Detection (DAD)
// for an interface, if the interface does not support enhanced_dad.
// We do this because interfaces with hairpin mode will see their own DAD packets
func disableIPV6DAD(ifName string) error {
	// ehanced_dad sends a nonce with the DAD packets, so that we can safely
	// ignore ourselves
	enh, err := ioutil.ReadFile(fmt.Sprintf("/proc/sys/net/ipv6/conf/%s/enhanced_dad", ifName))
	if err == nil && string(enh) == "1\n" {
		return nil
	}
	f := fmt.Sprintf("/proc/sys/net/ipv6/conf/%s/accept_dad", ifName)
	return ioutil.WriteFile(f, []byte("0"), 0644)
}

func enableIPForward(family int) error {
	if family == netlink.FAMILY_V4 {
		return ip.EnableIP4Forward()
	}
	return ip.EnableIP6Forward()
}

func cmdAdd(args *skel.CmdArgs) error {
	success := false

	netConf, cniVersion, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	bridge, err := getBridgeInterface()
	if err != nil {
		return fmt.Errorf("failed to setup bridge: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	hostInterface, containerInterface, err := setupVeth(netns, args.IfName)
	if err != nil {
		return err
	}

	podNamespace, podName, err := getPodInfo()
	if err != nil {
		return err
	}

	err = addPort(netConf.BridgeName, hostInterface.Name, containerInterface.Mac, args.Netns, podNamespace, podName)
	if err != nil {
		return err
	}

	hostLink, err := netlink.LinkByName(hostInterface.Name)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetUp(hostLink); err != nil {
		return err
	}

	result := &current.Result{CNIVersion: cniVersion, Interfaces: []*current.Interface{bridge, hostInterface, containerInterface}}

	if netConf.IPAM.Type != "" {
		// run the IPAM plugin and get back the config to apply
		r, err := ipam.ExecAdd(netConf.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}

		// release IP in case of failure
		defer func() {
			if !success {
				os.Setenv("CNI_COMMAND", "DEL")
				ipam.ExecDel(netConf.IPAM.Type, args.StdinData)
				os.Setenv("CNI_COMMAND", "ADD")
			}
		}()

		// Convert whatever the IPAM result was into the current Result type
		ipamResult, err := current.NewResultFromResult(r)
		if err != nil {
			return err
		}

		result.IPs = ipamResult.IPs
		result.Routes = ipamResult.Routes

		if len(result.IPs) == 0 {
			return errors.New("IPAM plugin returned missing IP config")
		}

		// Gather gateway information for each IP family
		gwsV4, gwsV6, err := calcGateways(result, netConf)
		if err != nil {
			return err
		}

		// Configure the container hardware address and IP address(es)
		if err := netns.Do(func(_ ns.NetNS) error {
			contVeth, err := net.InterfaceByName(args.IfName)
			if err != nil {
				return err
			}

			// Disable IPv6 DAD just in case hairpin mode is enabled on the
			// bridge. Hairpin mode causes echos of neighbor solicitation
			// packets, which causes DAD failures.
			for _, ipc := range result.IPs {
				if ipc.Version == "6" && (netConf.HairpinMode || netConf.PromiscMode) {
					if err := disableIPV6DAD(args.IfName); err != nil {
						return err
					}
					break
				}
			}

			// Add the IP to the interface
			if err := ipam.ConfigureIface(args.IfName, result); err != nil {
				return err
			}

			// Send a gratuitous arp
			for _, ipc := range result.IPs {
				if ipc.Version == "4" {
					_ = arping.GratuitousArpOverIface(ipc.Address.IP, *contVeth)
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("failed to configure container addresses: %v", err)
		}

		if netConf.IsGW {
			var firstV4Addr net.IP
			// Set the IP address(es) on the bridge and enable forwarding
			for _, gws := range []*gwInfo{gwsV4, gwsV6} {
				for _, gw := range gws.gws {
					if gw.IP.To4() != nil && firstV4Addr == nil {
						firstV4Addr = gw.IP
					}

					br, err := bridgeByName(netConf.BridgeName)
					if err != nil {
						return fmt.Errorf("failed to get bridge link: %v", err)
					}

					// TODO: configure force address here
					err = ensureBridgeAddr(br, gws.family, &gw, true)
					if err != nil {
						return fmt.Errorf("failed to set bridge addr: %v", err)
					}
				}

				if gws.gws != nil {
					if err = enableIPForward(gws.family); err != nil {
						return fmt.Errorf("failed to enable forwarding: %v", err)
					}
				}
			}
		}

		if netConf.IPMasq {
			chain := utils.FormatChainName(netConf.Name, args.ContainerID)
			comment := utils.FormatComment(netConf.Name, args.ContainerID)
			for _, ipc := range result.IPs {
				if err = ip.SetupIPMasq(ip.Network(&ipc.Address), chain, comment); err != nil {
					return err
				}
			}
		}
	}

	// Refetch the bridge since its MAC address may change when the first
	// veth is added or after its IP address is set
	br, err := bridgeByName(netConf.BridgeName)
	if err != nil {
		return err
	}
	bridge.Mac = br.Attrs().HardwareAddr.String()

	result.DNS = netConf.DNS

	// re: defer statement above for cleaning up IPAM
	success = true

	return types.PrintResult(result, cniVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	netConf, _, err := loadNetConf(args.StdinData)
	if err != nil {
		return err
	}

	podNamespace, podName, err := getPodInfo()
	if err != nil {
		return err
	}

	// attempt to deleted the associated port for the network namespace if it exists
	// otherwise skip assuming this port it was already deleted it
	if args.Netns != "" {
		portName, err := findPort(netConf.BridgeName, args.Netns, podNamespace, podName)
		if err != nil {
			return err
		}

		err = delPort(netConf.BridgeName, portName)
		if err != nil {
			return err
		}
	}

	if netConf.IPAM.Type != "" {
		if err := ipam.ExecDel(netConf.IPAM.Type, args.StdinData); err != nil {
			return err
		}
	}

	if args.Netns == "" {
		return nil
	}

	// There is a netns so try to clean up. Delete can be called multiple times
	// so don't return an error if the device is already removed.
	// If the device isn't there then don't try to clean up IP masq either.
	var ipnets []*net.IPNet
	err = ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		var err error
		ipnets, err = ip.DelLinkByNameAddr(args.IfName)
		if err != nil && err == ip.ErrLinkNotFound {
			return nil
		}
		return err
	})
	if err != nil {
		return err
	}

	if netConf.IPAM.Type != "" && netConf.IPMasq {
		chain := utils.FormatChainName(netConf.Name, args.ContainerID)
		comment := utils.FormatComment(netConf.Name, args.ContainerID)
		for _, ipn := range ipnets {
			if err := ip.TeardownIPMasq(ipn, chain, comment); err != nil {
				return err
			}
		}
	}

	return nil
}

func ensureBridgeAddr(br netlink.Link, family int, ipn *net.IPNet, forceAddress bool) error {
	addrs, err := netlink.AddrList(br, family)
	if err != nil && err != syscall.ENOENT {
		return fmt.Errorf("could not get list of IP addresses: %v", err)
	}

	ipnStr := ipn.String()
	for _, a := range addrs {

		// string comp is actually easiest for doing IPNet comps
		if a.IPNet.String() == ipnStr {
			return nil
		}

		// Multiple IPv6 addresses are allowed on the bridge if the
		// corresponding subnets do not overlap. For IPv4 or for
		// overlapping IPv6 subnets, reconfigure the IP address if
		// forceAddress is true, otherwise throw an error.
		if family == netlink.FAMILY_V4 || a.IPNet.Contains(ipn.IP) || ipn.Contains(a.IPNet.IP) {
			if forceAddress {
				if err = deleteBridgeAddr(br, a.IPNet); err != nil {
					return err
				}
			} else {
				return fmt.Errorf("%q already has an IP address different from %v", br.Attrs().Name, ipnStr)
			}
		}
	}

	addr := &netlink.Addr{IPNet: ipn, Label: ""}
	if err := netlink.AddrAdd(br, addr); err != nil {
		return fmt.Errorf("could not add IP address to %q: %v", br.Attrs().Name, err)
	}

	// Set the bridge's MAC to itself. Otherwise, the bridge will take the
	// lowest-numbered mac on the bridge, and will change as ifs churn
	if err := netlink.LinkSetHardwareAddr(br, br.Attrs().HardwareAddr); err != nil {
		return fmt.Errorf("could not set bridge's mac: %v", err)
	}

	return nil
}

func deleteBridgeAddr(br netlink.Link, ipn *net.IPNet) error {
	addr := &netlink.Addr{IPNet: ipn, Label: ""}

	if err := netlink.AddrDel(br, addr); err != nil {
		return fmt.Errorf("could not remove IP address from %q: %v", br.Attrs().Name, err)
	}

	return nil
}
