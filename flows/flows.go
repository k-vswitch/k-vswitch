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

	protocol string
	ipDest   string
	arpDest  string
	tunID    int32

	tunDest    string
	tunIDField int32
	modDlDest  string
	output     int
	resubmit   int
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

	if f.arpDest != "" {
		flow = fmt.Sprintf("%s arp_tpa=%s", flow, f.arpDest)
	}

	if f.tunID != 0 {
		flow = fmt.Sprintf("%s tun_id=%d", flow, f.tunID)
	}

	var actionSet []string
	if f.modDlDest != "" {
		actionSet = append(actionSet, fmt.Sprintf("mod_dl_dst:%s", f.modDlDest))
	}

	if f.tunDest != "" {
		actionSet = append(actionSet, fmt.Sprintf("set_field:%s->tun_dst", f.tunDest))
	}

	if f.tunIDField != 0 {
		actionSet = append(actionSet, fmt.Sprintf("set_field:%d->tun_id", f.tunIDField))
	}

	if f.output != 0 {
		actionSet = append(actionSet, fmt.Sprintf("output:%d", f.output))
	}

	if f.resubmit != 0 {
		actionSet = append(actionSet, fmt.Sprintf("resubmit(,%d)", f.resubmit))
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

func (f *Flow) WithArpDest(arpDst string) *Flow {
	f.arpDest = arpDst
	return f
}

func (f *Flow) WithTunnelID(tunID int32) *Flow {
	f.tunID = tunID
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

func (f *Flow) WithTunnelIDField(tunID int32) *Flow {
	f.tunIDField = tunID
	return f
}

func (f *Flow) WithOutputPort(output int) *Flow {
	f.output = output
	return f
}

func (f *Flow) WithResubmit(table int) *Flow {
	f.resubmit = table
	return f
}
