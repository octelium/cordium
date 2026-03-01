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

package workspacecommon

import (
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
)

func GetMembershipUserInfo(usr *corev1.User) *cordiumv1.Membership_Status_UserInfo {
	if usr == nil {
		return nil
	}

	ret := &cordiumv1.Membership_Status_UserInfo{
		DisplayName: usr.Metadata.DisplayName,
		PicURL:      usr.Metadata.PicURL,
	}

	return ret
}

const K8sNS = "cordium"
