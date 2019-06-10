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
	"strings"
	"time"

	"github.com/k-vswitch/k-vswitch/apis/kvswitch/v1alpha1"
	"github.com/k-vswitch/k-vswitch/flows"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
)

const (
	tableEntry           = 0
	tableNetworkPolicies = 10
	tableClassification  = 20
	tableL3Forwarding    = 30
	tableL2Forwarding    = 40
	tableL3Rewrites      = 50
	tableL2Rewrites      = 60
	tableProxy           = 70
	tableNAT             = 80
	tableAudit           = 90
	tableLocalARP        = 100

	// all pods have a fixed mac addr
	podMacAddr = "aa:bb:cc:dd:ee:ff"
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

	netPols, err := c.netPolLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error getting network policies: %v", err)
	}

	for _, netPol := range netPols {
		err = c.flowsForNetworkPolicy(netPol)
		if err != nil {
			klog.Warningf("error getting flows for network policy %q, err: %v",
				netPol.Name, err)
		}
	}

	err = c.flows.SyncFlows(c.bridgeName)
	if err != nil {
		return fmt.Errorf("error syncing flows: %v", err)
	}

	klog.V(2).Infof("full sync for flows took %s", time.Since(startTime).String())
	return nil
}

func (c *controller) defaultFlows() error {
	// table entry, for now, always go directly to network policies
	flow := flows.NewFlow().WithTable(tableEntry).
		WithPriority(1000).WithActionResubmit(tableNetworkPolicies)
	c.flows.AddFlow(flow)

	// table network policy defaults to table classification
	// higher priority rules will get added later as network policies are applied
	flow = flows.NewFlow().WithTable(tableNetworkPolicies).
		WithPriority(0).WithActionResubmit(tableClassification)
	c.flows.AddFlow(flow)

	// default to local if none of the following classifier flows match
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(0).
		WithActionLocal()
	c.flows.AddFlow(flow)

	// any IP traffic for the gateway should go to the bridge port
	// with a dl destination rewrite to account for traffic from the overlay
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(500).WithProtocol("ip").
		WithIPDest(c.gatewayIP).WithActionLocal()
	c.flows.AddFlow(flow)

	// traffic in the local pod CIDR should go straight to tableL2Rewrites
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(300).WithProtocol("ip").WithIPDest(c.podCIDR).
		WithActionResubmit(tableL2Rewrites)
	c.flows.AddFlow(flow)

	// remaining IP traffic in the cluster CIDR should go to tableL3Forwarding
	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(100).WithProtocol("ip").
		WithIPDest(c.clusterCIDR).WithActionResubmit(tableL3Forwarding)
	c.flows.AddFlow(flow)

	flow = flows.NewFlow().WithTable(tableClassification).
		WithPriority(100).WithProtocol("arp").WithActionResubmit(tableLocalARP)
	c.flows.AddFlow(flow)

	// arp responder for gateway IP
	rawFlow := fmt.Sprintf("table=%d priority=500 dl_type=0x0806 arp_tpa=%s actions=move:NXM_OF_ETH_SRC[]->NXM_OF_ETH_DST[],mod_dl_src:%s,load:0x2->NXM_OF_ARP_OP[],move:NXM_NX_ARP_SHA[]->NXM_NX_ARP_THA[],move:NXM_OF_ARP_SPA[]->NXM_OF_ARP_TPA[],set_field:%s->arp_sha,set_field:%s->arp_spa,in_port",
		tableLocalARP, c.gatewayIP, c.bridgeMAC, c.bridgeMAC, c.gatewayIP)
	flow = flows.NewFlow().WithRaw(rawFlow)
	c.flows.AddFlow(flow)

	// ARP responder for cluster CIDR
	rawFlow = fmt.Sprintf("table=%d priority=400 dl_type=0x0806 arp_tpa=%s actions=move:NXM_OF_ETH_SRC[]->NXM_OF_ETH_DST[],mod_dl_src:%s,load:0x2->NXM_OF_ARP_OP[],set_field:%s->arp_sha,move:NXM_OF_ARP_TPA[]->NXM_OF_ARP_SPA[],move:NXM_NX_ARP_SHA[]->NXM_NX_ARP_THA[],move:NXM_OF_ARP_SPA[]->NXM_OF_ARP_TPA[],in_port",
		tableLocalARP, c.clusterCIDR, podMacAddr, podMacAddr)
	flow = flows.NewFlow().WithRaw(rawFlow)
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
			WithIPDest(podCIDR).WithActionTunnelDest(vswitch.Spec.OverlayIP).
			WithActionOutputPort(c.overlayOFPort)
		c.flows.AddFlow(flow)

	} else {
		flow := flows.NewFlow().WithTable(tableL3Forwarding).
			WithPriority(100).WithProtocol("ip").WithIPDest(podCIDR).
			WithActionResubmit(tableL2Rewrites)
		c.flows.AddFlow(flow)
	}

	// directly output to local bridge if dest address is the current node,
	// otherwise route back to overlay
	nodeIP := vswitch.Spec.OverlayIP
	if isCurrentNode {
		flow := flows.NewFlow().WithTable(tableClassification).
			WithPriority(100).WithProtocol("ip").
			WithIPDest(nodeIP).WithIPSrc(podCIDR).
			WithActionModDlDest(c.bridgeMAC).WithActionLocal()
		c.flows.AddFlow(flow)

		flow = flows.NewFlow().WithTable(tableClassification).
			WithPriority(50).WithProtocol("ip").WithIPDest(nodeIP).
			WithActionModDlDest(c.clusterWideMAC).WithActionOutputPort(c.clusterWideOFPort)
		c.flows.AddFlow(flow)
	} else {
		// for IP traffic to remote node IP, send to table L3 forwarding
		flow := flows.NewFlow().WithTable(tableClassification).
			WithPriority(50).WithProtocol("ip").WithIPDest(nodeIP).
			WithActionResubmit(tableL3Forwarding)
		c.flows.AddFlow(flow)

		// send IP traffic to remote node IP through tunnel
		flow = flows.NewFlow().WithTable(tableL3Forwarding).
			WithPriority(100).WithProtocol("ip").WithIPDest(nodeIP).
			WithActionTunnelDest(nodeIP).WithActionOutputPort(c.overlayOFPort)
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

	ofport := podPortInfo.ofport

	flow := flows.NewFlow().WithTable(tableL2Rewrites).WithPriority(100).
		WithProtocol("ip").WithIPDest(podIP).WithActionModDlDest(podMacAddr).
		WithActionOutputPort(ofport)
	c.flows.AddFlow(flow)

	// arp responder per pod IP
	rawFlow := fmt.Sprintf("table=%d priority=500 dl_type=0x0806 arp_tpa=%s actions=move:NXM_OF_ETH_SRC[]->NXM_OF_ETH_DST[],mod_dl_src:%s,load:0x2->NXM_OF_ARP_OP[],move:NXM_NX_ARP_SHA[]->NXM_NX_ARP_THA[],move:NXM_OF_ARP_SPA[]->NXM_OF_ARP_TPA[],set_field:%s->arp_sha,set_field:%s->arp_spa,in_port",
		tableLocalARP, podIP, podMacAddr, podMacAddr, podIP)
	flow = flows.NewFlow().WithRaw(rawFlow)
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

func (c *controller) flowsForNetworkPolicy(netPol *netv1.NetworkPolicy) error {
	// given a network polices, find all the associated local IPs based on
	// the pod selector in the same network namespace as the network policy
	matchLabels := labels.Set(netPol.Spec.PodSelector.MatchLabels)
	pods, err := c.podLister.Pods(netPol.Namespace).List(matchLabels.AsSelector())
	if err != nil {
		return fmt.Errorf("error listing pods by pod selector: %v", err)
	}

	localPodIPs := []string{}
	for _, pod := range pods {
		podIP := pod.Status.PodIP
		if podIP == "" {
			// skip pods that do not have an IP assigned yet
			continue
		}

		// if the pod is not local, then we shouldn't try to apply
		// any network policies rules on this host
		local, err := c.isLocalIP(podIP)
		if err != nil {
			klog.Warningf("error checking if podIP %q is local", podIP)
			continue
		}

		if !local {
			continue
		}

		localPodIPs = append(localPodIPs, podIP)
	}

	err = c.flowsForIngressNetworkPolicy(netPol, localPodIPs)
	if err != nil {
		klog.Errorf("error getting ingress policy flows for policy %q, err: %v", netPol.Name, err)
	}

	err = c.flowsForEgressNetworkPolicy(netPol, localPodIPs)
	if err != nil {
		klog.Errorf("error getting ingress policy flows for policy %q, err: %v", netPol.Name, err)
	}

	return nil
}

func (c *controller) flowsForIngressNetworkPolicy(netPol *netv1.NetworkPolicy, localPodIPs []string) error {
	// for each pod, add low priority flow to drop traffic that does not match
	// any ingress network policy rules
	for _, podIP := range localPodIPs {
		// all traffic ingress towards the local pod IPs that match the pod
		// selector match should have default drop action flow
		// flows added as part of the pod selector match will have high priority
		flow := flows.NewFlow().WithTable(tableNetworkPolicies).
			WithPriority(100).WithProtocol("ip").WithIPDest(podIP).WithActionDrop()
		c.flows.AddFlow(flow)
	}

	ingressRules := netPol.Spec.Ingress
	for _, ing := range ingressRules {
		// for the case where an ingress rule has no From peers, we allow
		// all ingress traffic specified by the ports, if no ports
		// then we allow all "ip" traffic
		if len(ing.From) == 0 {
			// allow all ingress traffic based on ports
			// i.e. ignore src address
			for _, destIP := range localPodIPs {
				// if no ports specified, allow traffic with src/dest match on all ports
				if len(ing.Ports) == 0 {
					flow := flows.NewFlow().WithTable(tableNetworkPolicies).
						WithPriority(1000).WithProtocol("ip").WithIPDest(destIP).
						WithActionResubmit(tableClassification)
					c.flows.AddFlow(flow)

					continue
				}

				for _, port := range ing.Ports {
					portNum := port.Port.IntValue()
					protocol := strings.ToLower(string(*port.Protocol))

					if protocol == "tcp" {
						flow := flows.NewFlow().WithTable(tableNetworkPolicies).
							WithPriority(1000).WithProtocol("tcp").WithTCPDestPort(portNum).
							WithIPDest(destIP).WithActionResubmit(tableClassification)
						c.flows.AddFlow(flow)
					}

					if protocol == "udp" {
						flow := flows.NewFlow().WithTable(tableNetworkPolicies).
							WithPriority(1000).WithProtocol("udp").WithUDPDestPort(portNum).
							WithIPDest(destIP).WithActionResubmit(tableClassification)
						c.flows.AddFlow(flow)
					}

					// TODO: sctp
				}
			}

			continue
		}

		// for each from block, go through all the possibe IPs
		// and allow the port/protocol from ports
		for _, from := range ing.From {
			ipBlock := from.IPBlock
			if ipBlock != nil {
				srcIP := ipBlock.CIDR
				for _, destIP := range localPodIPs {
					flow := flows.NewFlow().WithTable(tableNetworkPolicies).WithPriority(1000).
						WithProtocol("ip").WithIPSrc(srcIP).WithIPDest(destIP).
						WithActionResubmit(tableClassification)
					c.flows.AddFlow(flow)
				}

				// drop flow with higher priority for CIRDs in Except block
				for _, srcIP := range ipBlock.Except {
					for _, destIP := range localPodIPs {
						flow := flows.NewFlow().WithTable(tableNetworkPolicies).WithPriority(1100).
							WithProtocol("ip").WithIPSrc(srcIP).WithIPDest(destIP).
							WithActionDrop()
						c.flows.AddFlow(flow)
					}
				}

				// according to NetworkPolicy API, if ipBlock is set, then any other
				// field cannot be set, so continue to next FROM field.
				continue
			}

			// pod / namespace selector matches

			// at this point pod selector should be set,
			// but we ignore this FROM block entirely if it's not
			if from.PodSelector == nil {
				continue
			}

			// default to namespace of the network policy resource
			namespaces := []string{netPol.Namespace}
			// if namespace selector is set, use the namespace list from selecttor
			if from.NamespaceSelector != nil {
				// TODO: account for empty match label list?
				matchLabels := labels.Set(from.NamespaceSelector.MatchLabels)
				nsList, err := c.nsLister.List(matchLabels.AsSelector())
				if err != nil {
					klog.Errorf("error listing namespaces: %v", err)
					continue
				}

				if len(nsList) > 0 {
					// if namespace selector is set, reset the default list
					namespaces = []string{}
					for _, ns := range nsList {
						namespaces = append(namespaces, ns.Name)
					}
				}
			}

			for _, namespace := range namespaces {
				// if match label is empty, then allow all
				// otherwise, filter pods based on the match labels
				var labelSelector labels.Selector
				if len(from.PodSelector.MatchLabels) == 0 {
					labelSelector = labels.Everything()
				} else {
					podMatchLabel := labels.Set(from.PodSelector.MatchLabels)
					labelSelector = podMatchLabel.AsSelector()
				}

				pods, err := c.podLister.Pods(namespace).List(labelSelector)
				if err != nil {
					klog.Errorf("error listing pods in namespace %q, err: %v", namespace, err)
					continue
				}

				// take list of source IPs and create flows
				// allow traffic from all these IPs, default drop with lowest priority
				srcIPs := getPodIPs(pods)
				for _, destIP := range localPodIPs {
					for _, srcIP := range srcIPs {
						// if no ports specified, allow traffic with src/dest match on all ports
						if len(ing.Ports) == 0 {
							flow := flows.NewFlow().WithTable(tableNetworkPolicies).
								WithPriority(1000).WithProtocol("ip").WithIPSrc(srcIP).WithIPDest(destIP).
								WithActionResubmit(tableClassification)
							c.flows.AddFlow(flow)

							continue
						}

						for _, port := range ing.Ports {
							portNum := port.Port.IntValue()
							protocol := strings.ToLower(string(*port.Protocol))

							if protocol == "tcp" {
								flow := flows.NewFlow().WithTable(tableNetworkPolicies).
									WithPriority(1000).WithProtocol("tcp").WithTCPDestPort(portNum).
									WithIPSrc(srcIP).WithIPDest(destIP).WithActionResubmit(tableClassification)
								c.flows.AddFlow(flow)
							}

							if protocol == "udp" {
								flow := flows.NewFlow().WithTable(tableNetworkPolicies).
									WithPriority(1000).WithProtocol("udp").WithUDPDestPort(portNum).
									WithIPSrc(srcIP).WithIPDest(destIP).WithActionResubmit(tableClassification)
								c.flows.AddFlow(flow)
							}

							// TODO: sctp
						}
					}
				}
			}
		}
	}

	return nil
}

func (c *controller) flowsForEgressNetworkPolicy(netPol *netv1.NetworkPolicy, localPodIPs []string) error {
	// for each pod, add low priority flow to drop traffic that does not match
	// any egress network policy rules
	for _, podIP := range localPodIPs {
		// all traffic egress towards the local pod IPs that match the pod
		// selector should have default drop action flow
		// flows added as part of the pod selector match will have high priority
		flow := flows.NewFlow().WithTable(tableNetworkPolicies).
			WithPriority(100).WithProtocol("ip").WithIPSrc(podIP).WithActionDrop()
		c.flows.AddFlow(flow)
	}

	egressRules := netPol.Spec.Egress
	for _, eg := range egressRules {
		// for the case where an egress rule has no To peers, we allow
		// all egress traffic specified by the ports, if no ports
		// then we allow all "ip" traffic
		if len(eg.To) == 0 {
			// allow all egress traffic based on ports
			// i.e. ignore destination address
			for _, srcIP := range localPodIPs {
				// if no ports specified, allow traffic with src/dest match on all ports
				if len(eg.Ports) == 0 {
					flow := flows.NewFlow().WithTable(tableNetworkPolicies).
						WithPriority(1000).WithProtocol("ip").WithIPSrc(srcIP).
						WithActionResubmit(tableClassification)
					c.flows.AddFlow(flow)

					continue
				}

				for _, port := range eg.Ports {
					portNum := port.Port.IntValue()
					protocol := strings.ToLower(string(*port.Protocol))

					if protocol == "tcp" {
						flow := flows.NewFlow().WithTable(tableNetworkPolicies).
							WithPriority(1000).WithProtocol("tcp").WithTCPDestPort(portNum).
							WithIPSrc(srcIP).WithActionResubmit(tableClassification)
						c.flows.AddFlow(flow)
					}

					if protocol == "udp" {
						flow := flows.NewFlow().WithTable(tableNetworkPolicies).
							WithPriority(1000).WithProtocol("udp").WithUDPDestPort(portNum).
							WithIPSrc(srcIP).WithActionResubmit(tableClassification)
						c.flows.AddFlow(flow)
					}

					// TODO: sctp
				}
			}

			continue
		}

		for _, to := range eg.To {
			ipBlock := to.IPBlock
			if ipBlock != nil {
				destIP := ipBlock.CIDR
				for _, srcIP := range localPodIPs {
					flow := flows.NewFlow().WithTable(tableNetworkPolicies).WithPriority(1000).
						WithProtocol("ip").WithIPSrc(srcIP).WithIPDest(destIP).
						WithActionResubmit(tableClassification)
					c.flows.AddFlow(flow)
				}

				// drop flow with higher priority for CIRDs in Except block
				for _, destIP := range ipBlock.Except {
					for _, srcIP := range localPodIPs {
						flow := flows.NewFlow().WithTable(tableNetworkPolicies).WithPriority(1100).
							WithProtocol("ip").WithIPSrc(srcIP).WithIPDest(destIP).
							WithActionDrop()
						c.flows.AddFlow(flow)
					}
				}

				// according to NetworkPolicy API, if ipBlock is set, then any other
				// field cannot be set, so continue to next FROM field.
				continue
			}

			// pod / namespace selector matches

			// at this point pod selector should be set,
			// but we ignore this FROM block entirely if it's not
			if to.PodSelector == nil {
				continue
			}

			// default to namespace of the network policy resource
			namespaces := []string{netPol.Namespace}
			// if namespace selector is set, use the namespace list from selecttor
			if to.NamespaceSelector != nil {
				// TODO: account for empty match label list?
				matchLabels := labels.Set(to.NamespaceSelector.MatchLabels)
				nsList, err := c.nsLister.List(matchLabels.AsSelector())
				if err != nil {
					klog.Errorf("error listing namespaces: %v", err)
					continue
				}

				if len(nsList) > 0 {
					// if namespace selector is set, reset the default list
					namespaces = []string{}
					for _, ns := range nsList {
						namespaces = append(namespaces, ns.Name)
					}
				}
			}

			for _, namespace := range namespaces {
				// if match label is empty, then allow all,
				// otherwise, filter pods based on the match labels
				var labelSelector labels.Selector
				if len(to.PodSelector.MatchLabels) == 0 {
					labelSelector = labels.Everything()
				} else {
					podMatchLabel := labels.Set(to.PodSelector.MatchLabels)
					labelSelector = podMatchLabel.AsSelector()
				}

				pods, err := c.podLister.Pods(namespace).List(labelSelector)
				if err != nil {
					klog.Errorf("error listing pods in namespace %q, err: %v", namespace, err)
					continue
				}

				// take list of destination IPs and create flows
				// allow traffic from all these IPs, default drop with lowest priority
				destIPs := getPodIPs(pods)
				for _, srcIP := range localPodIPs {
					for _, destIP := range destIPs {
						// if no ports specified, allow traffic with src/dest match on all ports
						if len(eg.Ports) == 0 {
							flow := flows.NewFlow().WithTable(tableNetworkPolicies).
								WithPriority(1000).WithProtocol("ip").WithIPSrc(srcIP).WithIPDest(destIP).
								WithActionResubmit(tableClassification)
							c.flows.AddFlow(flow)

							continue
						}

						for _, port := range eg.Ports {
							portNum := port.Port.IntValue()
							protocol := strings.ToLower(string(*port.Protocol))

							if protocol == "tcp" {
								flow := flows.NewFlow().WithTable(tableNetworkPolicies).
									WithPriority(1000).WithProtocol("tcp").WithTCPDestPort(portNum).
									WithIPSrc(srcIP).WithIPDest(destIP).WithActionResubmit(tableClassification)
								c.flows.AddFlow(flow)
							}

							if protocol == "udp" {
								flow := flows.NewFlow().WithTable(tableNetworkPolicies).
									WithPriority(1000).WithProtocol("udp").WithUDPDestPort(portNum).
									WithIPSrc(srcIP).WithIPDest(destIP).WithActionResubmit(tableClassification)
								c.flows.AddFlow(flow)
							}

							// TODO: sctp
						}
					}
				}
			}
		}
	}

	return nil
}

func getPodIPs(pods []*corev1.Pod) []string {
	podIPs := []string{}
	for _, pod := range pods {
		podIP := pod.Status.PodIP
		if podIP == "" {
			continue
		}

		podIPs = append(podIPs, podIP)
	}

	return podIPs
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

func (c *controller) OnAddNetworkPolicy(obj interface{}) {
	err := c.syncFlows()
	if err != nil {
		klog.Errorf("error syncing flows: %v", err)
		return
	}
}

func (c *controller) OnUpdateNetworkPolicy(oldObj, newObj interface{}) {
	// TODO: check if needs update
	err := c.syncFlows()
	if err != nil {
		klog.Errorf("error syncing flows: %v", err)
		return
	}
}

func (c *controller) OnDeleteNetworkPolicy(obj interface{}) {
	err := c.syncFlows()
	if err != nil {
		klog.Errorf("error syncing flows: %v", err)
		return
	}

}
