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
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
)

func (c *controller) OnAddPod(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	if err := c.syncPod(pod); err != nil {
		klog.Errorf("error syncing pod: %v", err)
		return
	}
}

func (c *controller) OnUpdatePod(oldObj, newObj interface{}) {
	pod, ok := newObj.(*corev1.Pod)
	if !ok {
		return
	}

	if err := c.syncPod(pod); err != nil {
		klog.Errorf("error syncing pod : %v", err)
		return
	}
}

func (c *controller) OnDeletePod(obj interface{}) {
	_, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}

	// TODO: handle delete case, right now this is not trivial
	// because we need to know the pod IP to find the correct match set
}

func (c *controller) isLocalIP(ip string) (bool, error) {
	_, ipnet, err := net.ParseCIDR(c.podCIDR)
	if err != nil {
		return false, err
	}

	return ipnet.Contains(net.ParseIP(ip)), nil
}

func (c *controller) syncPod(pod *corev1.Pod) error {
	podIP := pod.Status.PodIP
	if podIP == "" {
		return nil
	}

	local, err := c.isLocalIP(podIP)
	if err != nil {
		return fmt.Errorf("error checking if IP %q is local: %v", err)
	}

	if !local {
		return nil
	}

	flows, err := c.addDataLinkFlowsForLocalIP(pod)
	if err != nil {
		return fmt.Errorf("error getting data link flows for IP %q, err: %v", podIP, err)
	}

	for _, flow := range flows {
		c.connManager.Send(flow)
	}

	return nil
}

func (c *controller) addDataLinkFlowsForLocalIP(pod *corev1.Pod) ([]*ofp13.OfpFlowMod, error) {
	flows := []*ofp13.OfpFlowMod{}

	podIP := pod.Status.PodIP

	portName, err := findPort(pod.Namespace, pod.Name)
	if err != nil {
		return nil, fmt.Errorf("error finding port for pod %q, err: %v", pod.Name, err)
	}

	podMacAddr, err := macAddrFromPort(portName)
	if err != nil {
		return nil, fmt.Errorf("error getting mac address for pod %q, err: %v", pod.Name, err)
	}

	ofport, err := ofPortFromName(portName)
	if err != nil {
		return nil, fmt.Errorf("error getting ofport for port %q, err: %v", portName, err)
	}

	ipv4Match, err := ofp13.NewOxmIpv4Dst(podIP)
	if err != nil {
		return nil, fmt.Errorf("error getting IPv4Dst match: %v", err)
	}

	// add flow for this endpoint
	match := ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0800))
	match.Append(ipv4Match)

	instruction := ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
	ethDst, err := ofp13.NewOxmEthDst(podMacAddr)
	if err != nil {
		return nil, err
	}

	instruction.Append(ofp13.NewOfpActionSetField(ethDst))
	instruction.Append(ofp13.NewOfpActionOutput(ofport, 0))

	flow := ofp13.NewOfpFlowModAdd(0, 0, tableL2Rewrites, 100, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	// ARP flow for pod
	arpMatch, err := ofp13.NewOxmArpTpa(podIP)
	if err != nil {
		return nil, fmt.Errorf("error getting IPv4Dst match: %v", err)
	}
	match = ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0806))
	match.Append(arpMatch)

	instruction = ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
	instruction.Append(ofp13.NewOfpActionOutput(ofport, 0))

	flow = ofp13.NewOfpFlowModAdd(0, 0, tableL2Rewrites, 100, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	return flows, nil
}

func (c *controller) addDataLinkFlowForGateway() ([]*ofp13.OfpFlowMod, error) {
	flows := []*ofp13.OfpFlowMod{}

	ofport, err := ofPortFromName(hostLocalPort)
	if err != nil {
		return nil, fmt.Errorf("error getting ofport for port %q, err: %v", hostLocalPort, err)
	}

	ipv4Match, err := ofp13.NewOxmIpv4Dst(c.gatewayIP)

	if err != nil {
		return nil, fmt.Errorf("error getting IPv4Dst match: %v", err)
	}

	match := ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0800))
	match.Append(ipv4Match)

	instruction := ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)

	ethDst, err := ofp13.NewOxmEthDst(c.gatewayMAC)
	if err != nil {
		return nil, err
	}

	instruction.Append(ofp13.NewOfpActionSetField(ethDst))
	instruction.Append(ofp13.NewOfpActionOutput(ofport, 0))

	flow := ofp13.NewOfpFlowModAdd(0, 0, tableClassification, 500, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	arpMatch, err := ofp13.NewOxmArpTpa(c.gatewayIP)
	if err != nil {
		return nil, fmt.Errorf("error getting IPv4Dst match: %v", err)
	}

	match = ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0806))
	match.Append(arpMatch)

	instruction = ofp13.NewOfpInstructionActions(ofp13.OFPIT_APPLY_ACTIONS)
	instruction.Append(ofp13.NewOfpActionOutput(ofport, 0))

	flow = ofp13.NewOfpFlowModAdd(0, 0, tableLocalARP, 500, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	return flows, nil
}

func (c *controller) defaultARPFlows() ([]*ofp13.OfpFlowMod, error) {
	flows := []*ofp13.OfpFlowMod{}

	match := ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0806))

	instruction := ofp13.NewOfpInstructionGotoTable(tableLocalARP)
	flow := ofp13.NewOfpFlowModAdd(0, 0, tableClassification, 200, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	arpMatch, err := newOxmArpDst(c.podCIDR)
	if err != nil {
		return nil, err
	}

	match = ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0806))
	match.Append(arpMatch)

	instruction = ofp13.NewOfpInstructionGotoTable(tableL2Rewrites)
	flow = ofp13.NewOfpFlowModAdd(0, 0, tableLocalARP, 100, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	arpMatch, err = newOxmArpDst(c.clusterCIDR)
	if err != nil {
		return nil, err
	}

	match = ofp13.NewOfpMatch()
	match.Append(ofp13.NewOxmEthType(0x0806))
	match.Append(arpMatch)

	instruction = ofp13.NewOfpInstructionGotoTable(tableOverlay)
	flow = ofp13.NewOfpFlowModAdd(0, 0, tableLocalARP, 50, 0, match,
		[]ofp13.OfpInstruction{instruction})
	flows = append(flows, flow)

	return flows, nil
}

func findPort(podNamespace, podName string) (string, error) {
	commands := []string{
		"--format=json", "--column=name", "find", "port",
		fmt.Sprintf("external-ids:k8s_pod_namespace=%s", podNamespace),
		fmt.Sprintf("external-ids:k8s_pod_name=%s", podName),
	}

	out, err := exec.Command("ovs-vsctl", commands...).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get OVS port for %s/%s, err: %v",
			podNamespace, podName, err)
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

func macAddrFromPort(portName string) (string, error) {
	commands := []string{
		"get", "port", portName, "mac",
	}

	out, err := exec.Command("ovs-vsctl", commands...).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get MAC address from OVS port for %q, err: %v, out: %q",
			portName, err, string(out))
	}

	// TODO: validate mac address
	macAddr := strings.TrimSpace(string(out))
	if len(macAddr) > 0 && macAddr[0] == '"' {
		macAddr = macAddr[1:]
	}
	if len(macAddr) > 0 && macAddr[len(macAddr)-1] == '"' {
		macAddr = macAddr[:len(macAddr)-1]
	}

	return macAddr, nil
}
