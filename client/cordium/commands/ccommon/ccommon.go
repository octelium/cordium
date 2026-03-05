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

package ccommon

import (
	"strings"

	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
)

func GetResourceShortName(rsc umetav1.ResourceObjectI) string {
	if rsc == nil || rsc.GetMetadata() == nil {
		return ""
	}

	return doGetShortName(rsc.GetMetadata().Name)
}

func GetResourceRefShortName(itm *metav1.ObjectReference) string {
	if itm == nil {
		return ""
	}

	return doGetShortName(itm.Name)
}

func doGetShortName(arg string) string {
	if arg == "" {
		return ""
	}
	return strings.Split(arg, ".")[0]
}
