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

package portal

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"context"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestHandleAuthGitProviderBegin(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	svc, err := adminSrv.CreateService(ctx, tests.GenService("default"))
	assert.Nil(t, err)

	os.Setenv("OCTELIUM_SVC_UID", svc.Metadata.Uid)

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC,
		adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	srv, err := newServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err, "%+v", err)

	{
		req := httptest.NewRequest("POST", "http://localhost/auth/v1/begin", nil)
		req.Header.Set("X-Octelium-Session-Uid", usr.Session.Metadata.Uid)
		w := httptest.NewRecorder()

		req = req.WithContext(context.WithValue(req.Context(), middlewares.CtxRequestContext, &middlewares.RequestContext{
			Session: usr.Session,
		}))

		srv.handleAuthGitProviderBegin(w, req)

		resp := w.Result()
		assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	}

	{
		req := httptest.NewRequest("POST",
			fmt.Sprintf("http://localhost/auth/v1/begin/%s", utilrand.GetRandomStringLowercase(8)), nil)
		req.Header.Set("X-Octelium-Session-Uid", usr.Session.Metadata.Uid)
		w := httptest.NewRecorder()

		req = req.WithContext(context.WithValue(req.Context(), middlewares.CtxRequestContext, &middlewares.RequestContext{
			Session: usr.Session,
		}))

		srv.handleAuthGitProviderBegin(w, req)

		resp := w.Result()
		assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	}

	{
		req := httptest.NewRequest("POST",
			fmt.Sprintf("http://localhost/auth/v1/begin/%s", vutils.UUIDv4()), nil)
		req.Header.Set("X-Octelium-Session-Uid", usr.Session.Metadata.Uid)
		w := httptest.NewRecorder()

		req = req.WithContext(context.WithValue(req.Context(), middlewares.CtxRequestContext, &middlewares.RequestContext{
			Session: usr.Session,
		}))

		srv.handleAuthGitProviderBegin(w, req)

		resp := w.Result()
		assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	}

	{

		org, err := srv.octeliumC.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		_, err = srv.octeliumC.CordiumC().CreateMembership(ctx, &cordiumv1.Membership{
			Metadata: &metav1.Metadata{
				Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(org), umetav1.GetObjectReference(usr.Usr)),
			},
			Spec: &cordiumv1.Membership_Spec{
				Role: cordiumv1.Membership_Spec_ADMIN,
			},
			Status: &cordiumv1.Membership_Status{
				SpaceRef: umetav1.GetObjectReference(org),
				UserRef:  umetav1.GetObjectReference(usr.Usr),
			},
		})
		assert.Nil(t, err)

		sec, err := srv.octeliumC.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_Value{
					Value: utilrand.GetRandomString(12),
				},
			},
		})
		assert.Nil(t, err)
		gitProvider, err := srv.octeliumC.CordiumC().CreateGitProvider(ctx, &cordiumv1.GitProvider{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec: &cordiumv1.GitProvider_Spec{
				Type: &cordiumv1.GitProvider_Spec_Github_{
					Github: &cordiumv1.GitProvider_Spec_Github{
						ClientID: utilrand.GetRandomString(8),
						ClientSecret: &cordiumv1.GitProvider_Spec_Github_ClientSecret{
							Type: &cordiumv1.GitProvider_Spec_Github_ClientSecret_FromSecret{
								FromSecret: sec.Metadata.Name,
							},
						},
					},
				},
			},
			Status: &cordiumv1.GitProvider_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
		})
		assert.Nil(t, err)

		project, err := srv.octeliumC.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec: &cordiumv1.Template_Spec{},
			Status: &cordiumv1.Template_Status{
				SpaceRef:       umetav1.GetObjectReference(org),
				GitProviderRef: umetav1.GetObjectReference(gitProvider),
			},
		})
		assert.Nil(t, err)

		ws, err := srv.octeliumC.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				State:       cordiumv1.Workspace_Status_STOPPED,
				SpaceRef:    umetav1.GetObjectReference(org),
				TemplateRef: umetav1.GetObjectReference(project),
				UserRef:     umetav1.GetObjectReference(usr.Usr),
			},
		})
		assert.Nil(t, err)

		req := httptest.NewRequest("POST",
			fmt.Sprintf("http://localhost/auth/v1/begin/%s", ws.Metadata.Uid), nil)
		req.Header.Set("X-Octelium-Session-Uid", usr.Session.Metadata.Uid)
		w := httptest.NewRecorder()

		req = req.WithContext(context.WithValue(req.Context(), middlewares.CtxRequestContext, &middlewares.RequestContext{
			Session: usr.Session,
		}))

		srv.handleAuthGitProviderBegin(w, req)

		resp := w.Result()
		assert.Equal(t, resp.StatusCode, http.StatusOK)

		bb, err := io.ReadAll(resp.Body)
		assert.Nil(t, err)
		resp.Body.Close()
		var postAuthResp handleAuthGitProviderBeginResponse
		err = json.Unmarshal(bb, &postAuthResp)
		assert.Nil(t, err)
	}

}

type tstSrvHTTP struct {
	port        int
	srv         *http.Server
	isWS        bool
	bearerToken string
	accessToken string
}

func newSrvHTTP(t *testing.T, port int, accessToken string) *tstSrvHTTP {
	return &tstSrvHTTP{
		port:        port,
		accessToken: accessToken,
	}
}

func (s *tstSrvHTTP) run(t *testing.T) {
	addr := fmt.Sprintf("localhost:%d", s.port)
	var err error

	handler := http.AllowQuerySemicolons(s)

	s.srv = &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	var lis net.Listener

	lis, err = net.Listen("tcp", addr)
	assert.Nil(t, err)

	go s.srv.Serve(lis)
}

func (s *tstSrvHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type oauthAccessTokenResponse struct {
		AccessToken  string `json:"access_token,omitempty"`
		RefreshToken string `json:"refresh_token,omitempty"`
		TokenType    string `json:"token_type,omitempty"`
		ExpiresIn    int    `json:"expires_in,omitempty"`
		Scope        string `json:"scope,omitempty"`
	}

	w.Header().Set("Content-Type", "application/json")
	resp := &oauthAccessTokenResponse{
		AccessToken:  s.accessToken,
		RefreshToken: utilrand.GetRandomString(18),
		TokenType:    "Bearer",
		ExpiresIn:    int(3600),
	}
	respBytes, _ := json.Marshal(resp)
	w.Write(respBytes)
}

func (s *tstSrvHTTP) close() {
	s.srv.Close()
	time.Sleep(1 * time.Second)
}

func TestHandleAuthGitProviderCallback(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	accessToken := utilrand.GetRandomStringCanonical(32)
	tstSrv := newSrvHTTP(t, utilrand.GetRandomRangeMath(10000, 20000), accessToken)
	tstSrv.run(t)
	defer tstSrv.close()
	time.Sleep(3 * time.Second)

	svc, err := adminSrv.CreateService(ctx, tests.GenService("default"))
	assert.Nil(t, err)

	os.Setenv("OCTELIUM_SVC_UID", svc.Metadata.Uid)

	usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
	assert.Nil(t, err)

	srv, err := newServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err, "%+v", err)

	{
		req := httptest.NewRequest("POST",
			fmt.Sprintf("http://localhost/auth/v1/begin/%s", vutils.UUIDv4()), nil)
		req.Header.Set("X-Octelium-Session-Uid", usr.Session.Metadata.Uid)
		w := httptest.NewRecorder()

		req = req.WithContext(context.WithValue(req.Context(), middlewares.CtxRequestContext, &middlewares.RequestContext{
			Session: usr.Session,
		}))

		srv.handleAuthGitProviderBegin(w, req)

		resp := w.Result()
		assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
	}

	{

		org, err := srv.octeliumC.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		_, err = srv.octeliumC.CordiumC().CreateMembership(ctx, &cordiumv1.Membership{
			Metadata: &metav1.Metadata{
				Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(org), umetav1.GetObjectReference(usr.Usr)),
			},
			Spec: &cordiumv1.Membership_Spec{
				Role: cordiumv1.Membership_Spec_ADMIN,
			},
			Status: &cordiumv1.Membership_Status{
				SpaceRef: umetav1.GetObjectReference(org),
				UserRef:  umetav1.GetObjectReference(usr.Usr),
			},
		})
		assert.Nil(t, err)

		sec, err := srv.octeliumC.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec: &cordiumv1.Secret_Spec{},
			Status: &cordiumv1.Secret_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
			Data: &cordiumv1.Secret_Data{
				Type: &cordiumv1.Secret_Data_Value{
					Value: utilrand.GetRandomString(12),
				},
			},
		})
		assert.Nil(t, err)
		gitProvider, err := srv.octeliumC.CordiumC().CreateGitProvider(ctx, &cordiumv1.GitProvider{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec: &cordiumv1.GitProvider_Spec{
				Type: &cordiumv1.GitProvider_Spec_Oauth2{
					Oauth2: &cordiumv1.GitProvider_Spec_OAuth2{
						ClientID: utilrand.GetRandomString(8),
						ClientSecret: &cordiumv1.GitProvider_Spec_OAuth2_ClientSecret{
							Type: &cordiumv1.GitProvider_Spec_OAuth2_ClientSecret_FromSecret{
								FromSecret: sec.Metadata.Name,
							},
						},
						Scopes:   []string{"scope1"},
						TokenURL: fmt.Sprintf("http://localhost:%d/oauth2/token", tstSrv.port),
					},
				},
			},
			Status: &cordiumv1.GitProvider_Status{
				SpaceRef: umetav1.GetObjectReference(org),
			},
		})
		assert.Nil(t, err)

		project, err := srv.octeliumC.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), org.Metadata.Name),
			},
			Spec: &cordiumv1.Template_Spec{},
			Status: &cordiumv1.Template_Status{
				SpaceRef:       umetav1.GetObjectReference(org),
				GitProviderRef: umetav1.GetObjectReference(gitProvider),
			},
		})
		assert.Nil(t, err)

		ws, err := srv.octeliumC.CordiumC().CreateWorkspace(ctx, &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				State:       cordiumv1.Workspace_Status_STOPPED,
				SpaceRef:    umetav1.GetObjectReference(org),
				TemplateRef: umetav1.GetObjectReference(project),
				UserRef:     umetav1.GetObjectReference(usr.Usr),
			},
		})
		assert.Nil(t, err)

		var state string
		{
			req := httptest.NewRequest("POST",
				fmt.Sprintf("http://localhost/auth/v1/begin/%s",
					ws.Metadata.Uid), nil)
			req.Header.Set("X-Octelium-Session-Uid", usr.Session.Metadata.Uid)
			w := httptest.NewRecorder()

			req = req.WithContext(context.WithValue(req.Context(), middlewares.CtxRequestContext, &middlewares.RequestContext{
				Session: usr.Session,
			}))

			srv.handleAuthGitProviderBegin(w, req)

			resp := w.Result()
			assert.Equal(t, resp.StatusCode, http.StatusOK)

			bb, err := io.ReadAll(resp.Body)
			assert.Nil(t, err)
			resp.Body.Close()
			var postAuthResp handleAuthGitProviderBeginResponse
			err = json.Unmarshal(bb, &postAuthResp)
			assert.Nil(t, err)

			u, err := url.Parse(postAuthResp.LoginURL)
			assert.Nil(t, err)
			state = u.Query().Get("state")
		}

		zap.L().Debug("State", zap.String("state", state))

		{

			u, _ := url.Parse("http://localhost/auth/v1/callback")

			q := u.Query()
			q.Add("state", state)
			q.Add("code", utilrand.GetRandomString(8))

			u.RawQuery = q.Encode()

			req := httptest.NewRequest("GET",
				u.String(), nil)
			req.Header.Set("X-Octelium-Session-Uid", usr.Session.Metadata.Uid)
			w := httptest.NewRecorder()

			req = req.WithContext(context.WithValue(req.Context(), middlewares.CtxRequestContext, &middlewares.RequestContext{
				Session: usr.Session,
			}))

			srv.handleAuthGitProviderCallback(w, req)

			resp := w.Result()
			assert.Equal(t, resp.StatusCode, http.StatusSeeOther)

			usrSecret, err := srv.octeliumC.CordiumC().GetUserSecret(ctx, &rmetav1.GetOptions{
				Name: fmt.Sprintf("git-provider-tokens.%s", usr.Usr.Metadata.Name),
			})
			assert.Nil(t, err, "%+v", err)

			attrs := usrSecret.Data.GetAttrs().AsMap()
			zap.L().Debug("attrMap", zap.Any("attr", attrs))
			gitProviderInfo := &ccordiumv1.GitProviderInfo{}
			err = pbutils.UnmarshalFromMap(attrs[ws.Metadata.Uid].(map[string]any), gitProviderInfo)
			assert.Nil(t, err)

			assert.Equal(t, accessToken, gitProviderInfo.AccessToken)
		}

	}

}
