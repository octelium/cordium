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

package ovutils

import (
	"fmt"

	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/apiutils/ucorev1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

func NewResourceObject(api, version, kind string) (umetav1.ResourceObjectI, error) {
	switch api {
	case ucorev1.API:
		return ucorev1.NewObject(kind)
	case ucordiumv1.API:
		return ucordiumv1.NewObject(kind)
	default:
		return nil, errors.Errorf("Invalid API: %s", api)
	}
}

func NewResourceObjectList(api, version, kind string) (proto.Message, error) {
	switch api {
	case ucorev1.API:
		return ucorev1.NewObjectList(kind)
	case ucordiumv1.API:
		return ucordiumv1.NewObjectList(kind)
	default:
		return nil, errors.Errorf("Invalid API: %s", api)
	}
}

const ExtInfoLabel = "cordium"

func IsPrivateRegistry() bool {
	return true
}

func ToCordiumCondition(in *corev1.Condition) (*cordiumv1.Condition, error) {
	jsn, err := pbutils.MarshalJSON(in, false)
	if err != nil {
		return nil, err
	}

	ret := &cordiumv1.Condition{}

	if err := pbutils.UnmarshalJSON(jsn, ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func ToCoreCondition(in *cordiumv1.Condition) (*corev1.Condition, error) {
	jsn, err := pbutils.MarshalJSON(in, false)
	if err != nil {
		return nil, err
	}

	ret := &corev1.Condition{}

	if err := pbutils.UnmarshalJSON(jsn, ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func GetUserConfigName(usrRef *metav1.ObjectReference) string {
	if usrRef == nil || usrRef.Name == "" {
		return ""
	}

	return fmt.Sprintf("default.%s", usrRef.Name)
}
