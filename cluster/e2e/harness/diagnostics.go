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
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/common/vutils"
	k8scorev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	logTailLines  = 50
	maxEventLines = 20

	diagnosticsBudget = 30 * time.Second
)

func DiagnosticsCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), diagnosticsBudget)
}

var stdoutMu sync.Mutex

var (
	streamMu     sync.Mutex
	streamedWS   = map[string]bool{}
	streamedPods = map[string]bool{}
)

func markStreamed(seen map[string]bool, key string) bool {
	streamMu.Lock()
	defer streamMu.Unlock()

	if seen[key] {
		return false
	}
	seen[key] = true

	return true
}

func writeLine(format string, args ...any) {
	stdoutMu.Lock()
	defer stdoutMu.Unlock()

	fmt.Fprintf(os.Stdout, format+"\n", args...)
}

func (h *H) StreamWorkspaceLogs(t *testing.T, ws *cordiumv1.Workspace) {
	t.Helper()

	ctx := t.Context()

	if !markStreamed(streamedWS, ws.Metadata.Uid) {
		return
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			pods, err := h.WorkspacePods(ctx, ws)
			if err == nil {
				for i := range pods {
					pod := pods[i]
					if pod.Status.Phase == k8scorev1.PodPending {
						continue
					}
					if !markStreamed(streamedPods, pod.Name) {
						continue
					}

					go h.streamPodLogs(ctx, WorkspaceNamespace, pod.Name, "workspace",
						fmt.Sprintf("ws/%s", ws.Metadata.Name))
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()
}

func (h *H) streamPodLogs(ctx context.Context, ns, pod, container, prefix string) {
	tail := int64(logTailLines)

	for {
		strm, err := h.K8sC().CoreV1().Pods(ns).GetLogs(pod, &k8scorev1.PodLogOptions{
			Container: container,
			Follow:    true,
			TailLines: &tail,
		}).Stream(ctx)
		if err == nil {
			writeLine("--- [%s] streaming the logs of the pod %s ---", prefix, pod)

			scanner := bufio.NewScanner(strm)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				writeLine("[%s] %s", prefix, scanner.Text())
			}
			strm.Close()

			writeLine("--- [%s] the log stream of the pod %s ended ---", prefix, pod)
		}

		if _, err := h.K8sC().CoreV1().Pods(ns).
			Get(ctx, pod, k8smetav1.GetOptions{}); k8serr.IsNotFound(err) {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (h *H) WorkspaceDiagnostics(ctx context.Context, ws *cordiumv1.Workspace) string {
	var b strings.Builder

	name := fmt.Sprintf("ws-%s", ws.Metadata.Name)
	pvcName := fmt.Sprintf("ws-%s", ws.Metadata.Uid)

	fmt.Fprintf(&b, "--- Workspace %s (uid %s) ---\n", ws.Metadata.Name, ws.Metadata.Uid)

	if cur, err := h.CordiumC().GetWorkspace(ctx,
		&metav1.GetOptions{Uid: ws.Metadata.Uid}); err == nil {
		fmt.Fprintf(&b, "state=%s region=%s failure=%s\n",
			cur.Status.State, cur.Status.RegionRef.GetName(), WorkspaceFailure(cur))
	} else {
		fmt.Fprintf(&b, "the Workspace could not be read: %+v\n", err)
	}

	if dep, err := h.K8sC().AppsV1().Deployments(WorkspaceNamespace).
		Get(ctx, name, k8smetav1.GetOptions{}); err == nil {
		fmt.Fprintf(&b, "Deployment %s: %d/%d ready, %d updated, %d unavailable\n",
			name, dep.Status.ReadyReplicas, dep.Status.Replicas,
			dep.Status.UpdatedReplicas, dep.Status.UnavailableReplicas)
	} else {
		fmt.Fprintf(&b, "Deployment %s: %+v\n", name, err)
	}

	pods, err := h.WorkspacePods(ctx, ws)
	switch {
	case err != nil:
		fmt.Fprintf(&b, "The Workspace pods could not be listed: %+v\n", err)
	case len(pods) == 0:
		fmt.Fprintf(&b, "No Workspace pod exists yet\n")
	default:
		for i := range pods {
			b.WriteString(describePod(&pods[i]))
		}
	}

	b.WriteString(h.describePVC(ctx, pvcName))
	b.WriteString(h.describeStorageClasses(ctx))
	b.WriteString(h.describeNodes(ctx))
	b.WriteString(h.recentEvents(ctx, WorkspaceNamespace, name, pvcName))

	return b.String()
}

func describePod(pod *k8scorev1.Pod) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Pod %s: phase=%s node=%q nodeSelector=%v\n",
		pod.Name, pod.Status.Phase, pod.Spec.NodeName, pod.Spec.NodeSelector)

	for _, container := range pod.Spec.Containers {
		fmt.Fprintf(&b, "  spec container %s: image=%s\n", container.Name, container.Image)
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Status == k8scorev1.ConditionTrue {
			continue
		}
		fmt.Fprintf(&b, "  condition %s=%s %s %s\n",
			cond.Type, cond.Status, cond.Reason, cond.Message)
	}

	for _, cs := range pod.Status.ContainerStatuses {
		switch {
		case cs.State.Waiting != nil:
			fmt.Fprintf(&b, "  container %s: waiting on %s %s\n",
				cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
		case cs.State.Terminated != nil:
			fmt.Fprintf(&b, "  container %s: terminated with %s (exit %d)\n",
				cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
		case cs.State.Running != nil:
			fmt.Fprintf(&b, "  container %s: running since %s (ready=%t restarts=%d)\n",
				cs.Name, cs.State.Running.StartedAt.Format(time.RFC3339),
				cs.Ready, cs.RestartCount)
		}
	}

	return b.String()
}

func (h *H) describePVC(ctx context.Context, name string) string {
	pvc, err := h.K8sC().CoreV1().PersistentVolumeClaims(WorkspaceNamespace).
		Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("PersistentVolumeClaim %s: %+v\n", name, err)
	}

	storageClass := "<default>"
	if pvc.Spec.StorageClassName != nil {
		storageClass = *pvc.Spec.StorageClassName
	}

	return fmt.Sprintf(
		"PersistentVolumeClaim %s: phase=%s storageClass=%s requested=%s volume=%q\n",
		name, pvc.Status.Phase, storageClass,
		pvc.Spec.Resources.Requests.Storage().String(), pvc.Spec.VolumeName)
}

func (h *H) describeStorageClasses(ctx context.Context) string {
	list, err := h.K8sC().StorageV1().StorageClasses().List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		return fmt.Sprintf("The StorageClasses could not be listed: %+v\n", err)
	}

	var b strings.Builder
	for i := range list.Items {
		sc := &list.Items[i]

		bindingMode := ""
		if sc.VolumeBindingMode != nil {
			bindingMode = string(*sc.VolumeBindingMode)
		}

		fmt.Fprintf(&b, "StorageClass %s: provisioner=%s bindingMode=%s default=%s\n",
			sc.Name, sc.Provisioner, bindingMode,
			sc.Annotations["storageclass.kubernetes.io/is-default-class"])
	}

	if b.Len() == 0 {
		return "No StorageClass exists in the Cluster\n"
	}

	return b.String()
}

func (h *H) describeNodes(ctx context.Context) string {
	list, err := h.K8sC().CoreV1().Nodes().List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		return fmt.Sprintf("The nodes could not be listed: %+v\n", err)
	}

	var b strings.Builder
	for i := range list.Items {
		node := &list.Items[i]

		var labels []string
		for k := range node.Labels {
			if strings.HasPrefix(k, "octelium.com/") {
				labels = append(labels, k)
			}
		}
		sort.Strings(labels)

		fmt.Fprintf(&b, "Node %s: octelium labels=[%s]\n",
			node.Name, strings.Join(labels, " "))
	}

	return b.String()
}

func (h *H) recentEvents(ctx context.Context, ns string, names ...string) string {
	list, err := h.K8sC().CoreV1().Events(ns).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		return fmt.Sprintf("The events of the namespace %s could not be listed: %+v\n", ns, err)
	}

	matches := func(name string) bool {
		for _, want := range names {
			if strings.HasPrefix(name, want) {
				return true
			}
		}
		return false
	}

	var items []k8scorev1.Event
	for i := range list.Items {
		if matches(list.Items[i].InvolvedObject.Name) {
			items = append(items, list.Items[i])
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return eventTime(&items[i]).Before(eventTime(&items[j]))
	})

	if len(items) > maxEventLines {
		items = items[len(items)-maxEventLines:]
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Events in the namespace %s:\n", ns)
	for i := range items {
		event := &items[i]
		fmt.Fprintf(&b, "  %s %s/%s %s: %s\n",
			eventTime(event).Format(time.RFC3339), event.Type,
			event.InvolvedObject.Kind, event.Reason, event.Message)
	}

	if len(items) == 0 {
		b.WriteString("  none\n")
	}

	return b.String()
}

func eventTime(event *k8scorev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if event.EventTime.Time.IsZero() {
		return event.FirstTimestamp.Time
	}

	return event.EventTime.Time
}

func (h *H) ComponentLogTail(ctx context.Context, component string) string {
	pods, err := h.CordiumPods(ctx, component)
	if err != nil {
		return fmt.Sprintf("The pods of the component %s could not be listed: %+v\n",
			component, err)
	}

	tail := int64(logTailLines)
	var b strings.Builder

	for i := range pods {
		out, err := h.K8sC().CoreV1().Pods(vutils.K8sNS).
			GetLogs(pods[i].Name, &k8scorev1.PodLogOptions{TailLines: &tail}).DoRaw(ctx)
		if err != nil {
			fmt.Fprintf(&b, "--- %s: could not be read: %+v ---\n", pods[i].Name, err)
			continue
		}

		fmt.Fprintf(&b, "--- %s ---\n%s\n", pods[i].Name, string(out))
	}

	return b.String()
}

func (h *H) Diagnostics(ws *cordiumv1.Workspace) string {
	ctx, cancel := DiagnosticsCtx()
	defer cancel()

	return fmt.Sprintf("%s--- nocturne logs ---\n%s",
		h.WorkspaceDiagnostics(ctx, ws), h.ComponentLogTail(ctx, "nocturne"))
}

func (h *H) PrintClusterDiagnostics(t *testing.T) {
	t.Helper()

	ctx, cancel := DiagnosticsCtx()
	defer cancel()

	writeLine("--- Cluster storage and nodes ---\n%s%s%s",
		h.describeStorageClasses(ctx), h.describeNodes(ctx),
		h.recentEvents(ctx, WorkspaceNamespace, "ws-"))
}
