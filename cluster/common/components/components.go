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
