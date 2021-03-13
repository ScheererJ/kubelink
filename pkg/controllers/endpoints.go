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

package controllers

import (
	"net"

	"github.com/gardener/controller-manager-library/pkg/resources"
	"github.com/gardener/controller-manager-library/pkg/utils"
	core "k8s.io/api/core/v1"
)

var ENDPOINTS = resources.NewGroupKind("", "Endpoints")

func GetEndpoints(logger *utils.Notifier, obj resources.Object) []net.IP {
	var result []net.IP
	ep := obj.Data().(*core.Endpoints)
	logger.Add(false, "checking %d subsets", len(ep.Subsets))
	for _, sub := range ep.Subsets {
		var port *core.EndpointPort
		for _, p := range sub.Ports {
			if port == nil || p.Name == "wireguard" || p.Name == "bridge" {
				if p.Name == "wireguard" || p.Name == "bridge" {
					logger.Add(false, "found port %q", p.Name)
				} else {
					logger.Add(false, "found fallback port %q", p.Name)
				}
				tmp := p
				port = &tmp
			}
		}
		if port != nil {
			for _, a := range sub.Addresses {
				logger.Add(false, "found address %q", a.IP)
				ip := net.ParseIP(a.IP)
				if ip != nil {
					result = append(result, ip)
				}
			}
		} else {
			logger.Infof("no matching port found in subset")
		}
	}
	return result
}