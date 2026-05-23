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

package get

import (
	"github.com/octelium/cordium/client/cordium/commands/get/gitprovider"
	"github.com/octelium/cordium/client/cordium/commands/get/secret"
	"github.com/octelium/cordium/client/cordium/commands/get/space"
	"github.com/octelium/cordium/client/cordium/commands/get/template"
	"github.com/octelium/cordium/client/cordium/commands/get/usersecret"
	"github.com/octelium/cordium/client/cordium/commands/get/workspace"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use: "get",
}

func AddSubcommands() {
	Cmd.AddCommand(workspace.Cmd)
	Cmd.AddCommand(space.Cmd)
	Cmd.AddCommand(template.Cmd)
	Cmd.AddCommand(secret.Cmd)
	Cmd.AddCommand(usersecret.Cmd)
	Cmd.AddCommand(gitprovider.Cmd)
}
