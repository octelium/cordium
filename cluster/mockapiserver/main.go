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

package main

import (
	wsc "github.com/octelium/cordium/cluster/common/components"
	api "github.com/octelium/cordium/cluster/mockapiserver/mockapiserver"
	"github.com/octelium/octelium/cluster/common/components"
)

func init() {
	components.SetComponentNamespace(wsc.ComponentNamespaceCordium)
	components.SetComponentType(wsc.MockAPIServer)
}

func main() {
	components.RunComponent(api.Run, nil)
}
