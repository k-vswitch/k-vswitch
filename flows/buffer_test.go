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
	"testing"
)

func Test_AddFlow(t *testing.T) {
	flows := []*Flow{
		{
			table:    0,
			priority: 10,
			output:   1,
		},
		{
			table:    0,
			priority: 10,
			protocol: "ip",
			ipDest:   "10.0.0.1",
			resubmit: 10,
		},
		{
			table:    10,
			priority: 15,
			protocol: "ip",
			ipDest:   "10.0.0.1",
			output:   2,
		},
		{
			table:    20,
			priority: 5,
			protocol: "arp",
			arpDest:  "10.0.0.2",
			output:   5,
		},
		{
			table:     20,
			priority:  100,
			protocol:  "ip",
			ipDest:    "10.0.0.1",
			modDlDest: "aa:bb:cc:dd:ee:ff",
			output:    2,
		},
		{
			table:    30,
			priority: 100,
			protocol: "ip",
			ipDest:   "10.0.0.1",
			tunDest:  "172.20.0.1",
			output:   1,
		},
	}

	expectedBufferString := `table=0 priority=10 actions=output:1
table=0 priority=10 ip nw_dst=10.0.0.1 actions=resubmit(,10)
table=10 priority=15 ip nw_dst=10.0.0.1 actions=output:2
table=20 priority=5 arp arp_tpa=10.0.0.2 actions=output:5
table=20 priority=100 ip nw_dst=10.0.0.1 actions=mod_dl_dst:aa:bb:cc:dd:ee:ff,output:2
table=30 priority=100 ip nw_dst=10.0.0.1 actions=set_field:172.20.0.1->tun_dst,output:1
`

	flowsBuffer := NewFlowsBuffer()
	for _, flow := range flows {
		flowsBuffer.AddFlow(flow)
	}

	actualBufferString := flowsBuffer.String()
	if actualBufferString != expectedBufferString {
		t.Logf("actual buffer string: %q", actualBufferString)
		t.Logf("expected buffer string: %q", expectedBufferString)
		t.Errorf("unexpected buffer string")
	}
}
