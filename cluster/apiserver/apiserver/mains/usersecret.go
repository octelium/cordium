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

package mains

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/octelium/cordium/cluster/apiserver/apiserver/commonw"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/common"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/serr"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/grpcerr"
	utils_cert "github.com/octelium/octelium/pkg/utils/cert"
	"golang.org/x/crypto/ssh"
)

const maxUserSecretPerUser = 1000

func (s *Server) CreateUserSecret(ctx context.Context, req *cordiumv1.UserSecret) (*cordiumv1.UserSecret, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.ValidateCommon(getFullNamResourceUserChildWithUserCtx(i, req),
		&apivalidation.ValidateCommonOpts{
			ValidateMetadataOpts: apivalidation.ValidateMetadataOpts{
				RequireName: true,
				ParentsMust: 1,
			},
			RequireData: true,
		}); err != nil {
		return nil, err
	}

	nameArgs := strings.Split(req.Metadata.Name, ".")
	if len(nameArgs) != 2 {
		return nil, serr.InvalidArg("Invalid name: %s", req.Metadata.Name)
	}
	if nameArgs[1] != i.User.Metadata.Name {
		return nil, serr.Unauthorized("Invalid User")
	}

	{
		secretList, err := s.octeliumC.CordiumC().ListUserSecret(ctx, urscsrv.FilterByUser(i.User))
		if err != nil {
			return nil, serr.InternalWithErr(err)
		}
		if len(secretList.Items) >= maxUserSecretPerUser {
			return nil, serr.Unauthorized("Maximum UserSecret limit reached")
		}
	}

	{
		_, err := s.octeliumC.CordiumC().GetUserSecret(ctx, &rmetav1.GetOptions{
			Name: req.Metadata.Name,
		})
		if err == nil {
			return nil, grpcutils.InvalidArg("This UserSecret name already exists")
		} else if !grpcerr.IsNotFound(err) {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	item := &cordiumv1.UserSecret{
		Metadata: common.MetadataFrom(req.Metadata),
		Spec:     req.Spec,
		Status: &cordiumv1.UserSecret_Status{
			UserRef: umetav1.GetObjectReference(i.User),
		},
		Data: req.Data,
	}

	if err := s.checkUserSecretData(ctx, item); err != nil {
		return nil, err
	}

	switch item.Spec.Type {
	case cordiumv1.UserSecret_Spec_SSH_KEY:
		if err := s.setSSHKeyUserSecret(item); err != nil {
			return nil, grpcutils.InternalWithErr(err)
		}
	}

	ret, err := s.octeliumC.CordiumC().CreateUserSecret(ctx, item)
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	ret.Data = nil

	return ret, nil
}

func (s *Server) checkUserSecretData(ctx context.Context, req *cordiumv1.UserSecret) error {

	switch req.Spec.Type {
	case cordiumv1.UserSecret_Spec_SSH_KEY:
		req.Data = &cordiumv1.UserSecret_Data{}
		return nil
	}

	if req.Data == nil {
		return serr.InvalidArg("Nil data")
	}

	maxSize := 1500

	switch req.Data.Type.(type) {
	case *cordiumv1.UserSecret_Data_Value:
		lenVal := len(req.Data.GetValue())
		if lenVal == 0 {
			return serr.InvalidArg("Empty value")
		}
		if lenVal > maxSize {
			return serr.InvalidArg("UserSecret data is too large")
		}

	case *cordiumv1.UserSecret_Data_ValueBytes:
		lenVal := len(req.Data.GetValueBytes())
		if lenVal == 0 {
			return serr.InvalidArg("Empty value")
		}
		if lenVal > maxSize {
			return serr.InvalidArg("UserSecret data is too large")
		}
	case *cordiumv1.UserSecret_Data_Attrs:
		out, err := json.Marshal(req.Data.GetAttrs().AsMap())
		if err != nil {
			return serr.InvalidArg("Could not parse attributes")
		}
		if len(out) > maxSize {
			return serr.InvalidArg("UserSecret data is too large")
		}
	default:
		return serr.InvalidArg("Invalid UserSecret data type")
	}

	return nil

}

func (s *Server) ListUserSecret(ctx context.Context, req *cordiumv1.ListUserSecretOptions) (*cordiumv1.UserSecretList, error) {

	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	secretList, err := s.octeliumC.CordiumC().ListUserSecret(ctx,
		urscsrv.GetUserPublicListOptions(req, urscsrv.FilterStatusUserUID(i.User.Metadata.Uid)))
	if err != nil {
		return nil, serr.InternalWithErr(err)
	}

	for _, sec := range secretList.Items {
		sec.Data = nil
	}

	return secretList, nil
}

func (s *Server) DeleteUserSecret(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckDeleteOptions(req, &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 1,
	}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetUserSecret(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if item.Status.UserRef.Uid != i.User.Metadata.Uid {
		return nil, serr.Unauthorized("Cannot delete this UserSecret")
	}

	if item.Metadata.IsSystem {
		return nil, serr.Unauthorized("Cannot delete this system UserSecret")
	}

	if _, err := s.octeliumC.CordiumC().DeleteUserSecret(ctx, &rmetav1.DeleteOptions{Uid: item.Metadata.Uid}); err != nil {
		return nil, serr.InternalWithErr(err)
	}

	return &metav1.OperationResult{}, nil
}

func (s *Server) GetUserSecret(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.UserSecret, error) {
	i, err := commonw.GetUserCtx(ctx)
	if err != nil {
		return nil, err
	}

	if err := apivalidation.CheckGetOptions(getFullNameGetOptionsUserChild(ctx, req), &apivalidation.CheckGetOptionsOpts{
		ParentsMust: 1,
	}); err != nil {
		return nil, err
	}

	item, err := s.octeliumC.CordiumC().GetUserSecret(ctx, &rmetav1.GetOptions{
		Uid:  req.Uid,
		Name: req.Name,
	})
	if err != nil {
		return nil, serr.K8sNotFoundOrInternalWithErr(err)
	}

	if item.Status.UserRef.Uid != i.User.Metadata.Uid {
		return nil, serr.Unauthorized("Cannot get this UserSecret")
	}

	item.Data = nil

	return item, nil
}

func (s *Server) setSSHKeyUserSecret(sec *cordiumv1.UserSecret) error {

	k, err := utils_cert.GenerateECDSA()
	if err != nil {
		return err
	}

	kPrivatePEM, err := k.GetPrivateKeyPEM()
	if err != nil {
		return err
	}

	privSigner, err := ssh.NewSignerFromKey(k.PrivateKey)
	if err != nil {
		return err
	}

	nameArgs := strings.Split(sec.Metadata.Name, ".")
	if len(nameArgs) != 2 {
		return serr.InvalidArg("Invalid name: %s", sec.Metadata.Name)
	}

	pKey := fmt.Sprintf("%s %s@cordium",
		strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(privSigner.PublicKey())), "\n"),
		nameArgs[0],
	)

	sec.Data = &cordiumv1.UserSecret_Data{
		Type: &cordiumv1.UserSecret_Data_ValueBytes{
			ValueBytes: []byte(kPrivatePEM),
		},
	}

	if sec.Status == nil {
		sec.Status = &cordiumv1.UserSecret_Status{}
	}

	sec.Status.Details = &cordiumv1.UserSecret_Status_SshKey{
		SshKey: &cordiumv1.UserSecret_Status_SSHKey{
			PublicKey: pKey,
		},
	}

	return nil
}
