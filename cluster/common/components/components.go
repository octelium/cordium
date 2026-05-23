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

package components

import (
	"fmt"

	"github.com/octelium/octelium/pkg/utils/ldflags"
)

const APIServer = "apiserver"
const Nocturne = "nocturne"
const Portal = "portal"
const Supervisor = "supervisor"
const Workspace = "workspace"
const Genesis = "genesis"
const RscServer = "rscserver"
const Vigil = "vigil"
const MockAPIServer = "mockapiserver"

const ComponentNamespaceCordium = "cordium"

func GetImage(component, version string) string {
	return ldflags.GetImage(fmt.Sprintf("cordium-%s", component), version)
}

func CordiumComponent(arg string) string {
	return fmt.Sprintf("cordium-%s", arg)
}
