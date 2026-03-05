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

package controller

import (
	"context"
	"fmt"

	v1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/k8sutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	utils_types "github.com/octelium/octelium/pkg/utils/types"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const ns = workspacecommon.K8sNS

func (c *Controller) doOnAdd(ctx context.Context, ws *cordiumv1.Workspace) error {

	zap.S().Debugf("Creating Workspace deployment for: %s", ws.Metadata.Name)

	if err := c.k8sC.CoreV1().ConfigMaps(ns).Delete(ctx, getK8sRscName(ws), metav1.DeleteOptions{}); err != nil {
		if !k8serr.IsNotFound(err) {
			return err
		}
	}

	ownerCM, err := c.k8sC.CoreV1().ConfigMaps(ns).Create(ctx, c.getOwnerConfigMap(ws), metav1.CreateOptions{})
	if err != nil {
		return err
	}

	if _, err := c.k8sC.AppsV1().
		Deployments(ns).
		Create(ctx, c.newDeployment(ws, ownerCM), metav1.CreateOptions{}); err != nil {
		return err
	}

	if err := c.createK8sService(ctx, ws, ownerCM); err != nil {
		return err
	}

	return nil
}

func getResourceQuantity(arg string) *resource.Quantity {
	ret := resource.MustParse(arg)
	return &ret
}

func (c *Controller) newPodSpec(ws *cordiumv1.Workspace) corev1.PodSpec {

	storageLimit := func() int64 {

		return 250 * 1000
	}()

	return corev1.PodSpec{
		NodeSelector: func() map[string]string {
			return map[string]string{
				"octelium.com/node-mode-cordium": "",
			}
		}(),

		AutomountServiceAccountToken: utils_types.BoolToPtr(false),
		EnableServiceLinks:           utils_types.BoolToPtr(false),

		SecurityContext: &corev1.PodSecurityContext{
			Sysctls: []corev1.Sysctl{
				{
					Name:  "net.ipv4.ip_unprivileged_port_start",
					Value: "1",
				},
				{
					Name:  "net.ipv4.ping_group_range",
					Value: "0 2147483647",
				},
				{
					Name:  "net.ipv4.tcp_syncookies",
					Value: "1",
				},
			},
		},

		ImagePullSecrets: func() []corev1.LocalObjectReference {
			if ovutils.IsPrivateRegistry() {
				return []corev1.LocalObjectReference{
					{
						Name: "octelium-regcred",
					},
				}
			}

			return nil
		}(),

		Hostname:                      "octelium",
		TerminationGracePeriodSeconds: utils_types.Int64ToPtr(120),
		Volumes: func() []corev1.Volume {
			ret := []corev1.Volume{

				{
					Name: "octelium",
					VolumeSource: corev1.VolumeSource{

						/*
							EmptyDir: &corev1.EmptyDirVolumeSource{
								SizeLimit: getResourceQuantity(fmt.Sprintf("%dMi", int64(float64(storageLimit)*1.5))),
							},
						*/

						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: c.getPVCName(ws),
						},
					},
				},

				{
					Name: "var-tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							SizeLimit: getResourceQuantity(fmt.Sprintf("%dMi", int64(float64(storageLimit)*1.5))),
						},
					},
				},

				{
					Name: "octelium-root",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							SizeLimit: getResourceQuantity("1000Mi"),
						},
					},
				},
			}

			if ovutils.IsPrivateRegistry() {
				ret = append(ret, corev1.Volume{
					Name: "octelium-regcred",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "octelium-regcred",
						},
					},
				})
			}

			return ret
		}(),

		// RestartPolicyNever does not work
		// RestartPolicy: corev1.RestartPolicyNever,

		Containers: []corev1.Container{
			{
				Name:            "workspace",
				Image:           components.GetImage(components.Supervisor, ""),
				ImagePullPolicy: k8sutils.GetImagePullPolicy(),
				Env: func() []corev1.EnvVar {
					var ret []corev1.EnvVar

					ret = append(ret, corev1.EnvVar{
						Name:  "OCTELIUM_WS_NAME",
						Value: ws.Metadata.Name,
					})

					ret = append(ret, corev1.EnvVar{
						Name:  "OCTELIUM_WS_UID",
						Value: ws.Metadata.Uid,
					})

					/*
						if ldflags.IsDev() {
							ret = append(ret, corev1.EnvVar{
								Name:  "GRPC_GO_LOG_VERBOSITY_LEVEL",
								Value: "99",
							})

							ret = append(ret, corev1.EnvVar{
								Name:  "GRPC_GO_LOG_SEVERITY_LEVEL",
								Value: "info",
							})
						}
					*/

					return ret
				}(),

				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("5Mi"),
						corev1.ResourceCPU:    resource.MustParse("10m"),
						// corev1.ResourceEphemeralStorage: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse(
							fmt.Sprintf("%dMi", func() uint32 {
								if ws.Status.Limit == nil || ws.Status.Limit.Memory == nil ||
									ws.Status.Limit.Memory.Megabytes == 0 {
									return 512
								}
								return ws.Status.Limit.Memory.Megabytes
							}())),
						corev1.ResourceCPU: resource.MustParse(
							fmt.Sprintf("%dm", func() uint32 {
								if ws.Status.Limit == nil || ws.Status.Limit.Cpu == nil ||
									ws.Status.Limit.Cpu.Millicores == 0 {
									return 500
								}

								return ws.Status.Limit.Cpu.Millicores
							}())),

						/*
							corev1.ResourceEphemeralStorage: resource.MustParse(
								fmt.Sprintf("%dMi", func() int64 {
									if ws.Status.Limit == nil || ws.Status.Limit.Storage == nil ||
										ws.Status.Limit.Storage.Megabytes == 0 {
										return 1000
									}
									return ws.Status.Limit.Storage.Megabytes
								}())),
						*/
					},
				},

				SecurityContext: &corev1.SecurityContext{
					Privileged:             utils_types.BoolToPtr(true),
					ReadOnlyRootFilesystem: utils_types.BoolToPtr(false),

					RunAsUser:  utils_types.Int64ToPtr(0),
					RunAsGroup: utils_types.Int64ToPtr(0),

					Capabilities: &corev1.Capabilities{
						/*
							Drop: []corev1.Capability{
								"ALL",
							},
						*/

						/*
							Add: []corev1.Capability{
								"NET_BIND_SERVICE",
								"NET_RAW",
								"NET_ADMIN",

								"SYS_ADMIN",

								"MKNOD",
								"SYS_CHROOT",
								"SETFCAP",

								"SETUID",
								"SETGID",

								"CHOWN",
								"FOWNER",
								"KILL",
								"DAC_OVERRIDE",
								"FSETID",
								"DAC_READ_SEARCH",
							},
						*/

						/*
							Add: []corev1.Capability{
								"NET_BIND_SERVICE",
								"NET_RAW",
								"NET_ADMIN",

								"KILL",

								"MKNOD",

								"SYS_ADMIN",

								"SETUID",
								"SETGID",
								"FSETID",
								"SYS_CHROOT",

								"SETPCAP",
								"SETFCAP",

								"DAC_OVERRIDE",
								"AUDIT_WRITE",
								"CHOWN",
								"FOWNER",
							},
						*/
					},
				},

				VolumeMounts: func() []corev1.VolumeMount {
					ret := []corev1.VolumeMount{

						/*
							{
								Name:      "varcroot",
								MountPath: "/var/lib/containers",
							},
						*/

						/*
							{
								Name:      "varcusr",
								MountPath: "/home/octelium/.local/share/containers",
							},
						*/

						{
							Name:      "octelium",
							MountPath: "/octelium",
						},

						{
							Name:      "octelium-root",
							MountPath: "/octelium-root",
						},

						{
							Name:      "var-tmp",
							MountPath: "/var/tmp",
						},

						/*
							{
								Name:      "dind",
								MountPath: "/octelium-dind",
							},
						*/
					}

					if ovutils.IsPrivateRegistry() {
						ret = append(ret, corev1.VolumeMount{
							Name:      "octelium-regcred",
							MountPath: "/etc/regcred.json",
							SubPath:   ".dockerconfigjson",
						})
					}
					return ret
				}(),
			},
		},
	}
}

func (c *Controller) newDeployment(ws *cordiumv1.Workspace, ownerCM *corev1.ConfigMap) *appsv1.Deployment {

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getK8sRscName(ws),
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ownerCM, corev1.SchemeGroupVersion.WithKind("ConfigMap")),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(ws),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      getLabels(ws),
					Annotations: c.getAnnotations(ws),
				},
				Spec: c.newPodSpec(ws),
			},
		},
	}
}

func (c *Controller) getAnnotations(ws *cordiumv1.Workspace) map[string]string {
	return map[string]string{
		"cluster-autoscaler.kubernetes.io/safe-to-evict":           "false",
		"container.apparmor.security.beta.kubernetes.io/workspace": "unconfined",
	}
}

func (c *Controller) createK8sService(ctx context.Context,
	ws *cordiumv1.Workspace, ownerCM *corev1.ConfigMap) error {
	svcK8s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getK8sRscName(ws),
			Namespace: ns,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ownerCM, corev1.SchemeGroupVersion.WithKind("ConfigMap")),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: getLabels(ws),
			Ports: func() []corev1.ServicePort {

				ret := []corev1.ServicePort{
					{
						Name:     "grpc",
						Protocol: corev1.ProtocolTCP,
						Port:     8080,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 8080,
						},
					},
					{
						Name:     "essh",
						Protocol: corev1.ProtocolTCP,
						Port:     2022,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 2022,
						},
					},
					{
						Name:     "tunnel",
						Protocol: corev1.ProtocolUDP,
						Port:     int32(workspacecommon.GetWorkspaceTunnelPort()),
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: int32(workspacecommon.GetWorkspaceTunnelPort()),
						},
					},
				}

				return ret
			}(),
		},
	}

	if _, err := c.k8sC.CoreV1().Services(ns).Create(ctx, svcK8s, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (c *Controller) getOwnerConfigMap(ws *cordiumv1.Workspace) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getK8sRscName(ws),
			Namespace: ns,
		},
		Data: map[string]string{
			"uid": ws.Metadata.Uid,
		},
	}
}

func getK8sRscName(ws *cordiumv1.Workspace) string {
	return fmt.Sprintf("ws-%s", ws.Metadata.Name)
}

func getLabels(ws *cordiumv1.Workspace) map[string]string {
	return map[string]string{
		"app":                         "octelium",
		"octelium.com/component-type": "user",
		"octelium.com/component":      "workspace",
		"octelium.com/workspace-uid":  ws.Metadata.Uid,
	}
}

func (c *Controller) doOnDeleteK8s(ctx context.Context, ws *cordiumv1.Workspace) error {
	zap.S().Debugf("Deleting k8s resources of Workspace: %s", ws.Metadata.Name)
	if err := c.k8sC.CoreV1().ConfigMaps(ns).Delete(ctx, getK8sRscName(ws), metav1.DeleteOptions{}); err != nil {
		if k8serr.IsNotFound(err) {
			zap.S().Debugf("Owner configmap of Workspace is already deleted. Nothing to be done: %s",
				ws.Metadata.Name)
			return nil
		}
		return err
	}

	return nil
}

func DoDeleteWorkspaceOwner(ctx context.Context, ws *cordiumv1.Workspace, k8sC kubernetes.Interface) error {

	zap.S().Debugf("Deleting owner configmap of Workspace: %s", ws.Metadata.Name)

	if ldflags.IsDev() {
		podList, err := k8sC.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("octelium.com/workspace-uid=%s", ws.Metadata.Uid),
		})
		if err == nil {
			zap.L().Debug("To be deleted podList", zap.Any("podList", podList))
			if len(podList.Items) == 1 {
				zap.L().Debug("Workspace pod to be deleted", zap.Any("pod", podList.Items[0]))
			}
		}
	}

	if err := k8sC.CoreV1().ConfigMaps(ns).Delete(ctx, getK8sRscName(ws), metav1.DeleteOptions{}); err != nil {
		if k8serr.IsNotFound(err) {
			zap.S().Debugf("Owner configmap of Workspace is already deleted. Nothing to be done: %s", ws.Metadata.Name)
			return nil
		}
		return err
	}

	return nil
}

func (c *Controller) setPersistentVolumeClaim(ctx context.Context, ws *cordiumv1.Workspace) error {

	storageLimit := func() int64 {

		if ws.Status.Limit != nil && ws.Status.Limit.Storage != nil &&
			ws.Status.Limit.Storage.Megabytes > 0 &&
			ws.Status.Limit.Storage.Megabytes < 5000000 {
			return int64(ws.Status.Limit.Storage.Megabytes)
		}

		if ldflags.IsDev() {
			return 50 * 1000
		}

		return 10 * 1000
	}()

	storageReq := getResourceQuantity(fmt.Sprintf("%dMi", int64((storageLimit))))

	zap.L().Debug("Setting PVC", zap.String("name", ws.Metadata.Name), zap.Int64("sizeMB", storageLimit))

	if _, err := c.k8sC.CoreV1().PersistentVolumeClaims(ns).Create(ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.getPVCName(ws),
			Namespace: ns,
		},
		Spec: corev1.PersistentVolumeClaimSpec{

			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": *storageReq,
				},
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			StorageClassName: func() *string {
				if val := c.getStorageClassName(ctx, ws); val != "" {
					return utils_types.StrToPtr(val)
				}
				return nil
			}(),
			DataSource: func() *corev1.TypedLocalObjectReference {
				if ws.Status.IsBuild {
					return nil
				}
				if !ws.Status.IsEphemeral && ws.Status.SuccessfulRuns > 0 {
					return nil
				}

				tmpl, err := c.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
					Uid: ws.Status.TemplateRef.Uid,
				})
				if err != nil {
					return nil
				}

				if tmpl.Status.BuildInfo == nil || tmpl.Status.BuildInfo.CurrentReadyBuildID == "" {
					return nil
				}

				return &corev1.TypedLocalObjectReference{
					APIGroup: utils_types.StrToPtr("snapshot.storage.k8s.io"),
					Kind:     "VolumeSnapshot",
					Name:     c.getTemplateBuildName(tmpl),
				}
			}(),
		},
	}, metav1.CreateOptions{}); err != nil {
		if !k8serr.IsAlreadyExists(err) {
			zap.L().Debug("No need to recreate Workspace PVC. Already exists...",
				zap.String("wsName", ws.Metadata.Name))
			return err
		}
	}

	return nil
}

func (c *Controller) getPVCName(ws *cordiumv1.Workspace) string {
	return fmt.Sprintf("ws-%s", ws.Metadata.Uid)
}

func (c *Controller) getTemplateBuildName(tmpl *cordiumv1.Template) string {
	return fmt.Sprintf("%s-%s",
		tmpl.Metadata.Uid,
		tmpl.Status.BuildInfo.CurrentReadyBuildID)
}

func (c *Controller) getStorageClassName(ctx context.Context, ws *cordiumv1.Workspace) string {

	cc, err := c.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return ""
	}

	if cc.Spec.Workspace != nil && cc.Spec.Workspace.Storage != nil &&
		cc.Spec.Workspace.Storage.StorageClass != nil &&
		len(cc.Spec.Workspace.Storage.StorageClass.Rules) > 0 {

		for _, rule := range cc.Spec.Workspace.Storage.StorageClass.Rules {
			if rule.StorageClass == "" {
				continue
			}

			inputMap := map[string]any{
				"workspace": pbutils.MustConvertToMap(ws),
			}

			cond, err := ovutils.ToCoreCondition(rule.Condition)
			if err != nil {
				continue
			}

			match, err := c.celEngine.EvalCondition(ctx, cond, inputMap)
			if err != nil {
				continue
			}
			if match {
				return rule.StorageClass
			}
		}
	}

	return ""
}

func (c *Controller) createTemplateSnapshot(ctx context.Context,
	ws *cordiumv1.Workspace, tmpl *cordiumv1.Template) error {

	if tmpl.Status.BuildInfo == nil || tmpl.Status.BuildInfo.CurrentReadyBuildID == "" {
		return nil
	}

	var volumeSnapshotClassName *string
	if cc, err := c.octeliumC.CordiumV1Utils().GetClusterConfig(ctx); err == nil {
		if cc.Spec.Workspace != nil && cc.Spec.Workspace.Storage != nil &&
			cc.Spec.Workspace.Storage.VolumeSnapshotClass != nil &&
			len(cc.Spec.Workspace.Storage.VolumeSnapshotClass.Rules) > 0 {

			reqCtxMap := map[string]any{
				"ctx": map[string]any{
					"workspace": pbutils.MustConvertToMap(ws),
					"template":  pbutils.MustConvertToMap(tmpl),
				},
			}

			func() {
				for _, rule := range cc.Spec.Workspace.Storage.VolumeSnapshotClass.Rules {

					cond, err := ovutils.ToCoreCondition(rule.Condition)
					if err != nil {
						continue
					}

					isMatched, err := c.celEngine.EvalCondition(ctx, cond, reqCtxMap)
					if err != nil {
						continue
					}

					if isMatched {
						volumeSnapshotClassName = utils_types.StrToPtr(rule.VolumeSnapshotClass)
						return
					}
				}
			}()

		}
	}

	snapshot := &v1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.getTemplateBuildName(tmpl),
			Namespace: ns,
			Labels: map[string]string{
				"octelium.com/template-uid":  tmpl.Metadata.Uid,
				"octelium.com/parent-ws-uid": ws.Metadata.Uid,
			},
		},
		Spec: v1.VolumeSnapshotSpec{
			VolumeSnapshotClassName: volumeSnapshotClassName,
			Source: v1.VolumeSnapshotSource{
				PersistentVolumeClaimName: utils_types.StrToPtr(c.getPVCName(ws)),
			},
		},
	}

	zap.L().Debug("Creating template volume snapshot", zap.Any("tmpl", tmpl))
	if snapshot, err := c.snapshotC.SnapshotV1().
		VolumeSnapshots(ns).Create(ctx, snapshot, metav1.CreateOptions{}); err != nil {
		switch {
		case k8serr.IsNotFound(err):
			zap.L().Debug("Could not create template volume snapshot. CRD likely unhandled",
				zap.Any("tmpl", tmpl), zap.Error(err))
		case k8serr.IsAlreadyExists(err):
			zap.L().Debug("Could not create template volume snapshot. Already exists",
				zap.Any("tmpl", tmpl), zap.Error(err))
		default:
			return err
		}
	} else {
		zap.L().Debug("Snapshot successfully created", zap.Any("snapshot", snapshot))
	}

	return nil
}
