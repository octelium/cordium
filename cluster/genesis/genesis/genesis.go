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

package genesis

import (
	"github.com/octelium/cordium/cluster/common/octeliumc"
	octeliumcinit "github.com/octelium/octelium/cluster/common/octeliumc"
	k8scorev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Genesis struct {
	pod           *k8scorev1.Pod
	podNamespace  string
	k8sC          kubernetes.Interface
	octeliumCInit octeliumcinit.ClientInterface
	octeliumC     octeliumc.ClientInterface
}

func NewGenesis() (*Genesis, error) {
	ret := &Genesis{}

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return nil, err
	}

	k8sClientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	ret.k8sC = k8sClientSet

	if err := ret.setPodNamespace(); err != nil {
		return nil, err
	}
	return ret, nil
}
