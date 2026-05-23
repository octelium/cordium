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

package wsclient

import (
	"context"
	"net"
	"time"

	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

type GetWorkspaceGRPCClientOpts struct {
	ClientInWorkspace bool
}

func GetWorkspaceGRPCClient(o *GetWorkspaceGRPCClientOpts) (*grpc.ClientConn, error) {
	var socketPath string
	if ldflags.IsTest() {
		socketPath = "/tmp/oct-ws.sock"
	} else if o != nil && o.ClientInWorkspace {
		socketPath = "/run/octelium/workspace.sock"
	} else {
		socketPath = "/octelium/sockets/workspace.sock"
	}

	opts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    time.Duration(5) * time.Minute,
			Timeout: time.Duration(30) * time.Second,
		}),
	}
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	// opts = append(opts, grpc.WithBlock())
	opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, "unix", addr)
	}))

	// TODO: NewClient does NOT work with unix sockets

	grpcConn, err := grpc.Dial(socketPath, opts...)
	if err != nil {
		return nil, errors.Errorf("could not grpc dial: %+v", err)
	}

	return grpcConn, nil
}
