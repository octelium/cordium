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
