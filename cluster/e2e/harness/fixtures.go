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
	"context"
	"fmt"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"go.uber.org/zap"
)

func (h *H) UserConfig(t *testing.T) *cordiumv1.UserConfig {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	ret, err := h.CordiumC().GetUserConfig(ctx, &cordiumv1.GetUserConfigRequest{})
	if err != nil {
		t.Fatalf("Could not get the UserConfig: %+v", err)
	}

	return ret
}

func (h *H) UserName(t *testing.T) string {
	t.Helper()

	cfg := h.UserConfig(t)
	if cfg.Status == nil || cfg.Status.UserRef == nil {
		t.Fatalf("The UserConfig has no User reference")
	}

	return cfg.Status.UserRef.Name
}

func (h *H) SpaceName(t *testing.T, name string) string {
	t.Helper()

	return fmt.Sprintf("%s.%s", name, h.UserName(t))
}

func (h *H) CreateSpace(t *testing.T, spec *cordiumv1.Space_Spec) *cordiumv1.Space {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	if spec == nil {
		spec = &cordiumv1.Space_Spec{}
	}

	ret, err := h.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
		Metadata: &metav1.Metadata{
			Name: h.SpaceName(t, h.Name()),
		},
		Spec: spec,
	})
	if err != nil {
		t.Fatalf("Could not create the Space: %+v", err)
	}

	t.Cleanup(func() {
		h.deleteQuietly(t, "Space", ret.Metadata.Name, func(ctx context.Context) error {
			_, err := h.CordiumC().DeleteSpace(ctx,
				&metav1.DeleteOptions{Uid: ret.Metadata.Uid})
			return err
		})
	})

	zap.L().Debug("Created Space fixture", zap.String("name", ret.Metadata.Name))

	return ret
}

func (h *H) GetSpace(t *testing.T, name string) *cordiumv1.Space {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	ret, err := h.CordiumC().GetSpace(ctx, &metav1.GetOptions{Name: name})
	if err != nil {
		t.Fatalf("Could not get the Space %s: %+v", name, err)
	}

	return ret
}

func (h *H) CreateTemplate(t *testing.T,
	spc *cordiumv1.Space, spec *cordiumv1.Template_Spec) *cordiumv1.Template {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	if spec == nil {
		spec = &cordiumv1.Template_Spec{}
	}

	ret, err := h.CordiumC().CreateTemplate(ctx, &cordiumv1.Template{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s.%s", h.Name(), spc.Metadata.Name),
		},
		Spec: spec,
	})
	if err != nil {
		t.Fatalf("Could not create the Template: %+v", err)
	}

	t.Cleanup(func() {
		h.deleteQuietly(t, "Template", ret.Metadata.Name, func(ctx context.Context) error {
			_, err := h.CordiumC().DeleteTemplate(ctx,
				&metav1.DeleteOptions{Uid: ret.Metadata.Uid})
			return err
		})
	})

	zap.L().Debug("Created Template fixture", zap.String("name", ret.Metadata.Name))

	return ret
}

func (h *H) GetTemplate(t *testing.T, name string) *cordiumv1.Template {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	ret, err := h.CordiumC().GetTemplate(ctx, &metav1.GetOptions{Name: name})
	if err != nil {
		t.Fatalf("Could not get the Template %s: %+v", name, err)
	}

	return ret
}

func (h *H) CreateSecret(t *testing.T, spc *cordiumv1.Space, value string) *cordiumv1.Secret {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	ret, err := h.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s.%s", h.Name(), spc.Metadata.Name),
		},
		Spec: &cordiumv1.Secret_Spec{},
		Data: &cordiumv1.Secret_Data{
			Type: &cordiumv1.Secret_Data_Value{Value: value},
		},
	})
	if err != nil {
		t.Fatalf("Could not create the Secret: %+v", err)
	}

	t.Cleanup(func() {
		h.deleteQuietly(t, "Secret", ret.Metadata.Name, func(ctx context.Context) error {
			_, err := h.CordiumC().DeleteSecret(ctx,
				&metav1.DeleteOptions{Uid: ret.Metadata.Uid})
			return err
		})
	})

	zap.L().Debug("Created Secret fixture", zap.String("name", ret.Metadata.Name))

	return ret
}

func (h *H) CreateUserSecret(t *testing.T, value string) *cordiumv1.UserSecret {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	ret, err := h.CordiumC().CreateUserSecret(ctx, &cordiumv1.UserSecret{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s.%s", h.Name(), h.UserName(t)),
		},
		Spec: &cordiumv1.UserSecret_Spec{},
		Data: &cordiumv1.UserSecret_Data{
			Type: &cordiumv1.UserSecret_Data_Value{Value: value},
		},
	})
	if err != nil {
		t.Fatalf("Could not create the UserSecret: %+v", err)
	}

	t.Cleanup(func() {
		h.deleteQuietly(t, "UserSecret", ret.Metadata.Name, func(ctx context.Context) error {
			_, err := h.CordiumC().DeleteUserSecret(ctx,
				&metav1.DeleteOptions{Uid: ret.Metadata.Uid})
			return err
		})
	})

	zap.L().Debug("Created UserSecret fixture", zap.String("name", ret.Metadata.Name))

	return ret
}

func (h *H) CreateOrganizationSpace(t *testing.T, spec *cordiumv1.Space_Spec) *cordiumv1.Space {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	if spec == nil {
		spec = &cordiumv1.Space_Spec{}
	}

	ret, err := h.CordiumC().CreateSpace(ctx, &cordiumv1.Space{
		Metadata: &metav1.Metadata{
			Name: fmt.Sprintf("%s.cordium", h.Name()),
		},
		Spec: spec,
	})
	if err != nil {
		t.Fatalf("Could not create the organization Space: %+v", err)
	}

	t.Cleanup(func() {
		h.deleteQuietly(t, "Space", ret.Metadata.Name, func(ctx context.Context) error {
			_, err := h.CordiumC().DeleteSpace(ctx,
				&metav1.DeleteOptions{Uid: ret.Metadata.Uid})
			return err
		})
	})

	zap.L().Debug("Created organization Space fixture", zap.String("name", ret.Metadata.Name))

	return ret
}
