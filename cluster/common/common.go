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
	"fmt"
	"net"
	"strings"

	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
)

func GetMembershipName(spcRef, usrRef *metav1.ObjectReference) string {
	return fmt.Sprintf("%s.%s", usrRef.Name, spcRef.Name)
}

func GetWorkspaceTunnelPort() int {
	if ldflags.IsTest() {
		return 45001
	}
	return 45000
}

func GetInternalRegistryFQDN() string {
	return fmt.Sprintf("workspace-registry.%s.svc", vutils.K8sNS)
}

func GetInternalRegistryAddr() string {
	return net.JoinHostPort(GetInternalRegistryFQDN(), "8080")
}

func CleanupCmdString(arg string) string {
	return strings.TrimSpace(arg)
}
