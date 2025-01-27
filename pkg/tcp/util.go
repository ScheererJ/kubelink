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

package tcp

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"

	"github.com/vishvananda/netlink"
)

// onceCloseListener wraps a net.Listener, protecting it from
// multiple Close calls.
type onceCloseListener struct {
	net.Listener
	once     sync.Once
	closeErr error
}

func (oc *onceCloseListener) Close() error {
	oc.once.Do(oc.close)
	return oc.closeErr
}

func (oc *onceCloseListener) close() { oc.closeErr = oc.Listener.Close() }

// cloneTLSConfig returns a shallow clone of cfg, or a new zero tls.Config if
// cfg is nil. This is safe to call even if cfg is in active use by a TLS
// client or server.
func cloneTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{}
	}
	return cfg.Clone()
}

func EqualIP(a, b net.IP) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(b)
}

func CloneIP(ip net.IP) net.IP {
	return append(ip[:0:0], ip...)
}

func SubIP(cidr *net.IPNet, n int) net.IP {
	ip := CloneIP(cidr.IP)

	i := len(ip) - 1
	for n > 0 {
		n += int(ip[i])
		ip[i] = uint8(n % 256)
		n = n / 256
		i--
	}
	return ip
}

func EqualCIDR(a, b *net.IPNet) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if !a.IP.Equal(b.IP) {
		return false
	}
	if !net.IP(a.Mask).Equal(net.IP(b.Mask)) {
		return false
	}
	return true
}

func CIDRNet(cidr *net.IPNet) *net.IPNet {
	net := *cidr
	net.IP = cidr.IP.Mask(cidr.Mask)
	return &net
}

func CIDRIP(cidr *net.IPNet, ip net.IP) *net.IPNet {
	net := *cidr
	net.IP = ip
	return &net
}

////////////////////////////////////////////////////////////////////////////////

type CIDRList []*net.IPNet

func (this *CIDRList) String() string {
	sep := "["
	end := ""
	s := ""
	for _, c := range *this {
		s = fmt.Sprintf("%s%s%s", s, sep, c)
		sep = ","
		end = "]"
	}
	return s + end
}

func (this *CIDRList) Add(cidrs ...*net.IPNet) {
	*this = append(*this, cidrs...)
}

func (this *CIDRList) IsEmpty() bool {
	return len(*this) == 0
}

func (this *CIDRList) IsSet() bool {
	return *this != nil
}

func (this *CIDRList) Contains(ip net.IP) bool {
	for _, c := range *this {
		if c.Contains(ip) {
			return true
		}
	}
	return false
}

////////////////////////////////////////////////////////////////////////////////

func Family(ip net.IP) int {
	if ip.To4() == nil {
		return netlink.FAMILY_V6
	}
	return netlink.FAMILY_V4
}
