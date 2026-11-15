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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type gitProviderTestTransport func(*http.Request) (*http.Response, error)

func (f gitProviderTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func gitProviderTestResponse(r *http.Request, status int, body string) *http.Response {
	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(body))
	ret := w.Result()
	ret.Request = r
	return ret
}

func TestGetGitProviderInfoGitHub(t *testing.T) {
	for _, tc := range []struct {
		name       string
		body       string
		status     int
		username   string
		email      string
		emptyToken bool
		err        string
	}{
		{name: "profile", body: `{"login":"octocat","email":"octocat@example.com"}`, username: "octocat", email: "octocat@example.com"},
		{name: "missing optional fields", body: `{}`},
		{name: "unauthorized", status: http.StatusUnauthorized, body: `{"message":"Bad credentials"}`, err: "Could not get user"},
		{name: "invalid response", body: `invalid`, err: "Could not get user"},
		{name: "empty token", emptyToken: true, err: "Could not create GitHub client"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requested := false
			transport := gitProviderTestTransport(func(r *http.Request) (*http.Response, error) {
				requested = true
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "https://api.github.com/user", r.URL.String())
				assert.Equal(t, "Bearer github-token", r.Header.Get("Authorization"))
				status := tc.status
				if status == 0 {
					status = http.StatusOK
				}
				return gitProviderTestResponse(r, status, tc.body), nil
			})
			ctx := context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{Transport: transport})
			token := &oauth2.Token{AccessToken: "github-token", RefreshToken: "refresh-token", Expiry: time.Now().Add(time.Hour)}
			if tc.emptyToken {
				token.AccessToken = ""
			}
			provider := &cordiumv1.GitProvider{
				Metadata: &metav1.Metadata{Name: "github", Uid: "provider-uid"},
				Spec:     &cordiumv1.GitProvider_Spec{Type: &cordiumv1.GitProvider_Spec_Github_{}},
			}
			s := &Server{}

			ret, err := s.getGitProviderInfo(ctx, &oauth2.Config{}, token, provider)
			assert.Equal(t, !tc.emptyToken, requested)
			if tc.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.err)
				assert.Nil(t, ret)
				return
			}
			require.Nil(t, err)
			require.NotNil(t, ret)
			assert.Equal(t, tc.username, ret.Username)
			assert.Equal(t, tc.email, ret.Email)
			assert.Equal(t, token.AccessToken, ret.AccessToken)
			assert.Equal(t, token.RefreshToken, ret.RefreshToken)
			assert.True(t, token.Expiry.Equal(ret.ExpiresAt.AsTime()))
			assert.True(t, pbutils.IsEqual(umetav1.GetObjectReference(provider), ret.GitProviderRef))
			assert.NotNil(t, ret.CreatedAt)
		})
	}
}

func TestGetGitProviderInfoGitHubRedirectCredentials(t *testing.T) {
	for _, tc := range []struct {
		name       string
		url        string
		authorized bool
	}{
		{name: "API origin", url: "https://api.github.com/redirected-user", authorized: true},
		{name: "upload origin", url: "https://uploads.github.com/redirected-user", authorized: true},
		{name: "other host", url: "https://example.net/redirected-user"},
		{name: "subdomain", url: "https://sub.api.github.com/redirected-user"},
		{name: "other port", url: "https://api.github.com:8443/redirected-user"},
		{name: "downgrade", url: "http://api.github.com/redirected-user"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			redirected := false
			transport := gitProviderTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.String() == tc.url {
					redirected = true
					if tc.authorized {
						assert.Equal(t, "Bearer github-token", r.Header.Get("Authorization"))
					} else {
						assert.Empty(t, r.Header.Get("Authorization"))
					}
					return gitProviderTestResponse(r, http.StatusOK, `{"login":"octocat"}`), nil
				}
				if r.URL.String() != "https://api.github.com/user" {
					return nil, fmt.Errorf("Unexpected GitHub request: %s", r.URL)
				}
				assert.Equal(t, "Bearer github-token", r.Header.Get("Authorization"))
				response := gitProviderTestResponse(r, http.StatusFound, "")
				response.Header.Set("Location", tc.url)
				return response, nil
			})
			ctx := context.WithValue(t.Context(), oauth2.HTTPClient, &http.Client{Transport: transport})
			provider := &cordiumv1.GitProvider{
				Metadata: &metav1.Metadata{Name: "github", Uid: "provider-uid"},
				Spec:     &cordiumv1.GitProvider_Spec{Type: &cordiumv1.GitProvider_Spec_Github_{}},
			}
			s := &Server{}

			ret, err := s.getGitProviderInfo(ctx, &oauth2.Config{}, &oauth2.Token{AccessToken: "github-token"}, provider)
			require.Nil(t, err)
			require.NotNil(t, ret)
			assert.True(t, redirected)
			assert.Equal(t, "octocat", ret.Username)
		})
	}
}

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

func TestHandleAuthGitProviderCallbackQuery(t *testing.T) {
	for _, tc := range []struct {
		name    string
		query   string
		authErr bool
	}{
		{name: "query state", query: "?state=query-state&code=code", authErr: true},
		{name: "body state"},
		{name: "provider error", query: "?error=access_denied&state=query-state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			assert.Nil(t, writer.WriteField("state", "body-state"))
			file, err := writer.CreateFormFile("upload", "file")
			assert.Nil(t, err)
			_, err = io.WriteString(file, "payload")
			assert.Nil(t, err)
			assert.Nil(t, writer.Close())
			bodyLen := body.Len()

			s := &Server{rootURL: "https://cordium.example.com"}
			req := httptest.NewRequest("GET", s.rootURL+"/auth/v1/callback"+tc.query, &body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			w := httptest.NewRecorder()

			s.ServeHTTP(w, req)

			resp := w.Result()
			defer resp.Body.Close()
			assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
			redirect, err := resp.Location()
			assert.Nil(t, err)
			assert.Equal(t, "https", redirect.Scheme)
			assert.Equal(t, "cordium.example.com", redirect.Host)
			assert.Equal(t, tc.authErr, redirect.Query().Has("error"))
			assert.Equal(t, bodyLen, body.Len())
			assert.Nil(t, req.MultipartForm)
		})
	}
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
