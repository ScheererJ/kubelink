/*
 * Copyright 2020 Mandelsoft. All rights reserved.
 *  This file is licensed under the Apache Software License, v. 2 except as noted
 *  otherwise in the LICENSE file
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package tunnel

import (
	"encoding/binary"
	"net"
)

var (
	networkOrder = binary.BigEndian
)

func HtoNl(val uint32) []byte {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, val)
	return bytes
}

func HtoNs(val uint16) []byte {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, val)
	return bytes
}

func NtoHl(buf []byte) uint32 {
	return binary.BigEndian.Uint32(buf)
}

func NtoHs(buf []byte) uint16 {
	return binary.BigEndian.Uint16(buf)
}

func BroadcastAddress(ipNet net.IPNet) net.IP {
	ip := make(net.IP, len(ipNet.IP), len(ipNet.IP))
	copy(ip, ipNet.IP)
	for i, b := range ipNet.Mask {
		ip[i] &= b
	}
	return ip
}