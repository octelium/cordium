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

package harness

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/cluster/e2e/harness"
	octelium "github.com/octelium/octelium/octelium-go"
	"google.golang.org/grpc"
)

const (
	cordiumMainService      = "octelium.api.main.cordium.v1.MainService"
	cordiumWorkspaceService = "octelium.api.main.cordium.v1.WorkspaceService"
)

type Actor struct {
	User *corev1.User
	Conn *grpc.ClientConn
}

func (a *Actor) CordiumC() cordiumv1.MainServiceClient {
	return cordiumv1.NewMainServiceClient(a.Conn)
}

func (a *Actor) WorkspaceC() cordiumv1.WorkspaceServiceClient {
	return cordiumv1.NewWorkspaceServiceClient(a.Conn)
}

func CordiumAPIPolicy(name string, services ...string) []*corev1.InlinePolicy {
	quoted := make([]string, 0, len(services))
	for _, svc := range services {
		quoted = append(quoted, strconv.Quote(svc))
	}

	return []*corev1.InlinePolicy{
		{
			Name: name,
			Spec: &corev1.Policy_Spec{
				Rules: []*corev1.Policy_Spec_Rule{
					{
						Name:     "allow",
						Effect:   corev1.Policy_Spec_Rule_ALLOW,
						Priority: -1,
						Condition: &corev1.Condition{
							Type: &corev1.Condition_Match{
								Match: fmt.Sprintf(
									`ctx.namespace.metadata.name == "octelium-api" && `+
										`ctx.request.grpc.serviceFullName in [%s]`,
									strings.Join(quoted, ", ")),
							},
						},
					},
				},
			},
		},
	}
}

func (h *H) NewActor(t *testing.T) *Actor {
	t.Helper()

	usr := h.CreateWorkloadUser(t, &corev1.User_Spec_Authorization{
		InlinePolicies: CordiumAPIPolicy("cordium-api",
			cordiumMainService, cordiumWorkspaceService),
	})

	return &Actor{
		User: usr,
		Conn: h.UserConn(t, usr),
	}
}

func (h *H) UserConn(t *testing.T, usr *corev1.User) *grpc.ClientConn {
	t.Helper()

	cred := h.CreateCredential(t, harness.CredentialOpts{
		User:        usr.Metadata.Name,
		Type:        corev1.Credential_Spec_AUTH_TOKEN,
		SessionType: corev1.Session_Status_CLIENTLESS,
	})

	tkn := h.CredentialToken(t, cred)
	if tkn.GetAuthenticationToken() == nil {
		t.Fatalf("The Credential %s did not yield an authentication token", cred.Metadata.Name)
	}

	oC, err := octelium.NewClient(t.Context(),
		octelium.WithDomain(h.Domain),
		octelium.WithAuthenticator(
			octelium.AuthenticationToken(tkn.GetAuthenticationToken().AuthenticationToken)))
	if err != nil {
		t.Fatalf("Could not build a Client for the User %s: %+v", usr.Metadata.Name, err)
	}

	conn, err := oC.Conn(t.Context())
	if err != nil {
		t.Fatalf("Could not dial the Cluster as the User %s: %+v", usr.Metadata.Name, err)
	}

	t.Cleanup(func() { conn.Close() })

	return conn
}
