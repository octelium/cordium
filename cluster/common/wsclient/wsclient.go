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
