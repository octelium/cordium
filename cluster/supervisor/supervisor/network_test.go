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

package supervisor

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"
	"testing"

	"github.com/octelium/cordium/cluster/common/netpolicy"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

type scriptRecorder struct {
	mu      sync.Mutex
	scripts []string
	err     error
}

func (r *scriptRecorder) fn(ctx context.Context, script string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.scripts = append(r.scripts, script)
	return nil
}

func (r *scriptRecorder) get() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.scripts...)
}

func newNetPolicyServer(t *testing.T, rec *scriptRecorder) *Server {
	ctx := context.Background()

	srv, err := NewServer(ctx)
	assert.Nil(t, err, "%+v", err)

	srv.netPolicyPath = path.Join(t.TempDir(), "policy.sock")
	srv.netPolicyScriptFn = rec.fn
	srv.octeliumUID = 543210

	err = srv.doRunNetworkPolicyServer(ctx)
	assert.Nil(t, err, "%+v", err)

	t.Cleanup(func() {
		srv.closeNetworkPolicyServer()
	})

	return srv
}

func TestSetNetworkPolicy(t *testing.T) {
	ctx := context.Background()

	rec := &scriptRecorder{}
	srv := newNetPolicyServer(t, rec)

	srv.spec = &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Network: &cordiumv1.Workspace_Spec_Runtime_Network{
				Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
					DefaultAction: cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
					Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
						{
							Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
							Cidrs:  []string{"203.0.113.0/24"},
							Ports:  []uint32{443},
						},
					},
				},
			},
		},
	}

	err := srv.doSetNetworkPolicy(ctx)
	assert.Nil(t, err, "%+v", err)

	scripts := rec.get()
	assert.Equal(t, 1, len(scripts))

	assert.True(t, strings.Contains(scripts[0], "meta skuid != 543210 accept"))
	assert.True(t, strings.Contains(scripts[0],
		"ip daddr { 203.0.113.0/24 } meta l4proto { tcp, udp } th dport { 443 } accept"))
	assert.True(t, strings.Contains(scripts[0], "meta l4proto tcp th sport 2022 accept"))

	for _, port := range getReservedPorts() {
		assert.True(t, strings.Contains(scripts[0],
			fmt.Sprintf("meta l4proto %s th sport %d accept", port.protocol, port.port)))
	}
	assert.True(t, srv.netPolicyApplied)
}

func TestSetNetworkPolicyNoSpec(t *testing.T) {
	ctx := context.Background()

	rec := &scriptRecorder{}
	srv := newNetPolicyServer(t, rec)

	srv.spec = &cordiumv1.Workspace_Spec{}

	err := srv.doSetNetworkPolicy(ctx)
	assert.Nil(t, err, "%+v", err)

	scripts := rec.get()
	assert.Equal(t, 1, len(scripts))
	assert.True(t, strings.Contains(scripts[0], "ip daddr { 10.0.0.0/8"))
}

func TestSetNetworkPolicyFailsClosed(t *testing.T) {
	ctx := context.Background()

	rec := &scriptRecorder{err: errors.Errorf("nft is unavailable")}
	srv := newNetPolicyServer(t, rec)

	srv.spec = &cordiumv1.Workspace_Spec{}

	err := srv.doSetNetworkPolicy(ctx)
	assert.NotNil(t, err)
	assert.True(t, strings.Contains(err.Error(), "nft is unavailable"))
	assert.False(t, srv.netPolicyApplied)
}

func TestSetNetworkPolicyInvalidSpec(t *testing.T) {
	ctx := context.Background()

	rec := &scriptRecorder{}
	srv := newNetPolicyServer(t, rec)

	srv.spec = &cordiumv1.Workspace_Spec{
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Network: &cordiumv1.Workspace_Spec_Runtime_Network{
				Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
					Rules: []*cordiumv1.Workspace_Spec_Runtime_Network_Rule{
						{
							Action: cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW,
							Cidrs:  []string{"not-a-cidr"},
						},
					},
				},
			},
		},
	}

	err := srv.doSetNetworkPolicy(ctx)
	assert.NotNil(t, err)
	assert.Equal(t, 0, len(rec.get()))
	assert.False(t, srv.netPolicyApplied)
}

func TestNetworkPolicyTeardown(t *testing.T) {
	ctx := context.Background()

	rec := &scriptRecorder{}
	srv := newNetPolicyServer(t, rec)

	err := srv.teardownNetworkPolicy(ctx)
	assert.Nil(t, err, "%+v", err)
	assert.Equal(t, 0, len(rec.get()))

	srv.spec = &cordiumv1.Workspace_Spec{}
	err = srv.doSetNetworkPolicy(ctx)
	assert.Nil(t, err, "%+v", err)

	err = srv.teardownNetworkPolicy(ctx)
	assert.Nil(t, err, "%+v", err)
	assert.False(t, srv.netPolicyApplied)

	scripts := rec.get()
	assert.Equal(t, 2, len(scripts))
	assert.Equal(t, netpolicy.RenderNFTTeardown(), scripts[1])
}

func TestCompileNetworkPolicyUsesServerUID(t *testing.T) {
	rec := &scriptRecorder{}
	srv := newNetPolicyServer(t, rec)
	srv.octeliumUID = 111222

	networkBytes, err := proto.Marshal(&cordiumv1.Workspace_Spec_Runtime_Network{
		Egress: &cordiumv1.Workspace_Spec_Runtime_Network_Egress{
			DefaultAction: cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY,
		},
	})
	assert.Nil(t, err)

	policy, err := srv.compileNetworkPolicy(&netPolicyRequest{Network: networkBytes})
	assert.Nil(t, err, "%+v", err)

	assert.Equal(t, 111222, policy.UID)
	assert.Equal(t, netpolicy.DefaultActionDeny, policy.DefaultAction)
	assert.Equal(t, len(getReservedPorts()), len(policy.SystemSourcePorts))
	assert.Nil(t, policy.Check())
}

func TestCompileNetworkPolicyInvalidPayload(t *testing.T) {
	rec := &scriptRecorder{}
	srv := newNetPolicyServer(t, rec)

	_, err := srv.compileNetworkPolicy(&netPolicyRequest{
		Network: []byte{0xff, 0xff, 0xff, 0xff},
	})
	assert.NotNil(t, err)
}

func TestReservedPorts(t *testing.T) {
	ports := getReservedPorts()
	assert.Equal(t, 3, len(ports))

	systemPorts := getSystemSourcePorts()
	assert.Equal(t, len(ports), len(systemPorts))

	for i, port := range ports {
		assert.Equal(t, port.protocol, systemPorts[i].Protocol)
		assert.Equal(t, uint32(port.port), systemPorts[i].Port)
	}
}
