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

package harness

import (
	"context"
	"fmt"
	"testing"
	"time"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/cluster/e2e/harness"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	StartBudget       = 5 * time.Minute
	StopBudget        = 2 * time.Minute
	ExecBudget        = 60 * time.Second
	SessionBudget     = 2 * time.Minute
	PropagationBudget = 90 * time.Second
)

const WorkspaceNamespace = workspacecommon.K8sNS

const componentSelector = "octelium.com/app=cordium,octelium.com/component=%s"

const workspaceSelector = "octelium.com/component=workspace,octelium.com/workspace-uid=%s"

type H struct {
	*harness.H
}

func Wrap(h *harness.H) *H {
	return &H{H: h}
}

func (h *H) CordiumC() cordiumv1.MainServiceClient {
	return cordiumv1.NewMainServiceClient(h.Conn())
}

func (h *H) WorkspaceC() cordiumv1.WorkspaceServiceClient {
	return cordiumv1.NewWorkspaceServiceClient(h.Conn())
}

func (h *H) ManagementC() cordiumv1.ManagementServiceClient {
	return cordiumv1.NewManagementServiceClient(h.Conn())
}

func (h *H) Ctx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	if t.Context().Err() == nil {
		return context.WithCancel(t.Context())
	}

	return context.WithTimeout(context.Background(), 60*time.Second)
}

func (h *H) ClusterConfig(t *testing.T) *cordiumv1.ClusterConfig {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	ret, err := h.ManagementC().GetClusterConfig(ctx, &cordiumv1.GetClusterConfigRequest{})
	if err != nil {
		t.Fatalf("Could not get the Cordium ClusterConfig: %+v", err)
	}

	return ret
}

func (h *H) CordiumPods(ctx context.Context, component string) ([]k8scorev1.Pod, error) {
	podList, err := h.K8sC().CoreV1().Pods(vutils.K8sNS).List(ctx, k8smetav1.ListOptions{
		LabelSelector: fmt.Sprintf(componentSelector, component),
	})
	if err != nil {
		return nil, err
	}

	if len(podList.Items) < 1 {
		return nil, errors.Errorf("No Cordium pods found for the component %q", component)
	}

	return podList.Items, nil
}

func (h *H) CheckCordiumRestarts(t *testing.T, component string) {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	pods, err := h.CordiumPods(ctx, component)
	if err != nil {
		t.Errorf("%+v", err)
		return
	}

	for _, pod := range pods {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.RestartCount > 0 {
				reason := ""
				if cs.LastTerminationState.Terminated != nil {
					reason = cs.LastTerminationState.Terminated.Reason
				}
				t.Errorf("Component %s: container %s of pod %s restarted %d time(s) (last reason: %q)",
					component, cs.Name, pod.Name, cs.RestartCount, reason)
			}
		}
	}
}

func (h *H) WorkspacePods(ctx context.Context, ws *cordiumv1.Workspace) ([]k8scorev1.Pod, error) {
	podList, err := h.K8sC().CoreV1().Pods(WorkspaceNamespace).List(ctx, k8smetav1.ListOptions{
		LabelSelector: fmt.Sprintf(workspaceSelector, ws.Metadata.Uid),
	})
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}

func (h *H) StartCordiumLogStreams(t *testing.T, components ...string) {
	t.Helper()

	for _, component := range components {
		selector := fmt.Sprintf(componentSelector, component)

		if err := h.StartLogStream(t.Context(), "-l "+selector); err != nil {
			zap.L().Warn("Could not stream the component logs",
				zap.String("component", component), zap.Error(err))
		}
	}
}
