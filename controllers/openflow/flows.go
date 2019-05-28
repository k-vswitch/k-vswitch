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

package openflow

import (
	"fmt"
	"net"
	"time"

	"github.com/k-vswitch/k-vswitch/apis/kvswitch/v1alpha1"
	"github.com/k-vswitch/k-vswitch/flows"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
)

const (
	tableClassification         = 0
	tableLocalARP               = 10
	tableL3Forwarding           = 20
	tableL2Forwarding           = 30
	tableL3Rewrites             = 40
	tableL2Rewrites             = 50
	tableNetworkPoliciesIngress = 60
	tableNetworkPoliciesEgress  = 70
	tableProxy                  = 80
	tableNAT                    = 90
	tableAudit                  = 100
)

func (c *controller) syncFlows() error {
	c.flowsLock.Lock()
	defer c.flowsLock.Unlock()

	startTime := time.Now()

	// reset the flows buffer before each sync
	c.flows.Reset()

	err := c.defaultFlows()
	if err != nil {
		return fmt.Errorf("error adding default flows: %v", err)
	}

	vswitchCfgs, err := c.vswitchLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing vswitch configs: %v", err)
	}

	for _, vswitchCfg := range vswitchCfgs {
		err = c.flowsForVSwitch(vswitchCfg)
		if err != nil {
			klog.Warningf("error getting flows for vswitch %q, err: %v", vswitchCfg.Name, err)
		}
	}

	pods, err := c.podLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing pods: %v", err)
	}

	for _, pod := range pods {
		err = c.flowsForPod(pod)
		if err != nil {
			klog.Warningf("error getting flows for pod %q, err: %v", pod.Name, err)
		}
	}

	err = c.flows.SyncFlows(c.bridgeName)
	if err != nil {
		return fmt.Errorf("error syncing flows: %v", err)
	}

	klog.V(5).Infof("full sync for flows took %s", time.Since(startTime).String())
	return nil
}

func (c *controller) defaultFlows() error {
	// default to host-local if none of the following classifer flows match
	flow := flows.NewFlow().WithTable(tableClassification).
		WithPriority(0).
		WithOutputPort(c.hostLocalOFPort)
	c.flows.AddFlow(flow)

	// any IP traffic for the gatetway should go back out host-local port
	// with a dl destination rewrite to account for traffic from the overlay
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(500).WithProtocol("ip").
		WithIPDest(c.gatewayIP).WithModDlDest(c.hostLocalMAC).
		WithOutputPort(c.hostLocalOFPort)
	c.flows.AddFlow(flow)

	// traffic in the local pod CIDR should go straight to tableL2Rewrites
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(300).WithProtocol("ip").WithIPDest(c.podCIDR).
		WithResubmit(tableL2Rewrites)
	c.flows.AddFlow(flow)

	// remaining IP traffic in the cluster CIDR should go to tableL3Forwarding
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(100).WithProtocol("ip").
		WithIPDest(c.clusterCIDR).WithResubmit(tableL3Forwarding)
	c.flows.AddFlow(flow)

	// arp for the gateway IP should go back to host-local
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(500).WithProtocol("arp").
		WithArpDest(c.gatewayIP).WithOutputPort(c.hostLocalOFPort)
	c.flows.AddFlow(flow)

	// arp for the local pod CIDR should go straigh to L2 rewrite
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(300).WithProtocol("arp").
		WithArpDest(c.podCIDR).WithResubmit(tableL2Rewrites)
	c.flows.AddFlow(flow)

	// arp traffic towards the cluster CIDR should go to tableL2Forwarding
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(100).WithProtocol("arp").
		WithArpDest(c.clusterCIDR).WithResubmit(tableL2Forwarding)
	c.flows.AddFlow(flow)

	return nil
}

func (c *controller) flowsForVSwitch(vswitch *v1alpha1.VSwitchConfig) error {
	isCurrentNode := false
	if vswitch.Name == c.nodeName {
		isCurrentNode = true
	}

	podCIDR := vswitch.Spec.PodCIDR

	//
	// flow for table 30 - L3 Forwarding
	//

	// If pod cidr is not for current node, output to overlay port
	// If pod cidr is for current node, go straight to L2 rewrites
	if !isCurrentNode {
		flow := flows.NewFlow().WithTable(tableL3Forwarding).
			WithPriority(150).WithProtocol("ip").
			WithIPDest(podCIDR).WithTunnelDest(vswitch.Spec.OverlayIP).
			WithOutputPort(c.overlayOFPort)
		c.flows.AddFlow(flow)

		flow = flows.NewFlow().WithTable(tableL2Forwarding).
			WithPriority(150).WithProtocol("arp").
			WithArpDest(podCIDR).WithTunnelDest(vswitch.Spec.OverlayIP).
			WithOutputPort(c.overlayOFPort)
		c.flows.AddFlow(flow)

	} else {
		flow := flows.NewFlow().WithTable(tableL3Forwarding).
			WithPriority(100).WithProtocol("ip").WithIPDest(podCIDR).
			WithResubmit(tableL2Rewrites)
		c.flows.AddFlow(flow)

		flow = flows.NewFlow().WithTable(tableL2Forwarding).
			WithPriority(100).WithProtocol("arp").WithArpDest(podCIDR).
			WithResubmit(tableL2Rewrites)
		c.flows.AddFlow(flow)
	}

	// directly output to host-local port if dest address is the current node,
	// otherwise route back to overlay
	nodeIP := vswitch.Spec.OverlayIP
	if isCurrentNode {
		// traffic towards the local node IP from local pod CIDR
		// should go through host local port
		flow := flows.NewFlow().WithTable(tableClassification).
			WithPriority(100).WithProtocol("arp").WithArpDest(nodeIP).
			WithArpSrc(podCIDR).WithOutputPort(c.hostLocalOFPort)
		c.flows.AddFlow(flow)

		flow = flows.NewFlow().WithTable(tableClassification).
			WithPriority(100).WithProtocol("ip").
			WithIPDest(nodeIP).WithIPSrc(podCIDR).
			WithModDlDest(c.hostLocalMAC).WithOutputPort(c.hostLocalOFPort)
		c.flows.AddFlow(flow)

		// traffic towards the local node IP from the rest of the cluster
		// should go through cluster wide port
		flow = flows.NewFlow().WithTable(tableClassification).
			WithPriority(50).WithProtocol("arp").WithArpDest(nodeIP).
			WithOutputPort(c.clusterWideOFPort)
		c.flows.AddFlow(flow)

		flow = flows.NewFlow().WithTable(tableClassification).
			WithPriority(50).WithProtocol("ip").WithIPDest(nodeIP).
			WithModDlDest(c.clusterWideMAC).WithOutputPort(c.clusterWideOFPort)
		c.flows.AddFlow(flow)
	} else {
		// for IP traffic to remote node IP, send to table L3 forwarding
		flow := flows.NewFlow().WithTable(tableClassification).
			WithPriority(50).WithProtocol("ip").WithIPDest(nodeIP).
			WithResubmit(tableL3Forwarding)
		c.flows.AddFlow(flow)

		// for ARP traffic to remote node IP, send to L2 forwarding
		flow = flows.NewFlow().WithTable(tableClassification).
			WithPriority(50).WithProtocol("arp").WithArpDest(nodeIP).
			WithResubmit(tableL2Forwarding)
		c.flows.AddFlow(flow)

		// send IP traffic to remote node IP through tunnel
		flow = flows.NewFlow().WithTable(tableL3Forwarding).
			WithPriority(100).WithProtocol("ip").WithIPDest(nodeIP).
			WithTunnelDest(nodeIP).WithOutputPort(c.overlayOFPort)
		c.flows.AddFlow(flow)

		// send ARP traffic to node IP through tunnel
		flow = flows.NewFlow().WithTable(tableL2Forwarding).
			WithPriority(100).WithProtocol("arp").WithArpDest(nodeIP).
			WithTunnelDest(nodeIP).WithOutputPort(c.overlayOFPort)
		c.flows.AddFlow(flow)
	}

	return nil
}

func (c *controller) isLocalIP(ip string) (bool, error) {
	_, ipnet, err := net.ParseCIDR(c.podCIDR)
	if err != nil {
		return false, err
	}

	return ipnet.Contains(net.ParseIP(ip)), nil
}

func (c *controller) flowsForPod(pod *corev1.Pod) error {
	podIP := pod.Status.PodIP
	if podIP == "" {
		return nil
	}

	local, err := c.isLocalIP(podIP)
	if err != nil {
		return fmt.Errorf("error checking if IP %q is local: %v", podIP, err)
	}

	if !local {
		return nil
	}

	podPortInfo, err := c.portCache.GetPortInfo(pod)
	if err != nil {
		return fmt.Errorf("error checking port info for pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}

	podMacAddr := podPortInfo.mac
	ofport := podPortInfo.ofport

	flow := flows.NewFlow().WithTable(tableL2Rewrites).WithPriority(100).
		WithProtocol("ip").WithIPDest(podIP).WithModDlDest(podMacAddr).
		WithOutputPort(ofport)
	c.flows.AddFlow(flow)

	flow = flows.NewFlow().WithTable(tableL2Rewrites).WithPriority(100).
		WithProtocol("arp").WithArpDest(podIP).WithOutputPort(ofport)
	c.flows.AddFlow(flow)

	return nil
}

func (c *controller) podNeedsUpdate(pod *corev1.Pod) (bool, error) {
	podIP := pod.Status.PodIP
	if podIP == "" {
		return false, nil
	}

	local, err := c.isLocalIP(podIP)
	if err != nil {
		return false, fmt.Errorf("error checking if IP %q is local: %v", podIP, err)
	}

	return local, nil
}

func (c *controller) OnAddVSwitch(obj interface{}) {
	err := c.syncFlows()
	if err != nil {
		klog.Errorf("error syncing flows: %v", err)
		return
	}
}

func (c *controller) OnUpdateVSwitch(oldObj, newObj interface{}) {
	err := c.syncFlows()
	if err != nil {
		klog.Errorf("error syncing flows: %v", err)
		return
	}
}

func (c *controller) OnDeleteVSwitch(obj interface{}) {
}

func (c *controller) OnAddPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	needsUpdate, err := c.podNeedsUpdate(pod)
	if err != nil {
		klog.Errorf("error checking if pod needs update: %v", err)
		return
	}

	if !needsUpdate {
		return
	}

	if err := c.syncFlows(); err != nil {
		klog.Errorf("error syncing pod: %v", err)
		return
	}
}

func (c *controller) OnUpdatePod(oldObj, newObj interface{}) {
	pod, ok := newObj.(*corev1.Pod)
	if !ok {
		return
	}

	needsUpdate, err := c.podNeedsUpdate(pod)
	if err != nil {
		klog.Errorf("error checking if pod needs update: %v", err)
		return
	}

	if !needsUpdate {
		return
	}

	if err := c.syncFlows(); err != nil {
		klog.Errorf("error syncing pod : %v", err)
		return
	}
}

func (c *controller) OnDeletePod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	c.portCache.DelPortInfo(pod)
}
