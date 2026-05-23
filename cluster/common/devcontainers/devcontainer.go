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
