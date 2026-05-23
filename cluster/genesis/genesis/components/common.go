/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package components

import (
	"fmt"
	"time"

	"github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/octelium/apis/main/corev1"
	gcomponents "github.com/octelium/octelium/cluster/genesis/genesis/components"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	k8scorev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func defaultAnnotations() map[string]string {

	ret := make(map[string]string)
	ret["octelium.com/last-touched"] = time.Now().Format(time.RFC3339)

	return ret
}

func getNodeSelectorDataPlane(c *corev1.ClusterConfig) map[string]string {
	return map[string]string{
		"octelium.com/node-mode-dataplane": "",
	}
}

func getNodeSelectorControlPlane(c *corev1.ClusterConfig) map[string]string {
	return map[string]string{
		"octelium.com/node-mode-controlplane": "",
	}
}

var tcpProtocol = k8scorev1.ProtocolTCP
var udpProtocol = k8scorev1.ProtocolUDP

func getImagePullSecrets() []k8scorev1.LocalObjectReference {
	return nil

	/*
		return []k8scorev1.LocalObjectReference{
			{
				Name: "octelium-regcred",
			},
		}
	*/
}

func getComponentName(arg string) string {
	return fmt.Sprintf("cordium-%s", arg)
}

const ns = "octelium"

func getComponentLabels(arg string) map[string]string {
	return map[string]string{
		"app":                         "octelium",
		"octelium.com/app":            "cordium",
		"octelium.com/component":      arg,
		"octelium.com/component-type": "cluster",
	}
}

func getAnnotations() map[string]string {
	return map[string]string{
		"octelium.com/install-uid": utilrand.GetRandomStringLowercase(8),
	}
}

const componentNocturne = components.Nocturne
const componentRscServer = components.RscServer

func getDefaultRequests() k8scorev1.ResourceList {
	return k8scorev1.ResourceList{
		k8scorev1.ResourceMemory: resource.MustParse("5Mi"),
		k8scorev1.ResourceCPU:    resource.MustParse("10m"),
	}
}

func getDefaultLimits() k8scorev1.ResourceList {
	return k8scorev1.ResourceList{
		k8scorev1.ResourceMemory: resource.MustParse("700Mi"),
		k8scorev1.ResourceCPU:    resource.MustParse("1200m"),
	}
}

func getDefaultResourceRequirements() k8scorev1.ResourceRequirements {
	return k8scorev1.ResourceRequirements{
		Requests: getDefaultRequests(),
		Limits:   getDefaultLimits(),
	}
}

type CommonOpts struct {
	gcomponents.CommonOpts
	Version string
}

func getDefaultLivenessProbe() *k8scorev1.Probe {
	return gcomponents.MainLivenessProbe()
}
