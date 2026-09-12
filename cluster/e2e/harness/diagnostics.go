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
	"github.com/pkg/errors"
	k8scorev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	logTailLines  = 50
	maxEventLines = 20

	logRetryInterval  = 3 * time.Second
	crashLoopRestarts = 3

	diagnosticsBudget = 30 * time.Second
)

const workspaceContainer = "workspace"

func DiagnosticsCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), diagnosticsBudget)
}

var stdoutMu sync.Mutex

var (
	streamMu     sync.Mutex
	streamedWS   = map[string]bool{}
	streamedPods = map[string]bool{}
)

var (
	firstLogMu sync.Mutex
	firstLogs  = map[string]string{}
)

func recordFirstLog(pod string, lines []string) {
	firstLogMu.Lock()
	defer firstLogMu.Unlock()

	if _, ok := firstLogs[pod]; ok || len(lines) == 0 {
		return
	}

	firstLogs[pod] = strings.Join(lines, "\n")
}

func firstLog(pod string) string {
	firstLogMu.Lock()
	defer firstLogMu.Unlock()

	return firstLogs[pod]
}

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

					go h.streamPodLogs(ctx, WorkspaceNamespace, pod.Name,
						workspaceContainer, fmt.Sprintf("ws/%s", ws.Metadata.Name))
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
	var streamed string
	var reported int32

	for {
		cur, err := h.K8sC().CoreV1().Pods(ns).Get(ctx, pod, k8smetav1.GetOptions{})
		if k8serr.IsNotFound(err) {
			writeLine("--- [%s] the pod %s no longer exists ---", prefix, pod)
			return
		}

		if cs := containerStatus(cur, container); err == nil && cs != nil {
			if cs.RestartCount > reported {
				reported = cs.RestartCount
				writeLine("--- [%s] the container %s of the pod %s has restarted %d time(s), "+
					"the previous instance %s ---\n%s",
					prefix, container, pod, cs.RestartCount, lastTermination(cs),
					KubectlHints(ns, pod, container))
			}

			if gen := containerGeneration(cs); gen != "" && gen != streamed {
				streamed = gen
				h.followPodLogs(ctx, ns, pod, container, prefix)
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(logRetryInterval):
		}
	}
}

func (h *H) followPodLogs(ctx context.Context, ns, pod, container, prefix string) {
	tail := int64(logTailLines)

	strm, err := h.K8sC().CoreV1().Pods(ns).GetLogs(pod, &k8scorev1.PodLogOptions{
		Container: container,
		Follow:    true,
		TailLines: &tail,
	}).Stream(ctx)
	if err != nil {
		return
	}
	defer strm.Close()

	writeLine("--- [%s] streaming the logs of the pod %s ---\n%s",
		prefix, pod, KubectlHints(ns, pod, container))

	var tailed []string

	scanner := bufio.NewScanner(strm)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		tailed = append(tailed, line)
		if len(tailed) > logTailLines {
			tailed = tailed[1:]
		}

		writeLine("[%s] %s", prefix, line)
	}

	recordFirstLog(pod, tailed)

	writeLine("--- [%s] the log stream of the pod %s ended ---", prefix, pod)
}

func KubectlHints(ns, pod, container string) string {
	return fmt.Sprintf("To inspect the pod directly:\n"+
		"  kubectl -n %s get pod %s -o wide\n"+
		"  kubectl -n %s describe pod %s\n"+
		"  kubectl -n %s logs %s -c %s -f\n"+
		"  kubectl -n %s logs %s -c %s --previous",
		ns, pod, ns, pod, ns, pod, container, ns, pod, container)
}

func containerStatus(pod *k8scorev1.Pod, container string) *k8scorev1.ContainerStatus {
	if pod == nil {
		return nil
	}

	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == container {
			return &pod.Status.ContainerStatuses[i]
		}
	}

	return nil
}

func containerGeneration(cs *k8scorev1.ContainerStatus) string {
	switch {
	case cs.State.Running != nil:
		return fmt.Sprintf("%s/%s", cs.ContainerID, cs.State.Running.StartedAt)
	case cs.State.Terminated != nil:
		return fmt.Sprintf("%s/%s", cs.ContainerID, cs.State.Terminated.StartedAt)
	default:
		return ""
	}
}

func lastTermination(cs *k8scorev1.ContainerStatus) string {
	term := cs.LastTerminationState.Terminated
	if term == nil {
		term = cs.State.Terminated
	}
	if term == nil {
		return "did not report a termination state"
	}

	return fmt.Sprintf("exited with the code %d (%s) at %s",
		term.ExitCode, term.Reason, term.FinishedAt.Format(time.RFC3339))
}

func (h *H) CheckWorkspacePodRestarts(ctx context.Context, ws *cordiumv1.Workspace) error {
	pods, err := h.WorkspacePods(ctx, ws)
	if err != nil {
		return nil
	}

	for i := range pods {
		cs := containerStatus(&pods[i], workspaceContainer)
		if cs == nil || cs.RestartCount < crashLoopRestarts {
			continue
		}

		return errors.Errorf(
			"the container %s of the pod %s has restarted %d time(s), the previous instance %s",
			workspaceContainer, pods[i].Name, cs.RestartCount, lastTermination(cs))
	}

	return nil
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
			b.WriteString(firstLogTail(&pods[i]))
			b.WriteString(h.previousLogTail(ctx, &pods[i]))
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

	for i := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[i]

		switch {
		case cs.State.Waiting != nil:
			fmt.Fprintf(&b, "  container %s: waiting on %s %s\n",
				cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message)
		case cs.State.Terminated != nil:
			fmt.Fprintf(&b, "  container %s: terminated with %s (exit %d)\n",
				cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode)
		case cs.State.Running != nil:
			fmt.Fprintf(&b, "  container %s: running since %s (ready=%t)\n",
				cs.Name, cs.State.Running.StartedAt.Format(time.RFC3339), cs.Ready)
		}

		if cs.RestartCount > 0 {
			fmt.Fprintf(&b, "  container %s: restarts=%d, the previous instance %s\n",
				cs.Name, cs.RestartCount, lastTermination(cs))
		}
	}

	fmt.Fprintf(&b, "%s\n", KubectlHints(pod.Namespace, pod.Name, workspaceContainer))

	return b.String()
}

func firstLogTail(pod *k8scorev1.Pod) string {
	cs := containerStatus(pod, workspaceContainer)
	if cs == nil || cs.RestartCount == 0 {
		return ""
	}

	out := firstLog(pod.Name)
	if out == "" {
		return ""
	}

	return fmt.Sprintf("--- the first instance of the pod %s, which is the one that "+
		"explains why the container keeps restarting ---\n%s\n", pod.Name, out)
}

func (h *H) previousLogTail(ctx context.Context, pod *k8scorev1.Pod) string {
	cs := containerStatus(pod, workspaceContainer)
	if cs == nil || cs.RestartCount == 0 {
		return ""
	}

	tail := int64(logTailLines)

	out, err := h.K8sC().CoreV1().Pods(pod.Namespace).
		GetLogs(pod.Name, &k8scorev1.PodLogOptions{
			Container: workspaceContainer,
			Previous:  true,
			TailLines: &tail,
		}).DoRaw(ctx)
	if err != nil {
		return fmt.Sprintf("The previous logs of the pod %s could not be read: %+v\n",
			pod.Name, err)
	}

	return fmt.Sprintf("--- the previous instance of the pod %s ---\n%s\n",
		pod.Name, string(out))
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
