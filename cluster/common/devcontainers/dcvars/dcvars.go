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

package dcvars

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/octelium/octelium/apis/main/cordiumv1"
)

type SubstituteVarsOpts struct {
	Input        string
	Workspace    *cordiumv1.Workspace
	ContainerEnv []string
}

var re = regexp.MustCompile(`\$\{(.*?)\}`)

func toMap(arg []string) map[string]string {
	ret := make(map[string]string)
	for _, itm := range arg {
		args := strings.Split(itm, "=")
		if len(args) == 2 {
			ret[args[0]] = args[1]
		}
	}

	return ret
}

func SubstituteVars(o *SubstituteVarsOpts) (string, error) {
	matches := re.FindStringSubmatch(o.Input)

	containerEnv := toMap(o.ContainerEnv)

	ret := o.Input
	for _, match := range matches {
		switch {
		case match == "PATH":
			ret = doSubstitute(ret, match, containerEnv["PATH"])
		case match == "localWorkspaceFolder":
		case match == "devcontainerId":
			if o.Workspace != nil {
				ret = doSubstitute(ret, match, o.Workspace.Metadata.Uid)
			}
		case len(strings.Split(match, ":")) == 2:
			varArgs := strings.Split(match, ":")
			switch {
			case varArgs[0] == "containerEnv":
				if v := containerEnv[varArgs[1]]; v != "" {
					ret = doSubstitute(ret, match, v)
				} else {
					ret = strings.ReplaceAll(ret, fmt.Sprintf("${%s}:", match), "")
				}

			case varArgs[0] == "localEnv":
				ret = doSubstitute(ret, match, "")
			}
		}

	}

	return ret, nil
}

func doSubstitute(in, old, new string) string {
	return strings.ReplaceAll(in, fmt.Sprintf(`${%s}`, old), new)
}
