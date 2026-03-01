/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package genesis

import (
	octeliumcinit "github.com/octelium/octelium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Genesis struct {
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

	return ret, nil
}
