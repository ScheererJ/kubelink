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
	"github.com/mandelsoft/kubelink/pkg/iptables"
)

const CHAIN_PREFIX = "KUBELINK-"

const LINKS_CHAIN = CHAIN_PREFIX + "LINKS"
const TABLE_LINKS_CHAIN = "mangle"

const FIREWALL_CHAIN = CHAIN_PREFIX + "FIREWALL"
const TABLE_FIREWALL_CHAIN = "filter"

const DROP_CHAIN = CHAIN_PREFIX + "DROP"
const TABLE_DROP_CHAIN = TABLE_FIREWALL_CHAIN

const MARK_DROP_CHAIN = CHAIN_PREFIX + "MARK-DROP"
const TABLE_MARK_DROP_CHAIN = TABLE_LINKS_CHAIN

const FW_LINK_CHAIN_PREFIX = CHAIN_PREFIX + "FW-"
const TABLE_LINK_CHAIN = TABLE_MARK_DROP_CHAIN

const DROP_ACTION = "DROP"

type RuleDef struct {
	Table  string
	Chain  string
	Rule   iptables.Rule
	Before string
}

func FirewallEmbedding() []RuleDef {
	opt := iptables.Opt("-m", "comment", "--comment", "kubelink firewall rules")
	before := ""
	if TABLE_LINKS_CHAIN != "mangle" {
		before = "KUBE-SERVICES"
	}
	if DROP_ACTION == MARK_DROP_CHAIN {
		return []RuleDef{
			RuleDef{TABLE_LINKS_CHAIN, "PREROUTING", iptables.Rule{opt, iptables.Opt("-j", LINKS_CHAIN)}, before},
			RuleDef{TABLE_FIREWALL_CHAIN, "FORWARD", iptables.Rule{opt, iptables.Opt("-j", FIREWALL_CHAIN)}, "KUBE-FORWARD"},
			RuleDef{TABLE_FIREWALL_CHAIN, "OUTPUT", iptables.Rule{opt, iptables.Opt("-j", FIREWALL_CHAIN)}, ""},
		}
	} else {
		return []RuleDef{
			RuleDef{TABLE_LINKS_CHAIN, "PREROUTING", iptables.Rule{opt, iptables.Opt("-j", LINKS_CHAIN)}, before},
		}
	}
}
