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

package flows

import (
	"fmt"
	"strings"
)

const (
	NXM_OF_ARP_OP  = "NXM_OF_ARP_OP[]"
	NXM_OF_ETH_DST = "NXM_OF_ETH_DST[]"
	NXM_OF_ETH_SRC = "NXM_OF_ETH_SRC[]"
	NXM_NX_ARP_SHA = "NXM_NX_ARP_SHA[]"
	NXM_NX_ARP_THA = "NXM_NX_ARP_THA[]"
	NXM_OF_ARP_SPA = "NXM_OF_ARP_SPA[]"
	NXM_OF_ARP_TPA = "NXM_OF_ARP_TPA[]"

	ARP_RESPONSE  = "0x2"
    ETH_DST = "eth_dst"
	ETH_SRC = "eth_src"

	Move Action = "move"
	Load Action = "load"
	SetField Action = "set"
)

type Action string

type TargetAction struct {
	Source     string
	Target     string
	ActionType Action
}

type PriortyAction struct {
	action string
	priority int
}


type Flow struct {
	table       int
	priority    int
	tcpDestPort int
	tcpSrcPort  int
	udpDestPort int
	udpSrcPort  int
	output      int
	resubmit    int

	protocol  string
	ipDest    string
	ipSrc     string
	arpDest   string
	arpSrc    string
	tunDest   string
	modDlDest string
	modDlSrc string

	drop bool
	local     bool
	inPort bool

	targetActions []TargetAction
}

func NewFlow() *Flow {
	return &Flow{}
}

func (f *Flow) String() string {
	flow := fmt.Sprintf("table=%d priority=%d", f.table, f.priority)

	if f.protocol != "" {
		flow = fmt.Sprintf("%s %s", flow, f.protocol)
	}

	if f.ipDest != "" {
		flow = fmt.Sprintf("%s nw_dst=%s", flow, f.ipDest)
	}

	if f.ipSrc != "" {
		flow = fmt.Sprintf("%s nw_src=%s", flow, f.ipSrc)
	}

	if f.arpDest != "" {
		flow = fmt.Sprintf("%s arp_tpa=%s", flow, f.arpDest)
	}

	if f.arpSrc != "" {
		flow = fmt.Sprintf("%s arp_spa=%s", flow, f.arpSrc)
	}

	if f.tcpDestPort != 0 {
		flow = fmt.Sprintf("%s tcp_dst=%d", flow, f.tcpDestPort)
	}

	if f.tcpSrcPort != 0 {
		flow = fmt.Sprintf("%s tcp_src=%d", flow, f.tcpSrcPort)
	}

	if f.udpDestPort != 0 {
		flow = fmt.Sprintf("%s udp_dst=%d", flow, f.udpDestPort)
	}

	if f.udpSrcPort != 0 {
		flow = fmt.Sprintf("%s udp_src=%d", flow, f.udpSrcPort)
	}

	var actionSet []string
	if f.local {
		actionSet = append(actionSet, "local")
	}

	if f.modDlDest != "" {
		actionSet = append(actionSet, fmt.Sprintf("mod_dl_dst:%s", f.modDlDest))
	}

	if f.modDlSrc != "" {
		actionSet = append(actionSet, fmt.Sprintf("mod_dl_src:%s", f.modDlSrc))
	}

	if f.tunDest != "" {
		actionSet = append(actionSet, fmt.Sprintf("set_field:%s->tun_dst", f.tunDest))
	}

	if f.output != 0 {
		actionSet = append(actionSet, fmt.Sprintf("output:%d", f.output))
	}

	if f.resubmit != 0 {
		actionSet = append(actionSet, fmt.Sprintf("resubmit(,%d)", f.resubmit))
	}

	if f.drop {
		actionSet = append(actionSet, "drop")
	}

	if f.targetActions != nil {
		for _, action := range f.targetActions {
			switch action.ActionType {
			case Move:
				actionSet = append(actionSet, fmt.Sprintf("move:%s->%s", action.Source, action.Target))
			case Load:
				actionSet = append(actionSet, fmt.Sprintf("load:%s->%s", action.Source, action.Target))
			case SetField:
				actionSet = append(actionSet, fmt.Sprintf("set_field:%s->%s", action.Source, action.Target))
			}
		}
	}

	if f.inPort {
		actionSet = append(actionSet, "in_port")
	}

	actions := fmt.Sprintf("actions=%s", strings.Join(actionSet, ","))
	return  fmt.Sprintf("%s %s", flow, actions)
}

func (f *Flow) Validate() error {
	return nil
}

// Flow Matchers
func (f *Flow) WithTable(table int) *Flow {
	f.table = table
	return f
}

func (f *Flow) WithPriority(priority int) *Flow {
	f.priority = priority
	return f
}

func (f *Flow) WithProtocol(protocol string) *Flow {
	f.protocol = protocol
	return f
}

func (f *Flow) WithIPDest(ipDst string) *Flow {
	f.ipDest = ipDst
	return f
}

func (f *Flow) WithIPSrc(ipSrc string) *Flow {
	f.ipSrc = ipSrc
	return f
}

func (f *Flow) WithArpDest(arpDst string) *Flow {
	f.arpDest = arpDst
	return f
}

func (f *Flow) WithArpSrc(arpSrc string) *Flow {
	f.arpSrc = arpSrc
	return f
}

func (f *Flow) WithTCPSrcPort(srcPort int) *Flow {
	f.tcpSrcPort = srcPort
	return f
}

func (f *Flow) WithTCPDestPort(dstPort int) *Flow {
	f.tcpDestPort = dstPort
	return f
}

func (f *Flow) WithUDPSrcPort(srcPort int) *Flow {
	f.udpSrcPort = srcPort
	return f
}

func (f *Flow) WithUDPDestPort(dstPort int) *Flow {
	f.udpDestPort = dstPort
	return f
}

// Actions
func (f *Flow) WithActionModDlDest(dstMac string) *Flow {
	f.modDlDest = dstMac
	return f
}

func (f *Flow) WithActionModDlSrc(srcMac string) *Flow {
	f.modDlSrc = srcMac
	return f
}


func (f *Flow) WithActionTunnelDest(tunDst string) *Flow {
	f.tunDest = tunDst
	return f
}

func (f *Flow) WithActionOutputPort(output int) *Flow {
	f.output = output
	return f
}

func (f *Flow) WithActionDrop() *Flow {
	f.drop = true
	return f
}

func (f *Flow) WithActionResubmit(table int) *Flow {
	f.resubmit = table
	return f
}

func (f *Flow) WithInPort() *Flow {
	f.inPort = true
	return f
}

func (f *Flow) WithLocal() *Flow {
	f.local = true
	return f
}

func (f *Flow) WithTargetAction(targetAction TargetAction) *Flow {
	f.targetActions = append(f.targetActions, targetAction)
	return f
}



