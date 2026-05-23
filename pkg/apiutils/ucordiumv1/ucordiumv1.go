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

package ucordiumv1

import (
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type Workspace struct {
	*cordiumv1.Workspace
}

func ToWorkspace(a *cordiumv1.Workspace) *Workspace {
	return &Workspace{
		Workspace: a,
	}
}

type SecretList struct {
	*cordiumv1.SecretList
}

func ToSecretList(a *cordiumv1.SecretList) *SecretList {
	return &SecretList{
		SecretList: a,
	}
}

type UserSecretList struct {
	*cordiumv1.UserSecretList
}

func ToUserSecretList(a *cordiumv1.UserSecretList) *UserSecretList {
	return &UserSecretList{
		UserSecretList: a,
	}
}

type Template struct {
	*cordiumv1.Template
}

func ToTemplate(a *cordiumv1.Template) *Template {
	return &Template{
		Template: a,
	}
}

type Secret struct {
	*cordiumv1.Secret
}

func ToSecret(a *cordiumv1.Secret) *Secret {
	return &Secret{
		Secret: a,
	}
}

type UserSecret struct {
	*cordiumv1.UserSecret
}

func ToUserSecret(a *cordiumv1.UserSecret) *UserSecret {
	return &UserSecret{
		UserSecret: a,
	}
}

const (
	KindWorkspace     = "Workspace"
	KindSecret        = "Secret"
	KindTemplate      = "Template"
	KindEnvironment   = "Environment"
	KindSpace         = "Space"
	KindMembership    = "Membership"
	KindGitProvider   = "GitProvider"
	KindUserSecret    = "UserSecret"
	KindUserConfig    = "UserConfig"
	KindClusterConfig = "ClusterConfig"
)

func NewObjectList(kind string) (umetav1.ObjectI, error) {

	switch kind {
	case KindWorkspace:
		return &cordiumv1.WorkspaceList{}, nil
	case KindSecret:
		return &cordiumv1.SecretList{}, nil
	case KindTemplate:
		return &cordiumv1.TemplateList{}, nil
	case KindSpace:
		return &cordiumv1.SpaceList{}, nil
	case KindMembership:
		return &cordiumv1.MembershipList{}, nil
	case KindGitProvider:
		return &cordiumv1.GitProviderList{}, nil
	case KindUserSecret:
		return &cordiumv1.UserSecretList{}, nil
	default:
		return nil, errors.Errorf("Invalid kind: %s", kind)
	}
}

func NewObjectListOptions(kind string) (proto.Message, error) {

	switch kind {
	case KindWorkspace:
		return &cordiumv1.ListWorkspaceOptions{}, nil
	case KindTemplate:
		return &cordiumv1.ListTemplateOptions{}, nil
	case KindSecret:
		return &cordiumv1.ListSecretOptions{}, nil
	/*
		case KindEnvironment:
			return &cordiumv1.ListEnvironmentOptions{}, nil
	*/
	case KindSpace:
		return &cordiumv1.ListSpaceOptions{}, nil
	case KindMembership:
		return &cordiumv1.ListMembershipOptions{}, nil
	case KindGitProvider:
		return &cordiumv1.ListGitProviderOptions{}, nil
	case KindUserSecret:
		return &cordiumv1.ListUserSecretOptions{}, nil
	default:
		return nil, errors.Errorf("Invalid kind: %s", kind)
	}
}

func NewObject(kind string) (umetav1.ResourceObjectI, error) {

	switch kind {
	case KindWorkspace:
		return &cordiumv1.Workspace{}, nil
	case KindTemplate:
		return &cordiumv1.Template{}, nil
	case KindSecret:
		return &cordiumv1.Secret{}, nil

		/*
			case KindEnvironment:
				return &cordiumv1.Environment{}, nil
		*/
	case KindSpace:
		return &cordiumv1.Space{}, nil
	case KindMembership:
		return &cordiumv1.Membership{}, nil
	case KindGitProvider:
		return &cordiumv1.GitProvider{}, nil
	case KindUserSecret:
		return &cordiumv1.UserSecret{}, nil
	case KindUserConfig:
		return &cordiumv1.UserConfig{}, nil
	case KindClusterConfig:
		return &cordiumv1.ClusterConfig{}, nil
	default:
		return nil, errors.Errorf("Invalid kind: %s", kind)
	}
}

type ResourceObjectRefG interface {
	*cordiumv1.Workspace | *cordiumv1.Secret | *cordiumv1.Template |
		*cordiumv1.Space | *cordiumv1.Membership | *cordiumv1.GitProvider | *cordiumv1.UserSecret | *cordiumv1.UserConfig
}

const API = "cordium"
const Version = "v1"
const APIVersion = "cordium/v1"

func (w *Workspace) IsPreRunning() bool {
	switch w.Status.State {
	case cordiumv1.Workspace_Status_INIT_REQUEST, cordiumv1.Workspace_Status_INITIALIZING,
		cordiumv1.Workspace_Status_BUILDING_IMAGE,
		cordiumv1.Workspace_Status_PULLING_IMAGE,
		cordiumv1.Workspace_Status_STARTING_RUNTIME,
		cordiumv1.Workspace_Status_PREPARING:
		return true
	default:
		return false
	}
}

func (w *Workspace) IsActive() bool {
	return w.IsPreRunning() || w.IsRunning() || w.IsStoppingOrStopped()
}

func (w *Workspace) IsInitializing() bool {
	switch w.Status.State {
	case cordiumv1.Workspace_Status_INIT_REQUEST, cordiumv1.Workspace_Status_INITIALIZING:
		return true
	default:
		return false
	}
}

func (w *Workspace) IsRunning() bool {
	switch w.Status.State {
	case cordiumv1.Workspace_Status_RUNNING:
		return true
	default:
		return false
	}
}

func (w *Workspace) IsPreparingOrRunning() bool {
	switch w.Status.State {
	case cordiumv1.Workspace_Status_RUNNING, cordiumv1.Workspace_Status_PREPARING:
		return true
	default:
		return false
	}
}

func (w *Workspace) IsStopped() bool {
	switch w.Status.State {
	case cordiumv1.Workspace_Status_STOPPED:
		return true
	default:
		return false
	}
}

func (w *Workspace) IsStopping() bool {
	switch w.Status.State {
	case cordiumv1.Workspace_Status_STOPPING_REQUEST,
		cordiumv1.Workspace_Status_STOPPING:
		return true
	default:
		return false
	}
}

func (w *Workspace) IsStoppingOrStopped() bool {
	return w.IsStopped() || w.IsStopping()
}

func (s *SecretList) GetByName(name string) (*cordiumv1.Secret, error) {
	for _, idp := range s.Items {
		if idp.Metadata.Name == name {
			return idp, nil
		}
	}
	return nil, errors.Errorf("No Secret exists with name: %s", name)
}

func (s *UserSecretList) GetByName(name string) (*cordiumv1.UserSecret, error) {
	for _, idp := range s.Items {
		if idp.Metadata.Name == name {
			return idp, nil
		}
	}
	return nil, errors.Errorf("No Secret exists with name: %s", name)
}

func (s *Secret) GetValueStr() string {
	if s.Data == nil {
		return ""
	}
	switch s.Data.Type.(type) {
	case *cordiumv1.Secret_Data_Value:
		return s.Data.GetValue()
	case *cordiumv1.Secret_Data_ValueBytes:
		return string(s.Data.GetValueBytes())
	default:
		return ""
	}
}

func (s *UserSecret) GetValueStr() string {
	if s.Data == nil {
		return ""
	}
	switch s.Data.Type.(type) {
	case *cordiumv1.UserSecret_Data_Value:
		return s.Data.GetValue()
	case *cordiumv1.UserSecret_Data_ValueBytes:
		return string(s.Data.GetValueBytes())
	default:
		return ""
	}
}

func (w *Workspace) GetApplicationByName(name string) *cordiumv1.Workspace_Spec_Application {

	for _, app := range w.Spec.Applications {
		if app.Name == name {
			return app
		}
	}

	return nil
}

func (w *Workspace) GetDefaultApplication() *cordiumv1.Workspace_Spec_Application {

	for _, app := range w.Spec.Applications {
		if app.IsDefault {
			return app
		}
	}

	return nil
}

func (w *Workspace) GetCurrentRun() *cordiumv1.Workspace_Status_Run {
	if w == nil {
		return nil
	}
	return w.Status.Run
}

func (t *Template) HasReadyBuild() bool {
	return t != nil && t.Status.BuildInfo != nil && t.Status.BuildInfo.CurrentReadyBuildID != ""
}
