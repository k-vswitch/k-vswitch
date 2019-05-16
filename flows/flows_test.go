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
			name: "flow, with ipv4 match, tunnelling and output port",
			flow: &Flow{
				table:    0,
				priority: 100,
				protocol: "ip",
				ipDest:   "10.0.0.1",
				tunID:    70,
				tunDest:  "172.20.0.1",
				output:   1,
			},
			flowString: "table=0 priority=100 ip nw_dst=10.0.0.1 tun_id=70 actions=set_field:172.20.0.1->tun_dst,output:1",
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
