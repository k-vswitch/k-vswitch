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
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

type portCache struct {
	sync.Mutex

	store map[string]portInfo
}

type portInfo struct {
	name   string
	ofport int
	mac    string
}

func NewPortCache() *portCache {
	return &portCache{
		store: make(map[string]portInfo, 0),
	}
}

func portCacheKeyForPod(pod *corev1.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}

func (p *portCache) GetPortInfo(pod *corev1.Pod) (portInfo, error) {
	p.Lock()
	defer p.Unlock()

	key := portCacheKeyForPod(pod)
	port, exists := p.store[key]
	if exists {
		return port, nil
	}

	portName, err := findPort(pod.Namespace, pod.Name)
	if err != nil {
		return portInfo{}, err
	}

	mac, err := macAddrFromPort(portName)
	if err != nil {
		return portInfo{}, err
	}

	ofport, err := ofPortFromName(portName)
	if err != nil {
		return portInfo{}, err
	}

	podPortInfo := portInfo{
		name:   portName,
		mac:    mac,
		ofport: ofport,
	}

	p.store[key] = podPortInfo
	return podPortInfo, nil
}

func (p *portCache) DelPortInfo(pod *corev1.Pod) {
	p.Lock()
	defer p.Unlock()

	key := portCacheKeyForPod(pod)
	delete(p.store, key)
}

func findPort(podNamespace, podName string) (string, error) {
	commands := []string{
		"--format=json", "--column=name", "find", "port",
		fmt.Sprintf("external-ids:k8s_pod_namespace=%s", podNamespace),
		fmt.Sprintf("external-ids:k8s_pod_name=%s", podName),
	}

	out, err := exec.Command("ovs-vsctl", commands...).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get OVS port for %s/%s, err: %v",
			podNamespace, podName, err)
	}

	dbData := struct {
		Data [][]string
	}{}
	if err = json.Unmarshal(out, &dbData); err != nil {
		return "", err
	}

	if len(dbData.Data) == 0 {
		// TODO: might make more sense to not return an error here since
		// CNI delete can be called multiple times.
		return "", fmt.Errorf("OVS port for %s/%s was not found, OVS DB data: %v, output: %q",
			podNamespace, podName, dbData.Data, string(out))
	}

	portName := dbData.Data[0][0]
	return portName, nil
}

func macAddrFromPort(portName string) (string, error) {
	commands := []string{
		"get", "port", portName, "mac",
	}

	out, err := exec.Command("ovs-vsctl", commands...).Output()
	if err != nil {
		return "", fmt.Errorf("failed to get MAC address from OVS port for %q, err: %v, out: %q",
			portName, err, string(out))
	}

	// TODO: validate mac address
	macAddr := strings.TrimSpace(string(out))
	if len(macAddr) > 0 && macAddr[0] == '"' {
		macAddr = macAddr[1:]
	}
	if len(macAddr) > 0 && macAddr[len(macAddr)-1] == '"' {
		macAddr = macAddr[:len(macAddr)-1]
	}

	return macAddr, nil
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
