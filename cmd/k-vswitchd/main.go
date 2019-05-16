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
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/coreos/go-iptables/iptables"
	kvswitch "github.com/k-vswitch/k-vswitch/apis/generated/clientset/versioned"
	kvswitchinformer "github.com/k-vswitch/k-vswitch/apis/generated/informers/externalversions"
	kvswitchv1alpha1 "github.com/k-vswitch/k-vswitch/apis/kvswitch/v1alpha1"
	"github.com/k-vswitch/k-vswitch/connection"
	"github.com/k-vswitch/k-vswitch/controllers/openflow"
	"github.com/vishvananda/netlink"

	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

const (
	cniConfigPath           = "/etc/cni/net.d/10-k-vswitch.json"
	bridgeName              = "k-vswitch0"
	hostLocalPort           = "host-local"
	vxlanPort               = "vxlan0"
	defaultControllerTarget = "tcp:127.0.0.1:6653"
)

func main() {
	klog.InitFlags(flag.CommandLine)
	klog.Info("starting k-vswitch")

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("error create in cluster config: %v", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("error getting kubernetes client: %v", err)
		os.Exit(1)
	}

	kvswitchClientset, err := kvswitch.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("error getting k-vswitch clientset: %v", err)
		os.Exit(1)
	}

	// Get the current node's name. We're going to assume this was passed
	// via an env var called NODE_NAME for now
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		klog.Error("env variable NODE_NAME is required")
		os.Exit(1)
	}

	curNode, err := clientset.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get current node resource: %v", err)
		os.Exit(1)
	}

	if curNode.Spec.PodCIDR == "" {
		klog.Errorf("node %q has no pod CIDR assigned, ensure IPAM is enabled on the Kubernetes control plane", curNode.Name)
		os.Exit(1)
	}

	err = installCNIConf(curNode.Spec.PodCIDR)
	if err != nil {
		klog.Errorf("failed to install CNI: %v", err)
		os.Exit(1)
	}

	vswitchConfig, err := waitForVSwitchConfig(kvswitchClientset, curNode.Name)
	if err != nil {
		klog.Errorf("error getting vswitch config for node: %v", err)
		os.Exit(1)
	}

	podCIDR := vswitchConfig.Spec.PodCIDR
	clusterCIDR := vswitchConfig.Spec.ClusterCIDR
	serviceCIDR := vswitchConfig.Spec.ServiceCIDR

	err = setupBridgeIfNotExists()
	if err != nil {
		klog.Errorf("failed to setup OVS bridge: %v", err)
		os.Exit(1)
	}

	err = setupHostLocalInternalPort()
	if err != nil {
		klog.Errorf("failed to setup host-local port: %v", err)
		os.Exit(1)
	}

	err = setupVxLANPort()
	if err != nil {
		klog.Errorf("failed to setup vxlan port: %v", err)
		os.Exit(1)
	}

	hostLocalLink, err := netlink.LinkByName(hostLocalPort)
	if err != nil {
		klog.Errorf("failed to get bridge %q, err: %v", bridgeName, err)
		os.Exit(1)
	}

	addr, err := netlinkAddrForCIDR(clusterCIDR, podCIDR)
	if err != nil {
		klog.Errorf("failed to get netlink addr for CIDR %q, err: %v", podCIDR, err)
		os.Exit(1)
	}

	if err := netlink.AddrReplace(hostLocalLink, addr); err != nil {
		klog.Errorf("could not add addr %q to bridge %q, err: %v",
			podCIDR, bridgeName, err)
		os.Exit(1)
	}

	if err := setControllerTarget(); err != nil {
		klog.Errorf("failed to setup controller: %v", err)
		os.Exit(1)
	}

	if err := setSecureFailMode(); err != nil {
		klog.Errorf("failed to set fail-mode to 'secure': %v", err)
		os.Exit(1)
	}

	if err := setupModulesAndSysctls(); err != nil {
		klog.Errorf("failed to setup sysctls: %v", err)
		os.Exit(1)
	}

	if err := setupBridgeForwarding(podCIDR, clusterCIDR, serviceCIDR); err != nil {
		klog.Errorf("failed to setup bridge forwarding: %v", err)
		os.Exit(1)
	}

	hostLocalInterface, err := net.InterfaceByName(hostLocalPort)
	if err != nil {
		klog.Errorf("error getting %q: err: %v", bridgeName, err)
		os.Exit(1)
	}

	stopCh := make(chan struct{})

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-term
		close(stopCh)
	}()

	informerFactory := informers.NewSharedInformerFactory(clientset, 0)
	nodeInformer := informerFactory.Core().V1().Nodes()
	podInformer := informerFactory.Core().V1().Pods()

	kvswitchInformerFactory := kvswitchinformer.NewSharedInformerFactory(kvswitchClientset, 0)
	vswitchInformer := kvswitchInformerFactory.Kvswitch().V1alpha1().VSwitchConfigs()

	connectionManager, err := connection.NewOFConnect()
	if err != nil {
		klog.Errorf("error starting open flow connection manager: %v", err)
		os.Exit(1)
	}

	c, err := openflow.NewController(connectionManager, nodeInformer, podInformer, vswitchInformer, bridgeName, hostLocalInterface.HardwareAddr.String(), curNode.Name, podCIDR, clusterCIDR)
	if err != nil {
		klog.Errorf("error initializing open flow controller: %v", err)
		os.Exit(1)
	}

	vswitchInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.OnAddVSwitch,
		UpdateFunc: c.OnUpdateVSwitch,
		DeleteFunc: c.OnDeleteVSwitch,
	})
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.OnAddPod,
		UpdateFunc: c.OnUpdatePod,
		DeleteFunc: c.OnDeletePod,
	})

	err = c.Initialize()
	if err != nil {
		klog.Errorf("error initializing openflow controller: %v", err)
		os.Exit(1)
	}

	informerFactory.WaitForCacheSync(stopCh)
	kvswitchInformerFactory.WaitForCacheSync(stopCh)

	informerFactory.Start(stopCh)
	kvswitchInformerFactory.Start(stopCh)

	go connectionManager.ProcessQueue()
	go connectionManager.Serve()
	go c.Run(stopCh)

	// TODO: use context to gracefully handle shutdown
	<-stopCh
}

func waitForVSwitchConfig(kvswitchClient kvswitch.Interface, nodeName string) (*kvswitchv1alpha1.VSwitchConfig, error) {
	waitDuration := time.Minute
	pollInterval := 5 * time.Second

	for start := time.Now(); time.Since(start) < waitDuration; {
		vswitchConfig, err := kvswitchClient.KvswitchV1alpha1().VSwitchConfigs().Get(nodeName, metav1.GetOptions{})
		if err == nil {
			return vswitchConfig, nil
		}
		if !apierr.IsNotFound(err) {
			return nil, err
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("vswitch config with name %q not found", nodeName)
}

func netlinkAddrForCIDR(clusterCIDR, podCIDR string) (*netlink.Addr, error) {
	_, podIPNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return nil, err
	}

	gw := ip.NextIP(podIPNet.IP.Mask(podIPNet.Mask))

	_, clusterIPNet, err := net.ParseCIDR(clusterCIDR)
	if err != nil {
		return nil, err
	}

	return &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   gw,
			Mask: clusterIPNet.Mask,
		},
		Label: "",
	}, nil
}

// installCNIConf adds the CNI config file given the pod cidr of the node
func installCNIConf(podCIDR string) error {
	conf := fmt.Sprintf(`{
	"name": "k-vswitch-cni",
	"type": "k-vswitch-cni",
	"bridge": "k-vswitch0",
	"isGateway": true,
	"isDefaultGateway": true,
	"ipam": {
		"type": "host-local",
		"subnet": "%s"
	}
}`, podCIDR)

	return ioutil.WriteFile(cniConfigPath, []byte(conf), 0644)
}

func setupBridgeIfNotExists() error {
	command := []string{
		"--may-exist", "add-br", bridgeName,
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to setup OVS bridge %q, err: %v, output: %q",
			bridgeName, err, string(out))
	}

	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("could not lookup %q: %v", bridgeName, err)
	}

	if err := netlink.LinkSetUp(br); err != nil {
		return fmt.Errorf("failed to bring bridge %q up: %v", bridgeName, err)
	}

	return nil
}

func setupHostLocalInternalPort() error {
	command := []string{
		"--may-exist", "add-port", bridgeName, hostLocalPort,
		"--", "set", "Interface", hostLocalPort, "type=internal",
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to setup host-local port, err: %v, output: %q", err, out)
	}

	hostLocal, err := netlink.LinkByName(hostLocalPort)
	if err != nil {
		return fmt.Errorf("could not lookup %q: %v", hostLocalPort, err)
	}

	if err := netlink.LinkSetUp(hostLocal); err != nil {
		return fmt.Errorf("failed to bring bridge %q up: %v", hostLocalPort, err)
	}

	return nil
}

func setupVxLANPort() error {
	command := []string{
		"--may-exist", "add-port", bridgeName, vxlanPort,
		"--", "set", "Interface", vxlanPort, "type=vxlan",
		"option:remote_ip=flow", "option:key=flow",
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to setup vxlan port %q, err: %v, out: %q", vxlanPort, err, string(out))
	}

	return nil
}

func setSecureFailMode() error {
	command := []string{
		"set-fail-mode", bridgeName, "secure",
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set fail mode for bridge %q to 'secure', err: %v, out: %q",
			bridgeName, err, string(out))
	}

	return nil
}

func setControllerTarget() error {
	command := []string{
		"set-controller", bridgeName, defaultControllerTarget,
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set controller target for bridge %q, err: %v, out: %q",
			bridgeName, err, string(out))
	}

	return nil
}

func setupBridgeForwarding(podCIDR, clusterCIDR, serviceCIDR string) error {
	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		return err
	}

	rules := []string{"-o", hostLocalPort, "-j", "ACCEPT"}
	err = ipt.AppendUnique("filter", "FORWARD", rules...)
	if err != nil {
		return err
	}

	rules = []string{"-i", hostLocalPort, "-j", "ACCEPT"}
	err = ipt.AppendUnique("filter", "FORWARD", rules...)
	if err != nil {
		return err
	}

	rules = []string{"-s", podCIDR, "!", "-o", hostLocalPort, "-j", "MASQUERADE"}
	err = ipt.AppendUnique("nat", "POSTROUTING", rules...)
	if err != nil {
		return err
	}

	rules = []string{"!", "-d", clusterCIDR, "-m", "comment", "--comment", "k-vswitch: SNAT for outbound traffic from cluster CIDR", "-m", "addrtype", "!", "--dst-type", "LOCAL", "-j", "MASQUERADE"}
	err = ipt.AppendUnique("nat", "POSTROUTING", rules...)
	if err != nil {
		return err
	}

	rules = []string{"!", "-d", serviceCIDR, "-m", "comment", "--comment", "k-vswitch: SNAT for outbound traffic from service CIDR", "-m", "addrtype", "!", "--dst-type", "LOCAL", "-j", "MASQUERADE"}
	err = ipt.AppendUnique("nat", "POSTROUTING", rules...)
	if err != nil {
		return err
	}

	return nil
}

func setupModulesAndSysctls() error {
	if out, err := exec.Command("modprobe", "br_netfilter").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable br_netfilter module, err: %v, out: %q", err, string(out))
	}

	if err := ioutil.WriteFile("/proc/sys/net/bridge/bridge-nf-call-iptables", []byte(strconv.Itoa(1)), 0640); err != nil {
		return fmt.Errorf("failed to set /proc/sys/net/bridge/bridge-nf-call-iptables, err: %v", err)
	}

	if err := ioutil.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte(strconv.Itoa(1)), 0640); err != nil {
		return fmt.Errorf("failed to set /proc/sys/net/ipv4/ip_forward, err: %v", err)
	}

	return nil
}
