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

type Flow struct {
	table    int
	priority int

	protocol    string
	ipDest      string
	ipSrc       string
	arpDest     string
	arpSrc      string
	tcpDestPort int
	tcpSrcPort  int
	udpDestPort int
	udpSrcPort  int

	tunDest   string
	modDlDest string
	output    int
	resubmit  int
	drop      bool

	raw string
}

func NewFlow() *Flow {
	return &Flow{}
}

func (f *Flow) String() string {
	// raw flow inputs take precedence, this is useful for flow entries that are
	// less common and don't warrant helper methods, arp responder is a good use-case
	if f.raw != "" {
		return f.raw
	}

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
	if f.modDlDest != "" {
		actionSet = append(actionSet, fmt.Sprintf("mod_dl_dst:%s", f.modDlDest))
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

	actions := fmt.Sprintf("actions=%s", strings.Join(actionSet, ","))

	flow = fmt.Sprintf("%s %s", flow, actions)
	return flow
}

func (f *Flow) Validate() error {
	return nil
}

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

func (f *Flow) WithModDlDest(dstMac string) *Flow {
	f.modDlDest = dstMac
	return f
}

func (f *Flow) WithTunnelDest(tunDst string) *Flow {
	f.tunDest = tunDst
	return f
}

func (f *Flow) WithOutputPort(output int) *Flow {
	f.output = output
	return f
}

func (f *Flow) WithDrop() *Flow {
	f.drop = true
	return f
}

func (f *Flow) WithResubmit(table int) *Flow {
	f.resubmit = table
	return f
}

func (f *Flow) WithRaw(raw string) *Flow {
	f.raw = raw
	return f
}
