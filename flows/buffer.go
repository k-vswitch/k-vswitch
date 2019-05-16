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
	"bytes"
	"fmt"
	"os/exec"
	"time"

	"k8s.io/klog"
)

type FlowsBuffer struct {
	buffer *bytes.Buffer
}

func NewFlowsBuffer() *FlowsBuffer {
	buffer := bytes.NewBuffer(nil)

	return &FlowsBuffer{
		buffer: buffer,
	}
}

func (f *FlowsBuffer) AddFlow(flow *Flow) {
	f.buffer.WriteString(fmt.Sprintf("%s", flow))
	f.buffer.WriteByte('\n')
}

func (f *FlowsBuffer) String() string {
	return f.buffer.String()
}

func (f *FlowsBuffer) Reset() {
	f.buffer.Reset()
}

func (f *FlowsBuffer) SyncFlows(bridge string) error {
	startTime := time.Now()

	commands := []string{
		"-O", "OpenFlow13",
		"replace-flows", bridge, "-",
	}

	cmd := exec.Command("ovs-ofctl", commands...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("error creating stdin pipe: %v", err)
	}

	go func() {
		defer stdin.Close()
		_, err = stdin.Write(f.buffer.Bytes())
		if err != nil {
			klog.Errorf("error writing buffer to pipe: %v", err)
		}
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to sync flows: %v, out: %q", err, string(out))
	}

	klog.V(5).Infof("replace-flow took %s", time.Since(startTime).String())
	return nil
}
