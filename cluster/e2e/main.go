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

package main

import (
	cscenario "github.com/octelium/cordium/cluster/e2e/scenario"
	"github.com/octelium/octelium/cluster/e2e/cli"
)

func main() {
	cli.Main(cli.Opts{
		Name:            "cordium-e2e",
		Short:           "Provision an environment for the Cordium e2e suite and run it",
		DefaultScenario: cscenario.DefaultScenario,
	})
}
