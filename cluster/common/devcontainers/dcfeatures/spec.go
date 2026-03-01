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
