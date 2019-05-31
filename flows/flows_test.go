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
	"testing"
)

func Test_Flow(t *testing.T) {
	tests := []struct {
		name       string
		flow       *Flow
		flowString string
	}{
		{
			name: "flow, no match with output port",
			flow: &Flow{
				table:    0,
				priority: 100,
				output:   1,
			},
			flowString: "table=0 priority=100 actions=output:1",
		},
		{
			name: "flow, with ipv4 match and output port",
			flow: &Flow{
				table:    0,
				priority: 100,
				protocol: "ip",
				ipDest:   "10.0.0.1",
				output:   1,
			},
			flowString: "table=0 priority=100 ip nw_dst=10.0.0.1 actions=output:1",
		},
		{
			name: "flow, with arp match and output port",
			flow: &Flow{
				table:    0,
				priority: 100,
				protocol: "arp",
				arpDest:  "10.0.0.1",
				output:   1,
			},
			flowString: "table=0 priority=100 arp arp_tpa=10.0.0.1 actions=output:1",
		},
		{
			name: "flow, with ipv4 match, mod datalink destination and output port",
			flow: &Flow{
				table:     0,
				priority:  100,
				protocol:  "ip",
				ipDest:    "10.0.0.1",
				modDlDest: "aa:bb:cc:dd:ee:ff",
				output:    1,
			},
			flowString: "table=0 priority=100 ip nw_dst=10.0.0.1 actions=mod_dl_dst:aa:bb:cc:dd:ee:ff,output:1",
		},
		{
			name: "flow, with ipv4 match, tunneling and output port",
			flow: &Flow{
				table:    0,
				priority: 100,
				protocol: "ip",
				ipDest:   "10.0.0.1",
				tunDest:  "172.20.0.1",
				output:   1,
			},
			flowString: "table=0 priority=100 ip nw_dst=10.0.0.1 actions=set_field:172.20.0.1->tun_dst,output:1",
		},
		{
			name: "flow, with ipv4 match and resubmit",
			flow: &Flow{
				table:    0,
				priority: 100,
				protocol: "ip",
				ipDest:   "10.0.0.1",
				resubmit: 20,
			},
			flowString: "table=0 priority=100 ip nw_dst=10.0.0.1 actions=resubmit(,20)",
		},
		{
			name: "arp responder flow",
			flow: &Flow{
				table:    20,
				priority: 500,
				protocol: "arp",
				ipDest:   "10.0.0.1",
				targetActions: []TargetAction{
					{
					ActionType: Action("move"),
					Source:NXM_OF_ETH_SRC,
					Target:NXM_OF_ETH_DST,
				},
					{
						ActionType: Action("load"),
						Source:ARP_RESPONSE,
						Target:NXM_OF_ARP_OP,
					},
			},},
			flowString: "table=20 priority=500 arp nw_dst=10.0.0.1 actions=move:NXM_OF_ETH_SRC[]->NXM_OF_ETH_DST[],load:0x2->NXM_OF_ARP_OP[]",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualFlow := fmt.Sprintf("%s", test.flow)
			if actualFlow != test.flowString {
				t.Logf("actual flow: %q", actualFlow)
				t.Logf("expected flow: %q", test.flowString)
				t.Errorf("flow string did not match")
			}
		})
	}
}
