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

package ourscsrv

import (
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"google.golang.org/protobuf/types/known/structpb"
)

func FilterStatusSpaceUID(uid string) *rmetav1.ListOptions_Filter {
	return &rmetav1.ListOptions_Filter{
		Field: "status.spaceRef.uid",
		Op:    rmetav1.ListOptions_Filter_OP_EQ,
		Value: &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: uid,
			},
		},
	}
}

func FilterBySpace(u *cordiumv1.Space) *rmetav1.ListOptions {

	return &rmetav1.ListOptions{
		Filters: []*rmetav1.ListOptions_Filter{
			FilterStatusSpaceUID(u.Metadata.Uid),
		},
	}
}

func FilterBySpaceRef(u *metav1.ObjectReference) *rmetav1.ListOptions {
	return &rmetav1.ListOptions{
		Filters: []*rmetav1.ListOptions_Filter{
			FilterStatusSpaceUID(u.Uid),
		},
	}
}

func FilterStatusEnvironmentUID(uid string) *rmetav1.ListOptions_Filter {
	return &rmetav1.ListOptions_Filter{
		Field: "status.environmentRef.uid",
		Op:    rmetav1.ListOptions_Filter_OP_EQ,
		Value: &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: uid,
			},
		},
	}
}

/*
func FilterByEnvironment(u *cordiumv1.Environment) *rmetav1.ListOptions {

	return &rmetav1.ListOptions{
		Filters: []*rmetav1.ListOptions_Filter{
			FilterStatusEnvironmentUID(u.Metadata.Uid),
		},
	}
}

func FilterByEnvironmentRef(u *metav1.ObjectReference) *rmetav1.ListOptions {
	return &rmetav1.ListOptions{
		Filters: []*rmetav1.ListOptions_Filter{
			FilterStatusEnvironmentUID(u.Uid),
		},
	}
}
*/

func FilterStatusTemplateUID(uid string) *rmetav1.ListOptions_Filter {
	return &rmetav1.ListOptions_Filter{
		Field: "status.templateRef.uid",
		Op:    rmetav1.ListOptions_Filter_OP_EQ,
		Value: &structpb.Value{
			Kind: &structpb.Value_StringValue{
				StringValue: uid,
			},
		},
	}
}

func FilterByTemplate(u *cordiumv1.Template) *rmetav1.ListOptions {

	return &rmetav1.ListOptions{
		Filters: []*rmetav1.ListOptions_Filter{
			FilterStatusTemplateUID(u.Metadata.Uid),
		},
	}
}

func FilterByTemplateRef(u *metav1.ObjectReference) *rmetav1.ListOptions {
	return &rmetav1.ListOptions{
		Filters: []*rmetav1.ListOptions_Filter{
			FilterStatusTemplateUID(u.Uid),
		},
	}
}
