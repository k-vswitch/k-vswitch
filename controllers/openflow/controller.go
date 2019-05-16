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
	"os/exec"
	"strconv"
	"strings"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/containernetworking/plugins/pkg/ip"
	kvswitchinformers "github.com/k-vswitch/k-vswitch/apis/generated/informers/externalversions/kvswitch/v1alpha1"
	kvswitchlister "github.com/k-vswitch/k-vswitch/apis/generated/listers/kvswitch/v1alpha1"
	"github.com/k-vswitch/k-vswitch/flows"

	v1informer "k8s.io/client-go/informers/core/v1"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
)

const (
	hostLocalPort = "host-local"
	vxlanPort     = "vxlan0"
)

type connectionManager interface {
	Receive() ofp13.OFMessage
	Send(msg ofp13.OFMessage)
}

type controller struct {
	datapathID  uint64
	connManager connectionManager

	nodeName    string
	bridgeName  string
	gatewayIP   string
	gatewayMAC  string
	podCIDR     string
	clusterCIDR string

	hostLocalOFPort int
	vxlanOFPort     int

	flows *flows.FlowsBuffer

	nodeLister    v1lister.NodeLister
	podLister     v1lister.PodLister
	vswitchLister kvswitchlister.VSwitchConfigLister
}

func NewController(connManager connectionManager,
	nodeInformer v1informer.NodeInformer,
	podInformer v1informer.PodInformer,
	kvswitchInformer kvswitchinformers.VSwitchConfigInformer,
	bridgeName, gatewayMAC, nodeName,
	podCIDR, clusterCIDR string) (*controller, error) {

	_, podIPNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return nil, err
	}
	gatewayIP := ip.NextIP(podIPNet.IP.Mask(podIPNet.Mask)).String()

	hostLocalOFPort, err := ofPortFromName(hostLocalPort)
	if err != nil {
		return nil, err
	}

	vxlanOFPort, err := ofPortFromName(vxlanPort)
	if err != nil {
		return nil, err
	}

	return &controller{
		connManager:     connManager,
		nodeName:        nodeName,
		bridgeName:      bridgeName,
		gatewayIP:       gatewayIP,
		gatewayMAC:      gatewayMAC,
		podCIDR:         podCIDR,
		clusterCIDR:     clusterCIDR,
		hostLocalOFPort: hostLocalOFPort,
		vxlanOFPort:     vxlanOFPort,
		flows:           flows.NewFlowsBuffer(),
		nodeLister:      nodeInformer.Lister(),
		podLister:       podInformer.Lister(),
		vswitchLister:   kvswitchInformer.Lister(),
	}, nil
}

func (c *controller) Initialize() error {
	// send initial hello which is required to establish a proper connection
	// with an open flow switch.
	hello := ofp13.NewOfpHello()
	c.connManager.Send(hello)

	klog.Info("OF_HELLO message sent to switch")
	return nil
}

func (c *controller) Run() {
	for {
		msg := c.connManager.Receive()

		switch msgVal := msg.(type) {
		case *ofp13.OfpHeader:
			if msgVal.Type == ofp13.OFPT_ECHO_REQUEST {
				echoReply := ofp13.NewOfpEchoReply()
				klog.V(5).Info("echo reply sent to switch")
				c.connManager.Send(echoReply)
			}

			if msgVal.Type == ofp13.OFPT_ECHO_REPLY {
				klog.V(5).Info("received echo reply from switch")
			}

		case *ofp13.OfpHello:
			// hello received, next thing to do is send a feature request message
			// to receive the data path ID of the switch
			featureReq := ofp13.NewOfpFeaturesRequest()
			c.connManager.Send(featureReq)

		case *ofp13.OfpSwitchFeatures:
			c.datapathID = msgVal.DatapathId
			klog.Infof("set datapath ID to %d", c.datapathID)

		default:
		}
	}
}

func ofPortFromName(portName string) (int, error) {
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

	return ofport, nil
}
