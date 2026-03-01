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

package devcontainers

import (
	"encoding/json"
	"strings"

	"github.com/octelium/cordium/cluster/common/devcontainers/dcvars"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/tidwall/jsonc"
)

type Spec struct {
	Name string `json:"name"`

	Image      string     `json:"image"`
	Build      *SpecBuild `json:"build"`
	Dockerfile string     `json:"dockerFile"`

	ContainerEnv map[string]string `json:"containerEnv"`

	Init       bool     `json:"init"`
	Privileged bool     `json:"privileged"`
	CapAdd     []string `json:"capAdd"`

	OverrideCommand *bool  `json:"overrideCommand"`
	RemoteUser      string `json:"remoteUser"`

	OnCreateCommand      *SpecCommand `json:"onCreateCommand,omitempty"`
	UpdateContentCommand *SpecCommand `json:"updateContentCommand,omitempty"`
	PostCreateCommand    *SpecCommand `json:"postCreateCommand"`
	PostStartCommand     *SpecCommand `json:"postStartCommand,omitempty"`
	PostAttachCommand    *SpecCommand `json:"postAttachCommand,omitempty"`

	Settings   map[string]any `json:"settings,omitempty"`
	Extensions []string       `json:"extensions,omitempty"`

	DockerComposeFile string `json:"dockerComposeFile"`
	Service           string `json:"service"`

	Features map[string]any `json:"features"`

	Customizations *SpecCustomizations `json:"customizations,omitempty"`
}

type SpecCustomizations struct {
	VSCode *SpecCustomizationsVSCode `json:"vscode,omitempty"`
}

type SpecCustomizationsVSCode struct {
	Extensions []string `json:"extensions,omitempty"`
}

type SpecBuild struct {
	Dockerfile string            `json:"dockerfile"`
	Context    string            `json:"context"`
	Args       map[string]string `json:"args"`
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

type SpecCommand struct {
	Commands []string `json:"commands"`
}

func (c *SpecCommand) UnmarshalJSON(data []byte) error {
	var initJSON interface{}
	err := json.Unmarshal(data, &initJSON)
	if err != nil {
		return err
	}
	switch val := initJSON.(type) {
	case string:
		c.Commands = []string{val}
		return nil
	case []any:
		var cmdArgs []string
		for _, v := range val {
			value, ok := v.(string)
			if !ok {
				return &json.SyntaxError{}
			}
			cmdArgs = append(cmdArgs, value)
		}
		c.Commands = append(c.Commands, strings.Join(cmdArgs, " "))
		return nil
	case map[string]any:
		for _, v := range val {
			if value, ok := v.(string); ok {
				c.Commands = append(c.Commands, value)
			} else if arr, ok := v.([]any); ok {
				var cmdArgs []string
				for _, v := range arr {
					value, ok := v.(string)
					if !ok {
						return &json.SyntaxError{}
					}
					cmdArgs = append(cmdArgs, value)
				}
				c.Commands = append(c.Commands, strings.Join(cmdArgs, " "))
			} else {
				return &json.SyntaxError{}
			}
		}

		return nil
	}

	return &json.SyntaxError{}
}
