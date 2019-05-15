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

package controllers

import (
	"net"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
)

type endpointsHandler struct {
	conn *net.TCPConn
}

func (e *endpointsHandler) OnAdd(obj interface{}) {
	// ignore this event if there are no open flow connections established
	if e.conn == nil {
		return
	}

	ep, ok := obj.(*corev1.Endpoints)
	if !ok {
		return
	}

	klog.Infof("received OnAdd event for endpoint %q", ep.Name)
}

func (e *endpointsHandler) OnUpdate(oldObj, newObj interface{}) {
	// ignore this event if there are no open flow connections established
	if e.conn == nil {
		return
	}

	ep, ok := newObj.(*corev1.Endpoints)
	if !ok {
		return
	}

	klog.Infof("received OnUpdate event for endpoint %q", ep.Name)
}

func (e *endpointsHandler) OnDelete(obj interface{}) {
	// ignore this event if there are no open flow connections established
	if e.conn == nil {
		return
	}

	ep, ok := obj.(*corev1.Endpoints)
	if !ok {
		return
	}

	klog.Infof("received OnDelete event for endpoint %q", ep.Name)
}
