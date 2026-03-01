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
