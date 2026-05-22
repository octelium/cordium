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

package ccommon

import (
	"strings"

	pb "github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/pkg/errors"
)

func GetResourceShortName(rsc umetav1.ResourceObjectI) string {
	if rsc == nil || rsc.GetMetadata() == nil {
		return ""
	}

	return doGetShortName(rsc.GetMetadata().Name)
}

func GetResourceRefShortName(itm *metav1.ObjectReference) string {
	if itm == nil {
		return ""
	}

	return doGetShortName(itm.Name)
}

func doGetShortName(arg string) string {
	if arg == "" {
		return ""
	}
	return strings.Split(arg, ".")[0]
}

func ParseVars(raw []string) ([]*pb.Workspace_Spec_Var, error) {
	vars := make([]*pb.Workspace_Spec_Var, 0, len(raw))
	for _, s := range raw {
		name, value, ok := strings.Cut(s, "=")
		if !ok || name == "" {
			return nil, errors.Errorf("invalid --var value %q: expected NAME=VALUE", s)
		}
		vars = append(vars, &pb.Workspace_Spec_Var{
			Name:  name,
			Value: value,
		})
	}
	return vars, nil
}
