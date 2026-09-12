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

package scenario

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/octelium/octelium/cluster/common/k8sutils"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/cluster/e2e/scenario"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	genesisBudget    = 20 * time.Minute
	deploymentBudget = 10 * time.Minute

	pollInterval    = 3 * time.Second
	pollReportEvery = 15 * time.Second

	logTailLines = 50
)

func stepWaitDeployments(ctx context.Context, r *scenario.Runner) error {
	k8sC, err := r.K8sC()
	if err != nil {
		return err
	}

	if err := waitGenesis(ctx, k8sC); err != nil {
		return err
	}

	for _, name := range Deployments {
		zap.L().Debug("Waiting for the Cordium Deployment readiness",
			zap.String("deployment", name))

		err := pollUntil(ctx, fmt.Sprintf("the deployment %s", name), deploymentBudget,
			func(ctx context.Context) error {
				return deploymentReadiness(ctx, k8sC, name)
			})
		if err != nil {
			return err
		}
	}

	return nil
}

func waitGenesis(ctx context.Context, k8sC kubernetes.Interface) error {
	prefix := fmt.Sprintf("octelium-genesis-init-%s-", Package)

	return pollUntil(ctx, fmt.Sprintf("the %s genesis Job", Package), genesisBudget,
		func(ctx context.Context) error {
			job, err := latestJob(ctx, k8sC, prefix)
			if err != nil {
				return err
			}

			selector := fmt.Sprintf("job-name=%s", job.Name)

			for _, c := range job.Status.Conditions {
				if c.Status != k8scorev1.ConditionTrue {
					continue
				}
				switch c.Type {
				case batchv1.JobComplete:
					return nil
				case batchv1.JobFailed:
					return fatal(errors.Errorf("The Job %s failed: %s %s. Pods: %s. Logs:\n%s",
						job.Name, c.Reason, c.Message,
						notReadyPods(ctx, k8sC, selector),
						podLogsTail(ctx, k8sC, selector)))
				}
			}

			return errors.Errorf("The Job %s is still running (%d active, %d failed). Pods: %s",
				job.Name, job.Status.Active, job.Status.Failed,
				notReadyPods(ctx, k8sC, selector))
		})
}

func latestJob(ctx context.Context,
	k8sC kubernetes.Interface, prefix string) (*batchv1.Job, error) {
	jobList, err := k8sC.BatchV1().Jobs(vutils.K8sNS).List(ctx, k8smetav1.ListOptions{
		LabelSelector: "octelium.com/component=genesis",
	})
	if err != nil {
		return nil, err
	}

	var ret *batchv1.Job
	for i := range jobList.Items {
		job := &jobList.Items[i]
		if !strings.HasPrefix(job.Name, prefix) {
			continue
		}
		if ret == nil || job.CreationTimestamp.After(ret.CreationTimestamp.Time) {
			ret = job
		}
	}

	if ret == nil {
		return nil, errors.Errorf("No Job named %s* has been created yet", prefix)
	}

	return ret, nil
}

func deploymentReadiness(ctx context.Context,
	k8sC kubernetes.Interface, name string) error {
	dep, err := k8sC.AppsV1().Deployments(vutils.K8sNS).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		return err
	}

	rs, err := k8sutils.GetNewReplicaSet(dep, k8sC.AppsV1())
	if err != nil {
		return err
	}
	if rs == nil {
		return errors.Errorf("The Deployment has no current ReplicaSet yet")
	}

	want := *dep.Spec.Replicas - k8sutils.MaxUnavailable(*dep)
	if rs.Status.ReadyReplicas >= want {
		return nil
	}

	return errors.Errorf("%d of %d replicas are ready. Pods: %s",
		rs.Status.ReadyReplicas, want,
		notReadyPods(ctx, k8sC, k8smetav1.FormatLabelSelector(rs.Spec.Selector)))
}

func notReadyPods(ctx context.Context, k8sC kubernetes.Interface, selector string) string {
	podList, err := k8sC.CoreV1().Pods(vutils.K8sNS).List(ctx, k8smetav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Sprintf("could not be listed: %s", err.Error())
	}

	var ret []string
	for i := range podList.Items {
		pod := &podList.Items[i]
		if isPodReady(pod) {
			continue
		}

		ret = append(ret, podState(pod))
	}

	if len(ret) == 0 {
		return "none exist yet"
	}

	return strings.Join(ret, "; ")
}

func podLogsTail(ctx context.Context, k8sC kubernetes.Interface, selector string) string {
	podList, err := k8sC.CoreV1().Pods(vutils.K8sNS).List(ctx, k8smetav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return fmt.Sprintf("could not be listed: %s", err.Error())
	}

	tail := int64(logTailLines)
	var ret []string

	for i := range podList.Items {
		pod := &podList.Items[i]

		out, err := k8sC.CoreV1().Pods(vutils.K8sNS).GetLogs(pod.Name, &k8scorev1.PodLogOptions{
			TailLines: &tail,
		}).DoRaw(ctx)
		if err != nil {
			ret = append(ret, fmt.Sprintf("%s: could not be read: %s", pod.Name, err.Error()))
			continue
		}

		ret = append(ret, fmt.Sprintf("--- %s ---\n%s", pod.Name, string(out)))
	}

	if len(ret) == 0 {
		return "no pods exist"
	}

	return strings.Join(ret, "\n")
}

func isPodReady(pod *k8scorev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == k8scorev1.PodReady {
			return cond.Status == k8scorev1.ConditionTrue
		}
	}

	return false
}

func podState(pod *k8scorev1.Pod) string {
	var reasons []string

	for _, cond := range pod.Status.Conditions {
		if cond.Type == k8scorev1.PodScheduled && cond.Status != k8scorev1.ConditionTrue {
			reasons = append(reasons,
				strings.TrimSpace(fmt.Sprintf("unscheduled %s %s", cond.Reason, cond.Message)))
		}
	}

	for _, cs := range slices.Concat(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses) {
		switch {
		case cs.State.Waiting != nil:
			reasons = append(reasons, fmt.Sprintf("%s waiting on %s %s",
				cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message))
		case cs.State.Terminated != nil:
			reasons = append(reasons, fmt.Sprintf("%s terminated with %s (exit %d)",
				cs.Name, cs.State.Terminated.Reason, cs.State.Terminated.ExitCode))
		case cs.State.Running != nil && !cs.Ready:
			reasons = append(reasons, fmt.Sprintf("%s running but not ready (%d restarts)",
				cs.Name, cs.RestartCount))
		}
	}

	if len(reasons) == 0 {
		return fmt.Sprintf("%s is %s", pod.Name, pod.Status.Phase)
	}

	return fmt.Sprintf("%s is %s: %s",
		pod.Name, pod.Status.Phase, strings.Join(reasons, ", "))
}

type fatalError struct {
	err error
}

func (f *fatalError) Error() string { return f.err.Error() }

func fatal(err error) error { return &fatalError{err: err} }

func pollUntil(ctx context.Context, what string,
	budget time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	started := time.Now()
	reported := time.Now()
	attempts := 0

	var lastErr error
	for {
		attempts++
		lastErr = fn(ctx)
		if lastErr == nil {
			zap.L().Debug("Condition met",
				zap.String("what", what),
				zap.Int("attempts", attempts),
				zap.Duration("elapsed", time.Since(started)))
			return nil
		}

		var f *fatalError
		if errors.As(lastErr, &f) {
			return errors.Errorf("Gave up waiting for %s after %s: %+v",
				what, time.Since(started).Truncate(time.Millisecond), f.err)
		}

		if time.Since(reported) >= pollReportEvery {
			reported = time.Now()
			zap.L().Info("Still waiting",
				zap.String("what", what),
				zap.Int("attempts", attempts),
				zap.Duration("elapsed", time.Since(started).Truncate(time.Second)),
				zap.Duration("budget", budget),
				zap.NamedError("lastError", lastErr))
		}

		select {
		case <-ctx.Done():
			return errors.Errorf(
				"Timed out after %s waiting for %s (%d attempts). Last error: %+v",
				time.Since(started).Truncate(time.Millisecond), what, attempts, lastErr)
		case <-time.After(pollInterval):
		}
	}
}
