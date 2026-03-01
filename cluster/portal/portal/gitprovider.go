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

package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/go-resty/resty/v2"
	"github.com/google/go-github/v33/github"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type handleAuthGitProviderBeginResponse struct {
	LoginURL string `json:"loginURL"`
}

func (s *Server) handleAuthGitProviderBegin(w http.ResponseWriter, r *http.Request) {

	zap.S().Debugf("Starting handleAuthGitProviderBegin")
	ctx := r.Context()
	reqCtx := middlewares.GetCtxRequestContext(ctx)

	if !rgxGitProviderBegin.MatchString(r.URL.Path) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	wsUID := func() string {
		match := rgxGitProviderBegin.FindStringSubmatch(r.URL.Path)

		var ret string
		for i, name := range rgxGitProviderBegin.SubexpNames() {
			switch name {
			case "ws":
				ret = match[i]
			}
		}
		return ret
	}()

	if !govalidator.IsUUIDv4(wsUID) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid: wsUID,
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if ws.Status.UserRef == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if ws.Status.UserRef.Uid != reqCtx.Session.Status.UserRef.Uid {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if ws.Status.State != cordiumv1.Workspace_Status_STOPPED {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if ws.Status.TemplateRef == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	org, err := s.octeliumC.CordiumC().GetSpace(ctx, &rmetav1.GetOptions{
		Uid: ws.Status.SpaceRef.Uid,
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	/*
		if org.Status.Type != cordiumv1.Space_Status_ORGANIZATION {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	*/

	project, err := s.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
		Uid: ws.Status.TemplateRef.Uid,
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if project.Status.GitProviderRef == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mem, err := s.octeliumC.CordiumC().GetMembership(ctx, &rmetav1.GetOptions{
		Name: workspacecommon.GetMembershipName(umetav1.GetObjectReference(org), reqCtx.Session.Status.UserRef),
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	gitProvider, err := s.octeliumC.CordiumC().GetGitProvider(ctx, &rmetav1.GetOptions{
		Uid: project.Status.GitProviderRef.Uid,
	})
	if err != nil {
		if grpcerr.IsNotFound(err) {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if mem.Status.GitProviderStateMap == nil {
		mem.Status.GitProviderStateMap = make(map[string]*cordiumv1.Membership_Status_GitProviderState)
	}

	stateID := utilrand.GetRandomStringCanonical(24)

	state := fmt.Sprintf("%s.%s", ws.Metadata.Uid, stateID)

	if len(mem.Status.GitProviderStateMap) >= 1000 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	mem.Status.GitProviderStateMap[ws.Metadata.Uid] = &cordiumv1.Membership_Status_GitProviderState{
		CreatedAt:      pbutils.Now(),
		GitProviderRef: umetav1.GetObjectReference(gitProvider),
		WorkspaceRef:   umetav1.GetObjectReference(ws),
		StateID:        stateID,
	}

	// clean up states older than 24 hours
	for state, info := range mem.Status.GitProviderStateMap {
		if info.CreatedAt.AsTime().Add(24 * time.Hour).Before(time.Now()) {
			delete(mem.Status.GitProviderStateMap, state)
		}
	}

	zap.L().Debug("Created gitProviderstateMap info",
		zap.String("state", state),
		zap.Any("info", mem.Status.GitProviderStateMap[state]))

	_, err = s.octeliumC.CordiumC().UpdateMembership(ctx, mem)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	oauth2Cfg, err := s.oauth2Config(ctx, gitProvider)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	loginURL := oauth2Cfg.AuthCodeURL(state)

	zap.L().Debug("Successfully create auth code URL", zap.String("url", loginURL))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(handleAuthGitProviderBeginResponse{
		LoginURL: loginURL,
	})

}

func (s *Server) handleAuthGitProviderCallback(w http.ResponseWriter, r *http.Request) {
	u, _ := url.Parse(s.rootURL)
	q := r.URL.Query()
	if errType := q.Get("error"); errType != "" {
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	state := r.FormValue("state")
	if state == "" {
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	if err := s.doHandleAuthGitProviderCallback(r.Context(), state, q.Get("code")); err != nil {
		zap.L().Debug("Auth err msg", zap.Error(err))
		q := u.Query()
		q.Add("error", "Unsuccessful authentication. Please try again")
		u.RawQuery = q.Encode()
	} else {
		stateArgs := strings.Split(state, ".")
		if len(stateArgs) == 2 {
			q := u.Query()
			q.Add("redirect", fmt.Sprintf("/workspaces/uid/%s", stateArgs[0]))
			u.RawQuery = q.Encode()
		}
	}

	http.Redirect(w, r, u.String(), http.StatusSeeOther)

}

func (s *Server) doHandleAuthGitProviderCallback(ctx context.Context, state, code string) error {
	reqCtx := middlewares.GetCtxRequestContext(ctx)

	zap.S().Debugf("Starting doHandleAuthGitProviderCallback")

	zap.L().Debug("Found state", zap.String("state", state))

	stateArgs := strings.Split(state, ".")
	if len(stateArgs) != 2 {
		return errors.Errorf("Invalid state format")
	}

	wsUID := stateArgs[0]
	stateID := stateArgs[1]

	if !govalidator.IsUUIDv4(wsUID) {
		return errors.Errorf("Invalid state ws UID")
	}

	ws, err := s.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid: wsUID,
	})
	if err != nil {
		return err
	}

	if ws.Status.UserRef == nil {
		return errors.Errorf("Workspace must have userRef")
	}

	if ws.Status.UserRef.Uid != reqCtx.Session.Status.UserRef.Uid {
		return errors.Errorf("Workspace is not owned by the User")
	}

	mem, err := s.octeliumC.CordiumC().GetMembership(ctx, &rmetav1.GetOptions{
		Name: workspacecommon.GetMembershipName(ws.Status.SpaceRef, reqCtx.Session.Status.UserRef),
	})
	if err != nil {
		return err
	}

	if mem.Status.GitProviderStateMap == nil {
		return errors.Errorf("Nil gitProviderStateMap")
	}

	providerState, ok := mem.Status.GitProviderStateMap[ws.Metadata.Uid]
	if !ok {
		return errors.Errorf("No providerState found with the state: %s", ws.Metadata.Uid)
	}

	if !providerState.CreatedAt.IsValid() || providerState.GitProviderRef == nil {
		return errors.Errorf("No providerState gitProviderRef or createdAt")
	}

	if providerState.StateID != stateID {
		return errors.Errorf("Invalid state ID")
	}

	gitProvider, err := s.octeliumC.CordiumC().GetGitProvider(ctx, &rmetav1.GetOptions{
		Uid: providerState.GitProviderRef.Uid,
	})
	if err != nil {
		return err
	}

	if providerState.CreatedAt.AsTime().Add(10 * time.Minute).Before(time.Now()) {
		return errors.Errorf("providerState is older than 10 minutes")
	}

	oauth2Config, err := s.oauth2Config(ctx, gitProvider)
	if err != nil {
		return err
	}

	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		return err
	}

	if !token.Valid() {
		return errors.Errorf("Invalid oauth2 token")
	}

	usrSecret, err := s.octeliumC.CordiumC().GetUserSecret(ctx, &rmetav1.GetOptions{
		Name: fmt.Sprintf("git-provider-tokens.%s", reqCtx.Session.Status.UserRef.Name),
	})
	if err != nil {
		if !grpcerr.IsNotFound(err) {
			return err
		}

		zap.L().Debug("Creating a new git-provider-tokens UserSecret",
			zap.Any("userRef", reqCtx.Session.Status.UserRef))

		usrSecret, err = s.octeliumC.CordiumC().CreateUserSecret(ctx, &cordiumv1.UserSecret{
			Metadata: &metav1.Metadata{
				Name:           fmt.Sprintf("git-provider-tokens.%s", reqCtx.Session.Status.UserRef.Name),
				IsSystem:       true,
				IsUserHidden:   true,
				IsSystemHidden: true,
			},
			Spec: &cordiumv1.UserSecret_Spec{},
			Status: &cordiumv1.UserSecret_Status{
				UserRef: reqCtx.Session.Status.UserRef,
			},
			Data: &cordiumv1.UserSecret_Data{
				Type: &cordiumv1.UserSecret_Data_Attrs{
					Attrs: pbutils.MapToStructMust(map[string]any{}),
				},
			},
		})
		if err != nil {
			return err
		}
	}

	dataMap := usrSecret.Data.GetAttrs().AsMap()

	{
		// clean up gitProviderInfo older than 3 months
		for k, v := range dataMap {
			if v == nil {
				continue
			}
			vMap, ok := v.(map[string]any)
			if !ok {
				continue
			}
			createdAtAny, ok := vMap["createdAt"]
			if !ok {
				continue
			}
			createdAt, ok := createdAtAny.(string)
			if !ok {
				continue
			}
			if createdAt == "" {
				continue
			}

			if vutils.MustParseTime(createdAt).Add(24 * time.Hour * 30 * 3).Before(time.Now()) {
				delete(dataMap, k)
			}
		}
	}

	gitProviderInfo, err := s.getGitProviderInfo(ctx, oauth2Config, token, gitProvider)
	if err != nil {
		return err
	}

	gitProviderInfoMap, err := pbutils.ConvertToMap(gitProviderInfo)
	if err != nil {
		return err
	}
	dataMap[ws.Metadata.Uid] = gitProviderInfoMap

	usrSecret.Data.Type = &cordiumv1.UserSecret_Data_Attrs{
		Attrs: pbutils.MapToStructMust(dataMap),
	}

	_, err = s.octeliumC.CordiumC().UpdateUserSecret(ctx, usrSecret)
	if err != nil {
		return err
	}

	zap.S().Debugf("successfully done doHandleAuthGitProviderCallback")

	return nil
}

func (s *Server) oauth2Config(ctx context.Context, gitProvider *cordiumv1.GitProvider) (*oauth2.Config, error) {
	var authURL string
	var tokenURL string
	var scopes []string
	var clientID string
	var clientSecret string

	zap.L().Debug("Building oauth2Config", zap.Any("gitProvider", gitProvider))

	switch gitProvider.Spec.Type.(type) {
	case *cordiumv1.GitProvider_Spec_Github_:
		clientID = gitProvider.Spec.GetGithub().ClientID
		if gitProvider.Spec.GetGithub().ClientSecret == nil || gitProvider.Spec.GetGithub().ClientSecret.GetFromSecret() == "" {
			return nil, errors.Errorf("No github GitProvider clientSecret")
		}
		sec, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
			Name: gitProvider.Spec.GetGithub().ClientSecret.GetFromSecret(),
		})
		if err != nil {
			return nil, err
		}
		clientSecret = ucordiumv1.ToSecret(sec).GetValueStr()

		authURL = "https://github.com/login/oauth/authorize"
		tokenURL = "https://github.com/login/oauth/access_token"

		if len(gitProvider.Spec.GetGithub().Scopes) == 0 {
			scopes = []string{
				"read:user", "user:email", "repo",
			}
		} else {
			scopes = gitProvider.Spec.GetGithub().Scopes
		}

	case *cordiumv1.GitProvider_Spec_Gitlab_:
		clientID = gitProvider.Spec.GetGitlab().ClientID
		if gitProvider.Spec.GetGitlab().ClientSecret == nil || gitProvider.Spec.GetGitlab().ClientSecret.GetFromSecret() == "" {
			return nil, errors.Errorf("No gitlab GitProvider clientSecret")
		}
		sec, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
			Name: gitProvider.Spec.GetGitlab().ClientSecret.GetFromSecret(),
		})
		if err != nil {
			return nil, err
		}
		clientSecret = ucordiumv1.ToSecret(sec).GetValueStr()
		authURL = "https://gitlab.com/oauth/authorize"
		tokenURL = "https://gitlab.com/oauth/token"

		if len(gitProvider.Spec.GetGitlab().Scopes) == 0 {
			scopes = []string{
				"read_user", "write_repository",
			}
		} else {
			scopes = gitProvider.Spec.GetGitlab().Scopes
		}

	case *cordiumv1.GitProvider_Spec_Oauth2:
		clientID = gitProvider.Spec.GetOauth2().ClientID
		sec, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
			Name: gitProvider.Spec.GetOauth2().ClientSecret.GetFromSecret(),
		})
		if err != nil {
			return nil, err
		}
		clientSecret = ucordiumv1.ToSecret(sec).GetValueStr()
		authURL = gitProvider.Spec.GetOauth2().AuthURL
		tokenURL = gitProvider.Spec.GetOauth2().TokenURL
		scopes = gitProvider.Spec.GetOauth2().Scopes
	default:
		return nil, errors.Errorf("Invalid GitProvider type")
	}

	zap.L().Debug("Successfully built oauth2Config", zap.Any("gitProvider", gitProvider))

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
		Scopes:      scopes,
		RedirectURL: fmt.Sprintf("%s/auth/v1/callback", s.rootURL),
	}, nil
}

func (s *Server) getGitProviderInfo(ctx context.Context, oauth2Config *oauth2.Config, tkn *oauth2.Token, gitProvider *cordiumv1.GitProvider) (*ccordiumv1.GitProviderInfo, error) {
	ret := &ccordiumv1.GitProviderInfo{
		CreatedAt:      pbutils.Now(),
		AccessToken:    tkn.AccessToken,
		RefreshToken:   tkn.RefreshToken,
		GitProviderRef: umetav1.GetObjectReference(gitProvider),
	}

	if !tkn.Expiry.IsZero() {
		ret.ExpiresAt = pbutils.Timestamp(tkn.Expiry)
	}

	zap.L().Debug("getting gitProviderInfo")

	switch gitProvider.Spec.Type.(type) {
	case *cordiumv1.GitProvider_Spec_Github_:
		client := oauth2Config.Client(ctx, tkn)

		githubClient := github.NewClient(client)

		user, _, err := githubClient.Users.Get(ctx, "")
		if err != nil {
			return nil, errors.Errorf("Could not get user")
		}

		if user.Email != nil {
			ret.Email = *user.Email
		}

		if user.Login != nil {
			ret.Username = *user.Login
		}

	case *cordiumv1.GitProvider_Spec_Gitlab_:
		type gitlabUser struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Username string `json:"username"`
			Email    string `json:"email"`
		}
		client := oauth2Config.Client(ctx, tkn)
		resp, err := resty.NewWithClient(client).R().SetResult(gitlabUser{}).Get("https://gitlab.com/api/v4/user")
		if err != nil {
			return nil, err
		}

		if !resp.IsSuccess() {
			return nil, errors.Errorf("Could not get Gitlab user information")
		}

		res := resp.Result().(*gitlabUser)

		ret.Username = res.Username
		ret.Email = res.Email
	}

	zap.L().Debug("Successfully built gitProviderInfo", zap.Any("gitProviderInfo", ret))

	return ret, nil
}
