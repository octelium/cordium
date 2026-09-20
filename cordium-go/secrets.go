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

package cordium

import (
	"context"
	"iter"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"google.golang.org/protobuf/types/known/structpb"
)

// SecretClient is the Space scoped Secret API. A Secret holds a sensitive value
// that the Workspace and Template specs reference by name (e.g. as the source
// of an environment variable, of a registry password or of a repository
// password). The Cluster resolves it at initialization time and never returns
// its content back.
type SecretClient struct {
	c *Client
}

// CreateString creates a Secret from a string. The name can be short (e.g.
// "db-password") or qualified with its Space (e.g. "db-password.my-project").
// The caller must be at least an ADMIN Member of the Space.
func (sc *SecretClient) CreateString(ctx context.Context, name, value string) (*cordiumv1.Secret, error) {
	return sc.create(ctx, name, &cordiumv1.Secret_Data{
		Type: &cordiumv1.Secret_Data_Value{Value: value},
	})
}

// CreateBytes creates a Secret from raw bytes.
func (sc *SecretClient) CreateBytes(ctx context.Context, name string, value []byte) (*cordiumv1.Secret, error) {
	return sc.create(ctx, name, &cordiumv1.Secret_Data{
		Type: &cordiumv1.Secret_Data_ValueBytes{ValueBytes: value},
	})
}

// CreateAttrs creates a Secret from a structured map of attributes, which is
// how a multi-field credential (e.g. a username and a password) is stored as
// one Secret.
func (sc *SecretClient) CreateAttrs(ctx context.Context, name string, attrs map[string]any) (*cordiumv1.Secret, error) {
	value, err := structpb.NewStruct(attrs)
	if err != nil {
		return nil, invalidArgumentf("could not encode the Secret attributes: %v", err)
	}
	return sc.create(ctx, name, &cordiumv1.Secret_Data{
		Type: &cordiumv1.Secret_Data_Attrs{Attrs: value},
	})
}

func (sc *SecretClient) create(ctx context.Context, name string, data *cordiumv1.Secret_Data) (*cordiumv1.Secret, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Secret name")
	}

	return sc.c.MainService().CreateSecret(ctx, &cordiumv1.Secret{
		Metadata: &metav1.Metadata{Name: name},
		Spec:     &cordiumv1.Secret_Spec{},
		Status:   &cordiumv1.Secret_Status{},
		Data:     data,
	})
}

// Get retrieves a Secret. Its content is never included in the response.
func (sc *SecretClient) Get(ctx context.Context, name string) (*cordiumv1.Secret, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Secret name")
	}
	return sc.c.MainService().GetSecret(ctx, getOptionsFor(name))
}

// Delete deletes a Secret.
func (sc *SecretClient) Delete(ctx context.Context, name string) error {
	if err := sc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty Secret name")
	}
	_, err := sc.c.MainService().DeleteSecret(ctx, deleteOptionsFor(name))
	return err
}

// SecretList is a single page of Secrets.
type SecretList struct {
	// Items is the page's Secrets. Their content is not included.
	Items []*cordiumv1.Secret
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the Secrets of a Space, which is chosen with
// [InSpace]. The Secrets' content is not included.
func (sc *SecretClient) List(ctx context.Context, opts ...ListOption) (*SecretList, error) {
	if err := sc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	list, err := sc.c.MainService().ListSecret(ctx, &cordiumv1.ListSecretOptions{
		Common:   cfg.common(),
		SpaceRef: cfg.spaceRef,
	})
	if err != nil {
		return nil, err
	}

	return &SecretList{
		Items: list.GetItems(),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}, nil
}

// All iterates over every Secret of a Space, fetching the pages as it goes.
func (sc *SecretClient) All(ctx context.Context, opts ...ListOption) iter.Seq2[*cordiumv1.Secret, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*cordiumv1.Secret, PageInfo, error) {
			list, err := sc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}

// UserSecretClient is the User scoped Secret API. A UserSecret belongs to its
// owner User rather than to a Space, and it is what the UserConfig environment
// variables and the dotfiles authentication read from.
type UserSecretClient struct {
	c *Client
}

// CreateString creates a UserSecret from a string.
func (uc *UserSecretClient) CreateString(ctx context.Context, name, value string) (*cordiumv1.UserSecret, error) {
	return uc.create(ctx, name, cordiumv1.UserSecret_Spec_DEFAULT, &cordiumv1.UserSecret_Data{
		Type: &cordiumv1.UserSecret_Data_Value{Value: value},
	})
}

// CreateBytes creates a UserSecret from raw bytes.
func (uc *UserSecretClient) CreateBytes(ctx context.Context, name string, value []byte) (*cordiumv1.UserSecret, error) {
	return uc.create(ctx, name, cordiumv1.UserSecret_Spec_DEFAULT, &cordiumv1.UserSecret_Data{
		Type: &cordiumv1.UserSecret_Data_ValueBytes{ValueBytes: value},
	})
}

// CreateAttrs creates a UserSecret from a structured map of attributes.
func (uc *UserSecretClient) CreateAttrs(ctx context.Context, name string, attrs map[string]any) (*cordiumv1.UserSecret, error) {
	value, err := structpb.NewStruct(attrs)
	if err != nil {
		return nil, invalidArgumentf("could not encode the UserSecret attributes: %v", err)
	}
	return uc.create(ctx, name, cordiumv1.UserSecret_Spec_DEFAULT, &cordiumv1.UserSecret_Data{
		Type: &cordiumv1.UserSecret_Data_Attrs{Attrs: value},
	})
}

// CreateSSHKey creates a UserSecret whose ECDSA key pair the Cluster generates.
// The private key stays inside the Cluster and is loaded into an SSH agent in
// every Workspace of the User, while the returned UserSecret carries the public
// key in its status.
func (uc *UserSecretClient) CreateSSHKey(ctx context.Context, name string) (*cordiumv1.UserSecret, error) {
	return uc.create(ctx, name, cordiumv1.UserSecret_Spec_SSH_KEY, nil)
}

func (uc *UserSecretClient) create(ctx context.Context, name string,
	typ cordiumv1.UserSecret_Spec_Type, data *cordiumv1.UserSecret_Data) (*cordiumv1.UserSecret, error) {

	if err := uc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty UserSecret name")
	}

	return uc.c.MainService().CreateUserSecret(ctx, &cordiumv1.UserSecret{
		Metadata: &metav1.Metadata{Name: name},
		Spec:     &cordiumv1.UserSecret_Spec{Type: typ},
		Status:   &cordiumv1.UserSecret_Status{},
		Data:     data,
	})
}

// UpdateString replaces the content of a UserSecret with a string.
func (uc *UserSecretClient) UpdateString(ctx context.Context, name, value string) (*cordiumv1.UserSecret, error) {
	return uc.update(ctx, name, &cordiumv1.UserSecret_Data{
		Type: &cordiumv1.UserSecret_Data_Value{Value: value},
	})
}

// UpdateBytes replaces the content of a UserSecret with raw bytes.
func (uc *UserSecretClient) UpdateBytes(ctx context.Context, name string, value []byte) (*cordiumv1.UserSecret, error) {
	return uc.update(ctx, name, &cordiumv1.UserSecret_Data{
		Type: &cordiumv1.UserSecret_Data_ValueBytes{ValueBytes: value},
	})
}

func (uc *UserSecretClient) update(ctx context.Context, name string,
	data *cordiumv1.UserSecret_Data) (*cordiumv1.UserSecret, error) {

	current, err := uc.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	current.Data = data

	return uc.c.MainService().UpdateUserSecret(ctx, current)
}

// Get retrieves a UserSecret. Its content is never included in the response.
func (uc *UserSecretClient) Get(ctx context.Context, name string) (*cordiumv1.UserSecret, error) {
	if err := uc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty UserSecret name")
	}
	return uc.c.MainService().GetUserSecret(ctx, getOptionsFor(name))
}

// Delete deletes a UserSecret.
func (uc *UserSecretClient) Delete(ctx context.Context, name string) error {
	if err := uc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty UserSecret name")
	}
	_, err := uc.c.MainService().DeleteUserSecret(ctx, deleteOptionsFor(name))
	return err
}

// UserSecretList is a single page of UserSecrets.
type UserSecretList struct {
	// Items is the page's UserSecrets. Their content is not included.
	Items []*cordiumv1.UserSecret
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the UserSecrets that the calling User owns.
func (uc *UserSecretClient) List(ctx context.Context, opts ...ListOption) (*UserSecretList, error) {
	if err := uc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	list, err := uc.c.MainService().ListUserSecret(ctx, &cordiumv1.ListUserSecretOptions{
		Common: cfg.common(),
	})
	if err != nil {
		return nil, err
	}

	return &UserSecretList{
		Items: list.GetItems(),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}, nil
}

// All iterates over every UserSecret of the calling User.
func (uc *UserSecretClient) All(ctx context.Context, opts ...ListOption) iter.Seq2[*cordiumv1.UserSecret, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*cordiumv1.UserSecret, PageInfo, error) {
			list, err := uc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}
