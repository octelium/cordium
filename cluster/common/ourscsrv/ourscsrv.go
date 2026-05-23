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
