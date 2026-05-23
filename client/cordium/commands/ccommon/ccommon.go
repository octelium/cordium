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
