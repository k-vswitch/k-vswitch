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

package ports

import (
	"fmt"
	"os/exec"
	"strconv"

	kovsinformers "github.com/kube-ovs/kube-ovs/apis/generated/informers/externalversions/kubeovs/v1alpha1"
	kovslister "github.com/kube-ovs/kube-ovs/apis/generated/listers/kubeovs/v1alpha1"
	kovsv1alpha1 "github.com/kube-ovs/kube-ovs/apis/kubeovs/v1alpha1"

	"k8s.io/klog"
)

const (
	vxlanPortName = "vxlan0"
)

// vxlanPorts watches for VSwitchConfig events and
// creates a vxlan port on the local OVS bridge
type vxlanPorts struct {
	bridgeName string

	localOverlayIP string
	localTunnelID  string

	vswitchLister kovslister.VSwitchConfigLister
}

func NewVxlanPorts(bridgeName string, vswitch *kovsv1alpha1.VSwitchConfig, vswitchInformer kovsinformers.VSwitchConfigInformer) *vxlanPorts {
	return &vxlanPorts{
		bridgeName:     bridgeName,
		localOverlayIP: vswitch.Spec.OverlayIP,
		localTunnelID:  strconv.Itoa(int(vswitch.Spec.OverlayTunnelID)),
		vswitchLister:  vswitchInformer.Lister(),
	}
}

func (v *vxlanPorts) OnAddVSwitch(obj interface{}) {
	vswitch, ok := obj.(*kovsv1alpha1.VSwitchConfig)
	if !ok {
		return
	}

	if err := v.addVxLANPort(vswitch); err != nil {
		klog.Errorf("error adding vxlan port: %v", err)
	}
}

func (v *vxlanPorts) OnUpdateVSwitch(oldObj, newObj interface{}) {
	vswitch, ok := newObj.(*kovsv1alpha1.VSwitchConfig)
	if !ok {
		return
	}

	if err := v.addVxLANPort(vswitch); err != nil {
		klog.Errorf("error adding vxlan port: %v", err)
	}
}

func (v *vxlanPorts) OnDeleteVSwitch(obj interface{}) {
	_, ok := obj.(*kovsv1alpha1.VSwitchConfig)
	if !ok {
		return
	}

	// TODO: handle delete case
}

func (v *vxlanPorts) addVxLANPort(vswitchConfig *kovsv1alpha1.VSwitchConfig) error {
	command := []string{
		"--may-exist", "add-port", v.bridgeName, vxlanPortName,
		"--", "set", "Interface", vxlanPortName, "type=vxlan",
		fmt.Sprintf("option:remote_ip=flow"),
		"option:key=flow",
	}

	out, err := exec.Command("ovs-vsctl", command...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to setup vxlan port %q, err: %v, out: %q", vxlanPortName, err, out)
	}

	return nil
}
