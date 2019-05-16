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

package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	kvswitch "github.com/k-vswitch/k-vswitch/apis/generated/clientset/versioned"
	kvswitchinformer "github.com/k-vswitch/k-vswitch/apis/generated/informers/externalversions"
	"github.com/k-vswitch/k-vswitch/controllers/tunnel"
	"github.com/k-vswitch/k-vswitch/controllers/vswitchcfg"

	coreinformer "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
)

const (
	defaultClusterCIDR = "100.96.0.0/11"
	defaultServiceCIDR = "100.64.0.0/13"
	defaultOverlayType = "vxlan"
)

func main() {
	klog.InitFlags(flag.CommandLine)
	klog.Info("starting k-vswitch-controller")

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		klog.Errorf("error creating in-cluster config: %v", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("error getting kubernetes client: %v", err)
		os.Exit(1)
	}

	kvswitchClientset, err := kvswitch.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("error getting k-vswitch clientset: %v", err)
		os.Exit(1)
	}

	stopCh := make(chan struct{})

	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-term
		close(stopCh)
	}()

	kvswitchInformerFactory := kvswitchinformer.NewSharedInformerFactory(kvswitchClientset, 0)
	vswitchInformer := kvswitchInformerFactory.Kvswitch().V1alpha1().VSwitchConfigs()

	coreInformerFactory := coreinformer.NewSharedInformerFactory(clientset, 0)
	nodeInformer := coreInformerFactory.Core().V1().Nodes()

	kvswitchInformerFactory.WaitForCacheSync(stopCh)
	coreInformerFactory.WaitForCacheSync(stopCh)

	tunnelController := tunnel.NewTunnelIDAllocator(kvswitchClientset, vswitchInformer)
	vswitchInformer.Informer().AddEventHandler(tunnelController)

	vswitchController := vswitchcfg.NewVSwitchConfigController(vswitchInformer,
		nodeInformer, clientset, kvswitchClientset,
		defaultOverlayType, defaultClusterCIDR, defaultServiceCIDR)
	nodeInformer.Informer().AddEventHandler(vswitchController)

	kvswitchInformerFactory.Start(stopCh)
	coreInformerFactory.Start(stopCh)

	// TODO: use context to gracefully handle shutdown
	<-stopCh
}
