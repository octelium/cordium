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

package wsutils

import (
	"context"
	"os"
	"path"
	"regexp"

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
		return nil, errors.Errorf("Cannot load Workspace. Invaldi request")
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
	Workspace *cordiumv1.Workspace
	Template  *cordiumv1.Template
	// Environment *cordiumv1.Environment

	ChildWorkspace *cordiumv1.Workspace
}

func MergeSpec(req *MergeSpecReq) (*cordiumv1.Workspace_Spec, error) {
	// var specEnvironment *cordiumv1.Workspace_Spec
	var specTemplate *cordiumv1.Workspace_Spec
	if req.Workspace == nil {
		return nil, errors.Errorf("Cannot Merge a nil Workspace")
	}

	/*
		if req.Environment != nil && req.Environment.Spec != nil {
			spec := req.Environment.Spec
			specEnvironment = &cordiumv1.Workspace_Spec{
				Image:                  spec.Image,
				Runtime:                spec.Runtime,
				Repository:             spec.Repository,
				AdditionalRepositories: spec.AdditionalRepositories,
			}
		}
	*/

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

func Merge(req *MergeSpecReq) (*cordiumv1.Workspace, error) {
	spec, err := MergeSpec(req)
	if err != nil {
		return nil, err
	}

	ret := proto.Clone(req.Workspace).(*cordiumv1.Workspace)
	ret.Spec = spec
	return ret, nil
}
