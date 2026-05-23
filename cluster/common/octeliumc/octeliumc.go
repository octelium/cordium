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

package octeliumc

import (
	"context"
	"fmt"
	"os"

	"github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/rsc/rcachev1"
	"github.com/octelium/octelium/apis/rsc/rcordiumv1"
	"github.com/octelium/octelium/apis/rsc/rcorev1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/apis/rsc/rratelimitv1"
	"github.com/octelium/octelium/cluster/common/octeliumc"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Client struct {
	coreC      rcorev1.ResourceServiceClient
	cacheC     rcachev1.MainServiceClient
	rateLimitC rratelimitv1.MainServiceClient
	cordiumC   rcordiumv1.ResourceServiceClient

	coreV1UtilsC    *coreV1UtilsC
	cordiumV1UtilsC *cordiumV1UtilsC
}

type CoreV1Utils interface {
	GetClusterConfig(ctx context.Context) (*corev1.ClusterConfig, error)
}

type CordiumV1Utils interface {
	GetClusterConfig(ctx context.Context) (*cordiumv1.ClusterConfig, error)
}

type Opts struct {
	Addr string
}

func NewClient(ctx context.Context, opts *Opts) (*Client, error) {

	var host string

	if opts == nil {
		opts = &Opts{}
	}

	if opts.Addr != "" {
		host = opts.Addr
	} else if ldflags.IsTest() {
		host = fmt.Sprintf("localhost:%s", os.Getenv("OCTELIUM_TEST_RSCSERVER_PORT"))
	} else {
		host = fmt.Sprintf("%s.octelium.svc:8080", components.CordiumComponent(components.RscServer))
	}

	zap.L().Debug("Initializing new Cordium client", zap.String("host", host))

	tOpts, err := octeliumc.DefaultDialOpts(ctx)
	if err != nil {
		return nil, err
	}

	grpcConn, err := grpc.NewClient(
		host, tOpts...,
	)
	if err != nil {
		return nil, err
	}

	ret := &Client{
		coreC:    rcorev1.NewResourceServiceClient(grpcConn),
		cacheC:   rcachev1.NewMainServiceClient(grpcConn),
		cordiumC: rcordiumv1.NewResourceServiceClient(grpcConn),

		coreV1UtilsC:    &coreV1UtilsC{},
		cordiumV1UtilsC: &cordiumV1UtilsC{},
	}

	ret.coreV1UtilsC.c = ret.coreC
	ret.cordiumV1UtilsC.c = ret.cordiumC

	return ret, nil
}

func (c *Client) CoreC() rcorev1.ResourceServiceClient {
	return c.coreC
}

func (c *Client) CacheC() rcachev1.MainServiceClient {
	return c.cacheC
}

func (c *Client) RateLimitC() rratelimitv1.MainServiceClient {
	return c.rateLimitC
}

func (c *Client) CordiumC() rcordiumv1.ResourceServiceClient {
	return c.cordiumC
}

func (c *Client) CoreV1Utils() octeliumc.CoreV1Utils {
	return c.coreV1UtilsC
}

func (c *Client) CordiumV1Utils() CordiumV1Utils {
	return c.cordiumV1UtilsC
}

type ClientInterface interface {
	octeliumc.ClientInterface
	CordiumC() rcordiumv1.ResourceServiceClient
	CordiumV1Utils() CordiumV1Utils
}

type coreV1UtilsC struct {
	c rcorev1.ResourceServiceClient
}

func (c *coreV1UtilsC) GetClusterConfig(ctx context.Context) (*corev1.ClusterConfig, error) {
	return c.c.GetClusterConfig(ctx, &rmetav1.GetOptions{
		Name: "default",
	})
}

type cordiumV1UtilsC struct {
	c rcordiumv1.ResourceServiceClient
}

func (c *cordiumV1UtilsC) GetClusterConfig(ctx context.Context) (*cordiumv1.ClusterConfig, error) {
	return c.c.GetClusterConfig(ctx, &rmetav1.GetOptions{
		Name: "default",
	})
}
