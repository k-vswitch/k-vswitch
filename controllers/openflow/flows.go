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
	"os/exec"
	"strconv"
	"strings"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/kube-ovs/kube-ovs/apis/kubeovs/v1alpha1"

	"k8s.io/klog"
)

const (
	tableClassification         = 0
	tableLocalARP               = 10
	tableOverlay                = 20
	tableL3Rewrites             = 30
	tableL3Forwarding           = 40
	tableL2Rewrites             = 50
	tableL2Forwarding           = 60
	tableNetworkPoliciesIngress = 70
	tableNetworkPoliciesEgress  = 80
	tableProxy                  = 90
	tableNAT                    = 100
	tableAudit                  = 110

	vxlanPortName = "vxlan0"
)

func (c *controller) AddDefaultFlows(bridgeName string) error {
	ofport, err := ofPortFromName(hostLocalPort)
	if err != nil {
		return fmt.Errorf("getting host local port number: %v", err)
	}

	instruction := ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
	instruction.Actions = append(instruction.Actions, ofp13.NewOfpActionOutput(ofport, 0))

	defaultLocalFlow := ofp13.NewOfpFlowModAdd(0, 0, tableClassification, 0, 0, ofp13.NewOfpMatch(),
		[]ofp13.OfpInstruction{instruction})
	c.connManager.Send(defaultLocalFlow)

	gatewayFlows, err := c.addDataLinkFlowForGateway()
	if err != nil {
		return fmt.Errorf("error getting datalink flow for gateway IP %q, err: %v", c.gatewayIP)
	}

	for _, flow := range gatewayFlows {
		c.connManager.Send(flow)
	}

	arpFlows, err := c.defaultARPFlows()
	if err != nil {
		return fmt.Errorf("error adding arp responder flows: %v", err)
	}

	for _, flow := range arpFlows {
		c.connManager.Send(flow)
	}

	return nil
}

func (c *controller) flowsForVSwitch(vswitch *v1alpha1.VSwitchConfig) ([]*ofp13.OfpFlowMod, error) {
	flows := []*ofp13.OfpFlowMod{}

	isCurrentNode := false
	if vswitch.Name == c.nodeName {
		isCurrentNode = true
	}

	podCIDR := vswitch.Spec.PodCIDR

	vxlanOFPort, err := ofPortFromName(vxlanPortName)
	if err != nil {
		return nil, fmt.Errorf("error getting ofport for %q: %v", vxlanPortName, err)
	}

	var instruction ofp13.OfpInstruction

	//
	// flows for table 0 - classification
	//

	// traffic in the local pod CIDR should go to tableL2Rewrites
	// TODO: put this in a separate function for "local" flows

	match := ofp13.NewOfpMatch()
	ipv4Match, err := newOxmIpv4SubnetDst(c.podCIDR)
	if err != nil {
		return nil, err
	}
	match.Append(ofp13.NewOxmEthType(0x0800))
	match.Append(ipv4Match)
	instruction = ofp13.NewOfpInstructionGotoTable(tableL2Rewrites)
	flow := ofp13.NewOfpFlowModAdd(0, 0, tableClassification, 300, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	// traffic in the cluster CIDR without tunnel ID should go to tableOverlay
	match = ofp13.NewOfpMatch()
	ipv4Match, err = newOxmIpv4SubnetDst(c.clusterCIDR)
	if err != nil {
		return nil, err
	}
	match.Append(ofp13.NewOxmEthType(0x0800))
	match.Append(ipv4Match)
	instruction = ofp13.NewOfpInstructionGotoTable(tableOverlay)
	flow = ofp13.NewOfpFlowModAdd(0, 0, tableClassification, 100, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	//
	// flow for table 10 - overlay
	//

	if !isCurrentNode {
		ipv4Match, err = newOxmIpv4SubnetDst(podCIDR)
		if err != nil {
			return nil, err
		}

		// IPv4
		match = ofp13.NewOfpMatch()
		match.Append(ofp13.NewOxmEthType(0x0800))
		match.Append(ipv4Match)

		applyInstruction := ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
		tunnelField := ofp13.NewOxmTunnelId(uint64(vswitch.Spec.OverlayTunnelID))
		applyInstruction.Append(ofp13.NewOfpActionSetField(tunnelField))
		gotoInstruction := ofp13.NewOfpInstructionGotoTable(tableL3Forwarding)

		flow = ofp13.NewOfpFlowModAdd(0, 0, tableOverlay, 100, 0, match,
			[]ofp13.OfpInstruction{applyInstruction, gotoInstruction})
		flows = append(flows, flow)

		arpMatch, err := newOxmArpDst(podCIDR)
		if err != nil {
			return nil, err
		}

		// ARP
		match = ofp13.NewOfpMatch()
		match.Append(ofp13.NewOxmEthType(0x0806))
		match.Append(arpMatch)

		applyInstruction = ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
		tunnelField = ofp13.NewOxmTunnelId(uint64(vswitch.Spec.OverlayTunnelID))
		applyInstruction.Append(ofp13.NewOfpActionSetField(tunnelField))
		gotoInstruction = ofp13.NewOfpInstructionGotoTable(tableL3Forwarding)

		flow = ofp13.NewOfpFlowModAdd(0, 0, tableOverlay, 100, 0, match,
			[]ofp13.OfpInstruction{applyInstruction, gotoInstruction})
		flows = append(flows, flow)
	}

	//
	// flow for table 30 - L3 Forwarding
	//

	// If pod cidr is not for current node, output to vxlan overlay port
	// If pod cidr is for current node, go straight to L2 rewrites
	if !isCurrentNode {
		err = addTunnelDstFlows(vswitch, podCIDR, int(vxlanOFPort))
		if err != nil {
			return nil, fmt.Errorf("error adding tunnel forwarding flows: %v", err)
		}
	} else {
		ipv4Match, err = newOxmIpv4SubnetDst(podCIDR)
		if err != nil {
			return nil, err
		}

		match = ofp13.NewOfpMatch()
		match.Append(ofp13.NewOxmEthType(0x0800))
		match.Append(ipv4Match)
		match.Append(ofp13.NewOxmTunnelId(uint64(vswitch.Spec.OverlayTunnelID)))
		gotoInstruction := ofp13.NewOfpInstructionGotoTable(tableL2Rewrites)

		flow = ofp13.NewOfpFlowModAdd(0, 0, tableL3Forwarding, 100, 0, match,
			[]ofp13.OfpInstruction{gotoInstruction})
		flows = append(flows, flow)

		arpMatch, err := newOxmArpDst(podCIDR)
		if err != nil {
			return nil, err
		}

		match = ofp13.NewOfpMatch()
		match.Append(ofp13.NewOxmEthType(0x0806))
		match.Append(arpMatch)
		match.Append(ofp13.NewOxmTunnelId(uint64(vswitch.Spec.OverlayTunnelID)))
		gotoInstruction = ofp13.NewOfpInstructionGotoTable(tableL2Rewrites)

		flow = ofp13.NewOfpFlowModAdd(0, 0, tableL3Forwarding, 100, 0, match,
			[]ofp13.OfpInstruction{gotoInstruction})
		flows = append(flows, flow)
	}

	return flows, nil
}

func ofPortFromName(portName string) (uint32, error) {
	command := []string{
		"get", "Interface", portName, "ofport",
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to get ofport for port %q, err: %v, out: %q", portName, err, out)
	}

	ofport, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("error converting ofport output %q to int: %v", string(out), err)
	}

	return uint32(ofport), nil
}

func newOxmIpv4SubnetDst(dst string) (*ofp13.OxmIpv4, error) {
	dstSplit := strings.Split(dst, "/")
	if len(dstSplit) != 2 {
		return nil, fmt.Errorf("invalid destination: %q", dst)
	}

	addr := dstSplit[0]
	mask, err := strconv.Atoi(dstSplit[1])
	if err != nil {
		return nil, fmt.Errorf("invalid mask from cidr: %v", err)
	}

	ipv4Match, err := ofp13.NewOxmIpv4DstW(addr, mask)
	if err != nil {
		return nil, fmt.Errorf("error getting IPv4DstW match: %v", err)
	}

	return ipv4Match, nil
}

func newOxmArpDst(dst string) (*ofp13.OxmArpPa, error) {
	dstSplit := strings.Split(dst, "/")
	if len(dstSplit) != 2 {
		return nil, fmt.Errorf("invalid destination: %q", dst)
	}

	addr := dstSplit[0]
	mask, err := strconv.Atoi(dstSplit[1])
	if err != nil {
		return nil, fmt.Errorf("invalid mask from cidr: %v", err)
	}

	arpMatch, err := ofp13.NewOxmArpTpaW(addr, mask)
	if err != nil {
		return nil, fmt.Errorf("error getting IPv4DstW match: %v", err)
	}

	return arpMatch, nil
}

func addTunnelDstFlows(vswitch *v1alpha1.VSwitchConfig, podCIDR string, ofport int) error {
	command := []string{
		"add-flow", "kube-ovs0",
		fmt.Sprintf("table=%d, priority=150,ip,tun_id=%d,nw_dst=%s,actions=set_field:%s->tun_dst,%d",
			tableL3Forwarding,
			vswitch.Spec.OverlayTunnelID,
			podCIDR,
			vswitch.Spec.OverlayIP,
			ofport,
		),
	}

	out, err := exec.Command("ovs-ofctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add flows, err: %v, out: %q", err, out)
	}

	command = []string{
		"add-flow", "kube-ovs0",
		fmt.Sprintf("table=%d, priority=150,arp,tun_id=%d,nw_dst=%s,actions=set_field:%s->tun_dst,%d",
			tableL3Forwarding,
			vswitch.Spec.OverlayTunnelID,
			podCIDR,
			vswitch.Spec.OverlayIP,
			ofport,
		),
	}

	out, err = exec.Command("ovs-ofctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add flows, err: %v, out: %q", err, out)
	}

	return nil
}

func (c *controller) OnAddVSwitch(obj interface{}) {
	vswitch, ok := obj.(*v1alpha1.VSwitchConfig)
	if !ok {
		return
	}

	flows, err := c.flowsForVSwitch(vswitch)
	if err != nil {
		klog.Errorf("error getting flows for vswitch %q, err: %v", vswitch.Name, err)
		return
	}

	for _, flow := range flows {
		c.connManager.Send(flow)
	}
}

func (c *controller) OnUpdateVSwitch(oldObj, newObj interface{}) {
	vswitch, ok := newObj.(*v1alpha1.VSwitchConfig)
	if !ok {
		return
	}

	flows, err := c.flowsForVSwitch(vswitch)
	if err != nil {
		klog.Errorf("error getting flows for vswitch %q, err: %v", vswitch.Name, err)
		return
	}

	for _, flow := range flows {
		c.connManager.Send(flow)
	}
}

func (c *controller) OnDeleteVSwitch(obj interface{}) {
	_, ok := obj.(*v1alpha1.VSwitchConfig)
	if !ok {
		return
	}
}
