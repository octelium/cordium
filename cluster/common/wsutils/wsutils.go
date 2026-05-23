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

package wsutils

import (
	"context"
	"os"
	"path"
	"regexp"

	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

func GenWorkspaceName() string {
	return utilrand.GetRandomStringCanonical(4)
}

type LoadWorkspaceFileRequest struct {
	Parent         *cordiumv1.Workspace
	BaseDir        string
	Space          *cordiumv1.Space
	SecretList     *cordiumv1.SecretList
	UserSecretList *cordiumv1.UserSecretList
}

func LoadWorkspaceFile(ctx context.Context, req *LoadWorkspaceFileRequest) (*cordiumv1.Workspace, error) {

	if req == nil || req.Parent == nil || req.Space == nil || req.BaseDir == "" {
		return nil, errors.Errorf("Cannot load Workspace. Invalid request")
	}

	paths := []string{
		".cordium/workspace.yaml",
		".cordium/workspace.yml",
		".cordium.yaml",
		".cordium.yml",
	}

	for _, pth := range paths {
		fPath := path.Join(req.BaseDir, pth)
		if vutils.FSPathExists(fPath) {
			ws, err := doLoadWorkspaceFile(ctx, fPath)
			if err != nil {
				return nil, err
			}

			if ws.Spec == nil {
				ws.Spec = &cordiumv1.Workspace_Spec{}

			}

			ws.Metadata = req.Parent.Metadata
			ws.Status = req.Parent.Status

			if err := ValidateWorkspace(ctx, &ValidateWorkspaceReq{
				Workspace:      ws,
				Space:          req.Space,
				SecretList:     req.SecretList,
				UserSecretList: req.UserSecretList,
			}); err != nil {
				return nil, err
			}

			return ws, nil
		}
	}

	return nil, errors.Errorf("Could not find Workspace yaml file")

}

func doLoadWorkspaceFile(ctx context.Context, fPath string) (*cordiumv1.Workspace, error) {

	content, err := os.ReadFile(fPath)
	if err != nil {
		return nil, err
	}

	ws := &cordiumv1.Workspace{}
	if err := pbutils.UnmarshalYAML(content, ws); err != nil {
		return nil, err
	}

	return ws, nil

}

func isValidURL(arg string) bool {
	if !govalidator.IsURL(arg) {
		return false
	}
	if len(arg) > 256 {
		return false
	}
	return true
}

func isInList(lst []string, arg string) bool {
	for _, itm := range lst {
		if itm == arg {
			return true
		}
	}
	return false
}

/*
func PortalServiceName() string {
	return "cordium"
}

func SSHServiceName() string {
	return "cordium-ssh"
}
*/

var rgxTerminalID = regexp.MustCompile(
	`^[a-z][a-z0-9]{2,6}-[a-z0-9]{4}$`)

func CheckTerminalID(arg string) error {
	if !rgxTerminalID.MatchString(arg) {
		return grpcutils.InvalidArg("Invalid Terminal ID")
	}

	return nil
}

type MergeSpecReq struct {
	Workspace      *cordiumv1.Workspace
	Template       *cordiumv1.Template
	ChildWorkspace *cordiumv1.Workspace
}

func mergeSpec(req *MergeSpecReq) (*cordiumv1.Workspace_Spec, error) {
	var specTemplate *cordiumv1.Workspace_Spec
	if req.Workspace == nil {
		return nil, errors.Errorf("Cannot Merge a nil Workspace")
	}

	if req.Template != nil && req.Template.Spec != nil {
		spec := req.Template.Spec

		specTemplate = &cordiumv1.Workspace_Spec{
			Image:                  spec.Image,
			Runtime:                spec.Runtime,
			Repository:             spec.Repository,
			AdditionalRepositories: spec.AdditionalRepositories,
		}
	}

	specWorkspace := proto.Clone(req.Workspace.Spec).(*cordiumv1.Workspace_Spec)
	if req.ChildWorkspace != nil && req.ChildWorkspace.Spec != nil {
		proto.Merge(specWorkspace, req.ChildWorkspace.Spec)
	}

	if specTemplate != nil {
		ret := proto.Clone(specTemplate).(*cordiumv1.Workspace_Spec)
		proto.Merge(ret, specWorkspace)
		return ret, nil
	}

	return specWorkspace, nil
}

func MergeSpec(req *MergeSpecReq) (*cordiumv1.Workspace_Spec, error) {
	merged, err := mergeSpec(req)
	if err != nil {
		return nil, err
	}

	vars := resolveVars(
		req.Template.GetSpec().GetVars(),
		req.Workspace.GetSpec().GetVars(),
		func() []*cordiumv1.Workspace_Spec_Var {
			if req.Workspace.Status.Run == nil || req.Workspace.Status.Run.Config == nil {
				return nil
			}
			if len(req.Workspace.Status.Run.Config.Vars) > 1000 {
				return nil
			}
			return req.Workspace.GetStatus().GetRun().GetConfig().GetVars()
		}(),
	)
	if len(vars) > 0 {
		renderSpec(merged, vars)
	}

	return merged, nil
}

func resolveVars(
	templateVars []*cordiumv1.Workspace_Spec_Var,
	workspaceVars []*cordiumv1.Workspace_Spec_Var,
	runConfigVars []*cordiumv1.Workspace_Spec_Var,
) map[string]string {
	if len(templateVars) == 0 && len(workspaceVars) == 0 && len(runConfigVars) == 0 {
		return nil
	}

	ret := make(map[string]string)

	for _, v := range templateVars {
		if v.Name != "" {
			ret[v.Name] = v.Value
		}
	}
	for _, v := range workspaceVars {
		if v.Name != "" {
			ret[v.Name] = v.Value
		}
	}
	for _, v := range runConfigVars {
		if v.Name != "" {
			ret[v.Name] = v.Value
		}
	}

	return ret
}

func renderSpec(spec *cordiumv1.Workspace_Spec, vars map[string]string) {
	if spec == nil || len(vars) == 0 {
		return
	}

	if spec.Image != nil {
		switch img := spec.Image.Type.(type) {
		case *cordiumv1.Workspace_Spec_Image_Registry_:
			if img.Registry != nil {
				img.Registry.Url = renderString(img.Registry.Url, vars)
			}
		case *cordiumv1.Workspace_Spec_Image_Dockerfile_:
			if img.Dockerfile != nil {
				switch dt := img.Dockerfile.Type.(type) {
				case *cordiumv1.Workspace_Spec_Image_Dockerfile_Url:
					dt.Url = renderString(dt.Url, vars)
				}
			}
		case *cordiumv1.Workspace_Spec_Image_Git_:
			if img.Git != nil {
				img.Git.Url = renderString(img.Git.Url, vars)
				img.Git.Checkout = renderString(img.Git.Checkout, vars)
				img.Git.Dockerfile = renderString(img.Git.Dockerfile, vars)
				img.Git.Context = renderString(img.Git.Context, vars)
			}
		}
	}

	renderRepository(spec.Repository, vars)

	for _, repo := range spec.AdditionalRepositories {
		if repo != nil {
			renderRepository(repo.Repository, vars)
		}
	}

	if spec.Runtime != nil {
		for _, env := range spec.Runtime.EnvVars {
			if v, ok := env.Type.(*cordiumv1.Workspace_Spec_Runtime_EnvVar_Value); ok {
				v.Value = renderString(v.Value, vars)
			}
		}

		for _, task := range spec.Runtime.Tasks {
			task.Run = renderString(task.Run, vars)
			task.WorkingDir = renderString(task.WorkingDir, vars)

			for _, env := range task.EnvVars {
				if env != nil {
					env.Value = renderString(env.Value, vars)
				}
			}
		}
	}
}

func renderRepository(repo *cordiumv1.Workspace_Spec_Repository, vars map[string]string) {
	if repo == nil {
		return
	}
	repo.Url = renderString(repo.Url, vars)
	if repo.CloneOptions != nil {
		repo.CloneOptions.Branch = renderString(repo.CloneOptions.Branch, vars)
		repo.CloneOptions.Checkout = renderString(repo.CloneOptions.Checkout, vars)
	}
}

func renderString(s string, vars map[string]string) string {
	if !strings.Contains(s, "${{") {
		return s
	}
	for name, value := range vars {
		s = strings.ReplaceAll(s, "${{ vars."+name+" }}", value)
		s = strings.ReplaceAll(s, "${{vars."+name+"}}", value)
	}

	for strings.Contains(s, "${{ vars.") || strings.Contains(s, "${{vars.") {
		start := strings.Index(s, "${{")
		end := strings.Index(s[start:], "}}")
		if end == -1 {
			break
		}
		s = s[:start] + s[start+end+2:]
	}

	return s
}

func Merge(req *MergeSpecReq) (*cordiumv1.Workspace, error) {
	spec, err := MergeSpec(req)
	if err != nil {
		return nil, err
	}

	ret := proto.Clone(req.Workspace).(*cordiumv1.Workspace)
	ret.Spec = spec
	return ret, nil
}
