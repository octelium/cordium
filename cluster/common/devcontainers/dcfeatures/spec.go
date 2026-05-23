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

package dcfeatures

import (
	"encoding/json"

	"github.com/octelium/cordium/cluster/common/devcontainers/dcvars"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/tidwall/jsonc"
)

type Spec struct {
	ID               string              `json:"id"`
	Version          string              `json:"version"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	DocumentationURL string              `json:"documentationURL"`
	LicenseURL       string              `json:"licenseURL"`
	Options          map[string]any      `json:"options"`
	Init             bool                `json:"init"`
	Customizations   *SpecCustomizations `json:"customizations,omitempty"`
	InstallsAfter    []string            `json:"installsAfter"`
	ContainerEnv     map[string]string   `json:"containerEnv"`
	EntryPoint       string              `json:"entrypoint"`
}

type SpecCustomizations struct {
	VSCode *SpecCustomizationsVSCode
}

type SpecCustomizationsVSCode struct {
	Extensions []string       `json:"extensions,omitempty"`
	Settings   map[string]any `json:"settings,omitempty"`
}

type LoadSpecOpts struct {
	Data         []byte
	Workspace    *cordiumv1.Workspace
	ContainerEnv []string
}

func LoadSpec(o *LoadSpecOpts) (*Spec, error) {
	ret := &Spec{}
	data, err := dcvars.SubstituteVars(&dcvars.SubstituteVarsOpts{
		Input:        string(jsonc.ToJSON(o.Data)),
		Workspace:    o.Workspace,
		ContainerEnv: o.ContainerEnv,
	})
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(data), ret); err != nil {
		return nil, err
	}

	return ret, nil
}
