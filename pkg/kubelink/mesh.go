/*
 * Copyright 2021 Mandelsoft. All rights reserved.
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

package kubelink

import (
	"net"

	"github.com/mandelsoft/kubelink/pkg/tcp"
)

type Mesh struct {
	name           string
	clusterName    string
	clusterAddress *net.IPNet
	cidr           *net.IPNet
	dnsInfo        LinkDNSInfo
}

func NewMeshInfo(link *Link) *Mesh {
	return &Mesh{
		name:           link.Name.mesh,
		clusterName:    link.Name.name,
		clusterAddress: link.ClusterAddress,
		cidr:           tcp.CIDRNet(link.ClusterAddress),
		dnsInfo:        link.LinkDNSInfo,
	}
}

func (this *Mesh) Name() string {
	return this.name
}

func (this *Mesh) ClusterName() string {
	return this.clusterName
}

func (this *Mesh) ClusterAddress() *net.IPNet {
	return this.clusterAddress
}

func (this *Mesh) CIDR() *net.IPNet {
	return this.cidr
}

func (this *Mesh) DNSIP() net.IP {
	return this.dnsInfo.DnsIP
}

func (this *Mesh) ClusterDomain() string {
	return this.dnsInfo.ClusterDomain
}

func (this *Mesh) PropagateDNS() bool {
	return this.dnsInfo.DNSPropagation && this.clusterName != ""
}
