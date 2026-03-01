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

package mains

import (
	"context"

	"github.com/asaskevich/govalidator"
	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
)

func (s *Server) GetUserConfig(ctx context.Context, req *cordiumv1.GetUserConfigRequest) (*cordiumv1.UserConfig, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	name := ovutils.GetUserConfigName(umetav1.GetObjectReference(i.User))

	ret, err := s.octeliumC.CordiumC().GetUserConfig(ctx, &rmetav1.GetOptions{
		Name: name,
	})
	if err == nil {
		return ret, nil
	}
	if !grpcerr.IsNotFound(err) {
		return nil, grpcutils.InternalWithErr(err)
	}

	ret, err = s.octeliumC.CordiumC().CreateUserConfig(ctx, &cordiumv1.UserConfig{
		Metadata: &metav1.Metadata{
			Name: name,
		},
		Spec: &cordiumv1.UserConfig_Spec{},
		Status: &cordiumv1.UserConfig_Status{
			UserRef: umetav1.GetObjectReference(i.User),
		},
	})
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	return ret, nil
}

func (s *Server) UpdateUserConfig(ctx context.Context, req *cordiumv1.UserConfig) (*cordiumv1.UserConfig, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	item, err := s.GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{})
	if err != nil {
		return nil, err
	}

	{
		if req.Spec.EnvVars != nil {
			if len(req.Spec.EnvVars) > 128 {
				return nil, serr.InvalidArg("Too many container env vars")
			}

			for _, envVar := range req.Spec.EnvVars {
				if envVar.Key == "" {
					return nil, serr.InvalidArg("Env variable cannot have an empty key")
				}
				if !govalidator.IsASCII(envVar.Key) {
					return nil, serr.InvalidArg("Invalid env var key")
				}

				if len(envVar.Key) > 64 {
					return nil, serr.InvalidArg("Too long env var key")
				}

				switch envVar.Type.(type) {
				case *cordiumv1.UserConfig_Spec_EnvVar_FromUserSecret:
					if envVar.GetFromUserSecret() == "" {
						return nil, serr.InvalidArg("Empty Secret name for the env variable with key: %s", envVar.Key)
					}

					sec, err := s.octeliumC.CordiumC().GetUserSecret(ctx, &rmetav1.GetOptions{
						Name: envVar.GetFromUserSecret(),
					})
					if err != nil {
						return nil, serr.K8sNotFoundOrInternalWithErr(err)
					}

					if sec.Status.UserRef.Uid != i.User.Metadata.Uid {
						return nil, grpcutils.InvalidArg("The UserSecret does not exist: %s", envVar.GetFromUserSecret())
					}

				case *cordiumv1.UserConfig_Spec_EnvVar_Value:
					if len(envVar.GetValue()) == 0 {
						return nil, serr.InvalidArg("Empty value for env var: %s", envVar.Key)
					}

					if len(envVar.GetValue()) > 1024 {
						return nil, serr.InvalidArg("Empty value for env var: %s", envVar.Key)
					}
				default:
					return nil, serr.InvalidArg("No env variable value for the key: %s", envVar.Key)
				}
			}
		}
	}

	item.Spec = req.Spec

	if item.Spec.PreferredRegion != "" {
		rgn, err := s.octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{
			Name: item.Spec.PreferredRegion,
		})
		if err != nil {
			return nil, err
		}
		if !regionHasCordium(rgn) {
			return nil, serr.InvalidArg("This Region does not host Workspaces")
		}

		item.Status.PreferredRegionRef = umetav1.GetObjectReference(rgn)
	} else {
		item.Status.PreferredRegionRef = nil
	}

	ret, err := s.octeliumC.CordiumC().UpdateUserConfig(ctx, item)
	if err != nil {
		return nil, grpcutils.InternalWithErr(err)
	}

	return ret, nil
}
