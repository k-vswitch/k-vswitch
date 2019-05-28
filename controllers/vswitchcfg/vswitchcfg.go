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

package vswitchcfg

import (
	"errors"
	"fmt"
	kvswitch "github.com/k-vswitch/k-vswitch/apis/generated/clientset/versioned"
	kvswitchinformers "github.com/k-vswitch/k-vswitch/apis/generated/informers/externalversions/kvswitch/v1alpha1"
	kvswitchlister "github.com/k-vswitch/k-vswitch/apis/generated/listers/kvswitch/v1alpha1"
	kvswitchv1alpha1 "github.com/k-vswitch/k-vswitch/apis/kvswitch/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
)

// vswitchConfig is a controller that creates VSwitchConfig resources
// based on node events
// TODO: implement controller using queues
type vswitchConfig struct {
	overlayType string
	clusterCIDR string
	serviceCIDR string

	kvswitchClient kvswitch.Interface
	kubeClient     kubernetes.Interface

	vswitchLister kvswitchlister.VSwitchConfigLister
	nodeLister    v1lister.NodeLister
}

func NewVSwitchConfigController(
	vswitchInformer kvswitchinformers.VSwitchConfigInformer,
	nodeInformer v1informer.NodeInformer,
	kubeClient kubernetes.Interface,
	kvswitchClient kvswitch.Interface,
	overlayType, clusterCIDR, serviceCIDR string) *vswitchConfig {

	v := &vswitchConfig{
		overlayType:    overlayType,
		clusterCIDR:    clusterCIDR,
		serviceCIDR:    serviceCIDR,
		kvswitchClient: kvswitchClient,
		kubeClient:     kubeClient,
		vswitchLister:  vswitchInformer.Lister(),
		nodeLister:     nodeInformer.Lister(),
	}

	return v
}

func (v *vswitchConfig) OnAdd(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("obj %v was not core/v1 node", obj)
		return
	}

	shouldUpdate, err := v.needsUpdate(node)
	if err != nil {
		klog.Errorf("error checking if vswitch config for node %q needs update: %v", node.Name, err)
		return
	}

	if !shouldUpdate {
		return
	}

	err = v.syncVSwitchConfig(node)
	if err != nil {
		klog.Errorf("error syncing VSwitchConfig: %v", err)
	}
}

func (v *vswitchConfig) OnUpdate(oldObj, newObj interface{}) {
	node, ok := newObj.(*corev1.Node)
	if !ok {
		klog.Errorf("obj %v was not core/v1 node", newObj)
	}

	shouldUpdate, err := v.needsUpdate(node)
	if err != nil {
		klog.Errorf("error checking if vswitch config for node %q needs update: %v", node.Name, err)
		return
	}

	if !shouldUpdate {
		return
	}

	err = v.syncVSwitchConfig(node)
	if err != nil {
		klog.Errorf("error syncing VSwitchConfig: %v", err)
	}
}

func (v *vswitchConfig) OnDelete(obj interface{}) {
	node, ok := obj.(*corev1.Node)
	if !ok {
		klog.Errorf("obj %v was not core/v1 node", obj)
	}

	err := v.kvswitchClient.KvswitchV1alpha1().VSwitchConfigs().Delete(node.Name, &metav1.DeleteOptions{})
	if err != nil {
		klog.Errorf("error syncing VSwitchConfig: %v", err)
	}
}

func (v *vswitchConfig) needsUpdate(node *corev1.Node) (bool, error) {
	vswitchCfg, err := v.vswitchLister.Get(node.Name)
	if apierr.IsNotFound(err) {
		return true, nil
	}

	if err != nil {
		return false, fmt.Errorf("error getting vswitch config from lister: %v", err)
	}

	overlayIP, err := nodeOverlayIP(node)
	if err != nil {
		return false, fmt.Errorf("failed to get overlay IP for node: %v", err)
	}

	nodePodCIDR, err := nodePodCIDR(node)
	if err != nil {
		return false, fmt.Errorf("failed to get pod CIDR for node: %v", err)
	}

	var (
		overlayIPChanged   = vswitchCfg.Spec.OverlayIP != overlayIP
		overlayTypeChanged = vswitchCfg.Spec.OverlayType != v.overlayType
		podCIDRChanged     = vswitchCfg.Spec.PodCIDR != nodePodCIDR
		clusterCIDRChanged = vswitchCfg.Spec.ClusterCIDR != v.clusterCIDR
	)

	return overlayIPChanged || overlayTypeChanged || podCIDRChanged || clusterCIDRChanged, nil
}

func (v *vswitchConfig) syncVSwitchConfig(node *corev1.Node) error {
	overlayIP, err := nodeOverlayIP(node)
	if err != nil {
		return fmt.Errorf("error getting overlay IP for node %q, err: %v", node.Name, err)
	}

	nodePodCIDR, err := nodePodCIDR(node)
	if err != nil {
		return fmt.Errorf("failed to get pod CIDR for node: %v", err)
	}

	vswitchCfg, err := v.vswitchLister.Get(node.Name)
	if apierr.IsNotFound(err) {
		vswitchCfg := nodeToVSwitchConfig(node, v.overlayType, overlayIP,
			nodePodCIDR, v.clusterCIDR, v.serviceCIDR)
		_, err = v.kvswitchClient.KvswitchV1alpha1().VSwitchConfigs().Create(vswitchCfg)
		return err
	}

	if err != nil {
		return fmt.Errorf("error getting vswitch config from cache: %v", err)
	}

	newVSwitchCfg := vswitchCfg.DeepCopy()
	newVSwitchCfg.Spec.OverlayIP = overlayIP
	newVSwitchCfg.Spec.OverlayType = v.overlayType
	newVSwitchCfg.Spec.PodCIDR = nodePodCIDR
	newVSwitchCfg.Spec.ClusterCIDR = v.clusterCIDR
	newVSwitchCfg.Spec.ServiceCIDR = v.serviceCIDR

	_, err = v.kvswitchClient.KvswitchV1alpha1().VSwitchConfigs().Update(newVSwitchCfg)
	return err
}

func nodeToVSwitchConfig(node *corev1.Node, overlayType, overlayIP,
	podCIDR, clusterCIDR, serviceCIDR string) *kvswitchv1alpha1.VSwitchConfig {
	return &kvswitchv1alpha1.VSwitchConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: node.Name,
		},
		Spec: kvswitchv1alpha1.VSwitchConfigSpec{
			OverlayIP:   overlayIP,
			OverlayType: overlayType,
			PodCIDR:     podCIDR,
			ClusterCIDR: clusterCIDR,
			ServiceCIDR: serviceCIDR,
		},
	}
}

func nodeOverlayIP(node *corev1.Node) (string, error) {
	var internalIP, externalIP string

	nodeAddresses := node.Status.Addresses
	for _, nodeAddress := range nodeAddresses {
		if nodeAddress.Type == corev1.NodeInternalIP {
			internalIP = nodeAddress.Address
		}

		if nodeAddress.Type == corev1.NodeExternalIP {
			externalIP = nodeAddress.Address
		}
	}

	if internalIP != "" {
		return internalIP, nil
	}

	if externalIP != "" {
		return externalIP, nil
	}

	return "", errors.New("no valid node IP found for tunnel overlay")
}

func nodePodCIDR(node *corev1.Node) (string, error) {
	if podCIDR := node.Spec.PodCIDR; podCIDR != "" {
		return podCIDR, nil
	}
	return "", fmt.Errorf("node %q did not have pod CIDR set", node.Name)
}
