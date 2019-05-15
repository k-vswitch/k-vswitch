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

package connection

import (
	"encoding/binary"

	"github.com/Kmotiko/gofc/ofprotocol/ofp13"
)

func SerializeMessage(msg ofp13.OFMessage) []byte {
	return msg.Serialize()
}

func ParseMessage(buf []byte) ofp13.OFMessage {
	return ofp13.Parse(buf)
}

func MessageLength(buf []byte) int {
	// Length attribute in OFP header is uint16 read in BigEndian
	// buf[2:] because first byte is version, second byte is type and
	// length is next
	// TODO: check length of byte sequence first?
	return int(binary.BigEndian.Uint16(buf[2:]))
}
