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

package rscserver

import (
	"context"

	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rcordiumv1"
	"github.com/octelium/octelium/apis/rsc/rcorev1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/healthcheck"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/cluster/rscserver/rscserver"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Server struct {
	inner *rscserver.Server
}

func NewServer(ctx context.Context) (*Server, error) {

	zap.L().Debug("Initializing Cordium RscServer")
	ret := &Server{}

	opts := &rscserver.Opts{
		RegisterResourceFn: func(s grpc.ServiceRegistrar) error {
			rcorev1.RegisterResourceServiceServer(s, &struct {
				rcorev1.UnimplementedResourceServiceServer
			}{})

			rcordiumv1.RegisterResourceServiceServer(s, &struct {
				rcordiumv1.UnimplementedResourceServiceServer
			}{})

			return nil
		},

		NewResourceObject:     ovutils.NewResourceObject,
		NewResourceObjectList: ovutils.NewResourceObjectList,
	}

	inner, err := rscserver.NewServer(ctx, opts)
	if err != nil {
		return nil, err
	}
	ret.inner = inner

	return ret, nil
}

func (s *Server) CreateResource(ctx context.Context, req umetav1.ResourceObjectI, api, version, kind string) (umetav1.ResourceObjectI, error) {
	return s.inner.CreateResource(ctx, req, api, version, kind)
}

func (s *Server) GetResource(ctx context.Context, req *rmetav1.GetOptions, api, version, kind string) (umetav1.ResourceObjectI, error) {
	return s.inner.GetResource(ctx, req, api, version, kind)
}

func (s *Server) Run(ctx context.Context) error {
	return s.inner.Run(ctx)
}

func (s *Server) setInitResources(ctx context.Context) error {
	if _, err := s.GetResource(ctx, &rmetav1.GetOptions{
		Name: "default",
	}, ucordiumv1.API, ucordiumv1.Version, ucordiumv1.KindClusterConfig); err != nil {
		if !grpcerr.IsNotFound(err) {
			return err
		}

		zap.L().Debug("Initializing a default cluster-config for cordium")

		if _, err := s.CreateResource(ctx, &cordiumv1.ClusterConfig{
			ApiVersion: ucordiumv1.APIVersion,
			Kind:       ucordiumv1.KindClusterConfig,
			Metadata: &metav1.Metadata{
				Name: "default",
			},
			Spec: &cordiumv1.ClusterConfig_Spec{
				Space: &cordiumv1.ClusterConfig_Spec_Space{
					Ownership: &cordiumv1.ClusterConfig_Spec_Space_Ownership{
						Rules: []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule{
							{
								Condition: &cordiumv1.Condition{
									Type: &cordiumv1.Condition_MatchAny{
										MatchAny: true,
									},
								},
								Effect: cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_ALLOW,
							},
						},
					},
				},
			},
			Status: &cordiumv1.ClusterConfig_Status{},
		},
			ucordiumv1.API, ucordiumv1.Version, ucordiumv1.KindClusterConfig); err != nil {
			return err
		}

	}
	return nil
}

func Run(ctx context.Context) error {

	zap.S().Debug("Starting Cordium Resource server")

	srv, err := NewServer(ctx)
	if err != nil {
		return err
	}

	if err := srv.setInitResources(ctx); err != nil {
		return errors.Errorf("Could not set init resources")
	}

	zap.S().Debug("starting gRPC server...")

	if err := srv.Run(ctx); err != nil {
		return err
	}

	healthcheck.Run(vutils.HealthCheckPortMain)
	zap.S().Infof("Cordium Resource Server is now running...")

	<-ctx.Done()

	return nil
}
