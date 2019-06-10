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
	"net"
	"sync"
	"time"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
	"github.com/containernetworking/plugins/pkg/ip"
	kvswitchinformers "github.com/k-vswitch/k-vswitch/apis/generated/informers/externalversions/kvswitch/v1alpha1"
	kvswitchlister "github.com/k-vswitch/k-vswitch/apis/generated/listers/kvswitch/v1alpha1"
	"github.com/k-vswitch/k-vswitch/flows"

	"k8s.io/apimachinery/pkg/util/wait"
	v1informer "k8s.io/client-go/informers/core/v1"
	netinformer "k8s.io/client-go/informers/networking/v1"
	v1lister "k8s.io/client-go/listers/core/v1"
	netlister "k8s.io/client-go/listers/networking/v1"
	"k8s.io/klog"
)

const (
	clusterWidePort = "cluster-wide"
	overlayPort     = "overlay0"
)

type connectionManager interface {
	Receive() ofp13.OFMessage
	Send(msg ofp13.OFMessage)
}

type controller struct {
	datapathID  uint64
	connManager connectionManager

	nodeName       string
	bridgeName     string
	gatewayIP      string
	bridgeMAC      string
	clusterWideMAC string
	podCIDR        string
	clusterCIDR    string

	clusterWideOFPort int
	overlayOFPort     int

	flows     *flows.FlowsBuffer
	flowsLock sync.Mutex

	portCache *portCache

	podLister     v1lister.PodLister
	nsLister      v1lister.NamespaceLister
	netPolLister  netlister.NetworkPolicyLister
	vswitchLister kvswitchlister.VSwitchConfigLister
}

func NewController(connManager connectionManager,
	podInformer v1informer.PodInformer,
	namespaceInformer v1informer.NamespaceInformer,
	netPolInformer netinformer.NetworkPolicyInformer,
	kvswitchInformer kvswitchinformers.VSwitchConfigInformer,
	bridgeName, bridgeMAC, clusterWideMAC, nodeName,
	podCIDR, clusterCIDR string) (*controller, error) {

	_, podIPNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		return nil, err
	}
	gatewayIP := ip.NextIP(podIPNet.IP.Mask(podIPNet.Mask)).String()

	clusterWideOFPort, err := ofPortFromName(clusterWidePort)
	if err != nil {
		return nil, err
	}

	overlayOFPort, err := ofPortFromName(overlayPort)
	if err != nil {
		return nil, err
	}

	return &controller{
		connManager:       connManager,
		nodeName:          nodeName,
		bridgeName:        bridgeName,
		gatewayIP:         gatewayIP,
		bridgeMAC:         bridgeMAC,
		clusterWideMAC:    clusterWideMAC,
		podCIDR:           podCIDR,
		clusterCIDR:       clusterCIDR,
		clusterWideOFPort: clusterWideOFPort,
		overlayOFPort:     overlayOFPort,
		flows:             flows.NewFlowsBuffer(),
		portCache:         NewPortCache(),
		podLister:         podInformer.Lister(),
		nsLister:          namespaceInformer.Lister(),
		netPolLister:      netPolInformer.Lister(),
		vswitchLister:     kvswitchInformer.Lister(),
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

func (c *controller) Run(stopCh <-chan struct{}) {
	fullResync := func() {
		klog.V(5).Info("running periodic full resync for flows")

		err := c.syncFlows()
		if err != nil {
			klog.Errorf("error syncing flows: %v", err)
		}
	}
	go wait.Until(fullResync, time.Minute, stopCh)

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
