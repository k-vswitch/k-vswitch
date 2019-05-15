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

package tunnel

import (
	"errors"
	"fmt"
	"sync"

	kovs "github.com/kube-ovs/kube-ovs/apis/generated/clientset/versioned"
	kovsinformers "github.com/kube-ovs/kube-ovs/apis/generated/informers/externalversions/kubeovs/v1alpha1"
	kovslisters "github.com/kube-ovs/kube-ovs/apis/generated/listers/kubeovs/v1alpha1"
	kovsv1alpha1 "github.com/kube-ovs/kube-ovs/apis/kubeovs/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	defaultMinTunnelID = 100
	defaultMaxTunnelID = 9999
)

// TODO: implement controller using queue
type tunnelIDAllocactor struct {
	// mutex ensures only 1 tunnel ID is being allocated at a time
	allocatorLock sync.Mutex

	kovsClientset kovs.Interface
	vswitchLister kovslisters.VSwitchConfigLister
}

func NewTunnelIDAllocator(kovsClient kovs.Interface,
	vswitchInformer kovsinformers.VSwitchConfigInformer) *tunnelIDAllocactor {
	t := &tunnelIDAllocactor{
		kovsClientset: kovsClient,
		vswitchLister: vswitchInformer.Lister(),
	}

	return t
}

func (t *tunnelIDAllocactor) OnAdd(obj interface{}) {
	vswitchcfg, ok := obj.(*kovsv1alpha1.VSwitchConfig)
	if !ok {
		klog.Errorf("obj %v was not kubeovs/v1alpha1 VSwitchConfig", obj)
		return
	}

	if vswitchcfg.Spec.OverlayTunnelID != 0 {
		return
	}

	err := t.allocateTunnelID(vswitchcfg)
	if err != nil {
		klog.Errorf("error allocating tunnel ID for vswitch %q, err: %v", vswitchcfg.Name, err)
		return
	}
}

func (t *tunnelIDAllocactor) OnUpdate(oldObj, newObj interface{}) {
	vswitchcfg, ok := newObj.(*kovsv1alpha1.VSwitchConfig)
	if !ok {
		klog.Errorf("obj %v was not kubeovs/v1alpha1 VSwitchConfig", newObj)
		return
	}

	if vswitchcfg.Spec.OverlayTunnelID != 0 {
		return
	}

	err := t.allocateTunnelID(vswitchcfg)
	if err != nil {
		klog.Errorf("error allocating tunnel ID for vswitch %q, err: %v", vswitchcfg.Name, err)
		return
	}
}

func (t *tunnelIDAllocactor) OnDelete(obj interface{}) {
	_, ok := obj.(*kovsv1alpha1.VSwitchConfig)
	if !ok {
		klog.Errorf("obj %v was not kubeovs/v1alpha1 VSwitchConfig", obj)
		return
	}
}

func (t *tunnelIDAllocactor) allocateTunnelID(vswitchcfg *kovsv1alpha1.VSwitchConfig) error {
	t.allocatorLock.Lock()
	defer t.allocatorLock.Unlock()

	// fetch the VSwitchConfig resource one more time to ensure it doesn't exist before we try to allocate one.
	vswitchcfg, err := t.kovsClientset.KubeovsV1alpha1().VSwitchConfigs().Get(vswitchcfg.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error refetching vswitch config: %v", err)
	}

	if vswitchcfg.Spec.OverlayTunnelID != 0 {
		return nil
	}

	tunIDs, err := t.getCurrentTunnelIDs()
	if err != nil {
		return fmt.Errorf("error getting the current set of tunnel IDs: %v", err)
	}

	var candTunID int32
	for candTunID = defaultMinTunnelID; candTunID <= defaultMaxTunnelID; candTunID++ {
		_, found := tunIDs[candTunID]
		if !found {
			break
		}
	}

	newVSwitchCfg := vswitchcfg.DeepCopy()
	newVSwitchCfg.Spec.OverlayTunnelID = candTunID
	_, err = t.kovsClientset.KubeovsV1alpha1().VSwitchConfigs().Update(newVSwitchCfg)
	if err != nil {
		return fmt.Errorf("error updating vswitch config with tunnel ID: %v", err)
	}

	return nil
}

func (t *tunnelIDAllocactor) getCurrentTunnelIDs() (map[int32]struct{}, error) {
	vswitchCfgs, err := t.kovsClientset.KubeovsV1alpha1().VSwitchConfigs().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	tunnelIDs := make(map[int32]struct{})
	for _, vswitchCfg := range vswitchCfgs.Items {
		tunID := vswitchCfg.Spec.OverlayTunnelID

		// vswitch configs with tunnel ID 0 require allocation, skip checking them
		if tunID == 0 {
			continue
		}

		err = validateTunnelID(tunID)
		if err != nil {
			klog.Warningf("invalid tunnel ID %d, err: %v", tunID, err)
			continue
		}

		tunnelIDs[tunID] = struct{}{}
	}

	return tunnelIDs, nil
}

func validateTunnelID(tunnelID int32) error {
	if tunnelID < defaultMinTunnelID {
		return errors.New("tunnel ID is less than the minimum ID allowed")
	}

	if tunnelID > defaultMaxTunnelID {
		return errors.New("tunnel ID is greater than the maximum ID allowed")
	}

	return nil
}
