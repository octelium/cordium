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

package genesis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/k8sretry"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/pkg/errors"
	corev1k8s "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const targetClusterUID = "octelium.com/genesis-cluster-uid"
const targetDomain = "octelium.com/genesis-domain"
const targetRegionUID = "octelium.com/genesis-region-uid"
const targetRegionName = "octelium.com/genesis-region-name"
const targetNamespaceUID = "octelium.com/genesis-namespace-uid"
const targetStorageHash = "octelium.com/genesis-storage-target"

func sameDomain(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}

func checkTargetDomain(requested, actual string) error {
	if actual == "" || (requested != "" && !sameDomain(requested, actual)) {
		return errors.Errorf("The requested domain %q does not match the Cluster domain %q", requested, actual)
	}
	return nil
}

func checkTargetAnnotations(actual, expected map[string]string) error {
	bound := false
	for _, key := range []string{targetClusterUID, targetDomain, targetRegionUID, targetRegionName, targetNamespaceUID} {
		_, ok := actual[key]
		bound = bound || ok
	}
	if !bound {
		return nil
	}
	for _, key := range []string{targetClusterUID, targetDomain, targetRegionUID, targetRegionName, targetNamespaceUID} {
		value := actual[key]
		matches := value != "" && value == expected[key]
		if key == targetDomain {
			matches = value != "" && sameDomain(value, expected[key])
		}
		if !matches {
			return errors.Errorf("The Kubernetes installation identity %s does not match the target Cluster/Region", key)
		}
	}
	return nil
}

func storageTargetHash(secret *corev1k8s.Secret) string {
	data := map[string]string{"host": strings.ToLower(strings.TrimSuffix(string(secret.Data["host"]), ".")), "port": string(secret.Data["port"]), "database": string(secret.Data["database"])}
	if secret.Name == "octelium-postgres" {
		if data["port"] == "" || data["port"] == "0" {
			data["port"] = "5432"
		}
		if data["database"] == "" {
			data["database"] = string(secret.Data["username"])
			if data["database"] == "" {
				data["database"] = "postgres"
			}
		}
	} else {
		if data["port"] == "" || data["port"] == "0" {
			data["port"] = "6379"
		}
		if data["database"] == "" {
			data["database"] = "0"
		}
	}
	encoded, _ := json.Marshal(data)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (g *Genesis) getK8sSecret(ctx context.Context, namespace, name string) (*corev1k8s.Secret, error) {
	return k8sretry.Read(ctx, func(ctx context.Context) (*corev1k8s.Secret, error) {
		return g.k8sC.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	})
}

func (g *Genesis) getK8sConfigMap(ctx context.Context, namespace, name string) (*corev1k8s.ConfigMap, error) {
	return k8sretry.Read(ctx, func(ctx context.Context) (*corev1k8s.ConfigMap, error) {
		return g.k8sC.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	})
}

func (g *Genesis) prepareRegion(ctx context.Context, cc *corev1.ClusterConfig) (*corev1.Region, error) {
	if err := checkTargetDomain(os.Getenv("OCTELIUM_DOMAIN"), cc.GetStatus().GetDomain()); err != nil {
		return nil, err
	}
	cm, err := g.getK8sConfigMap(ctx, vutils.K8sNS, "octelium-installation")
	if err != nil && !k8serr.IsNotFound(err) {
		return nil, err
	}
	var annotations map[string]string
	if cm != nil {
		annotations = cm.Annotations
	}
	name := annotations[targetRegionName]
	var bootstrapUID string
	bootstrap, err := g.getK8sSecret(ctx, vutils.K8sNS, "octelium-init")
	if err == nil {
		if domain := string(bootstrap.Data["domain"]); domain != "" {
			if err := checkTargetDomain(domain, cc.Status.Domain); err != nil {
				return nil, err
			}
		}
		if data, ok := bootstrap.Data["region"]; ok {
			stored := &corev1.Region{}
			if err := pbutils.Unmarshal(data, stored); err != nil {
				return nil, errors.Wrap(err, "Invalid bootstrap Region")
			}
			storedName := stored.GetMetadata().GetName()
			if storedName == "" {
				storedName = "default"
			}
			if name != "" && name != storedName {
				return nil, errors.New("The bootstrap Region does not match the Kubernetes installation identity")
			}
			name, bootstrapUID = storedName, stored.GetMetadata().GetUid()
		}
	} else if !k8serr.IsNotFound(err) {
		return nil, err
	}
	requested := os.Getenv("OCTELIUM_REGION_NAME")
	if name == "" {
		name = requested
		if name == "" {
			name = "default"
		}
	} else if requested != "" && requested != "default" && requested != name {
		return nil, errors.Errorf("The requested Region %q does not match the Kubernetes target Region %q", requested, name)
	}
	region, err := g.octeliumCInit.CoreC().GetRegion(ctx, &rmetav1.GetOptions{Name: name})
	if err != nil {
		return nil, err
	}
	if region.GetMetadata().GetName() != name || (bootstrapUID != "" && bootstrapUID != region.GetMetadata().GetUid()) {
		return nil, errors.New("The bootstrap Region identity does not match the live Region")
	}
	if err := g.validateRegionTarget(ctx, cc, region); err != nil {
		return nil, err
	}
	os.Setenv("OCTELIUM_REGION_NAME", name)
	return region, nil
}

func (g *Genesis) validateRegionTarget(ctx context.Context, cc *corev1.ClusterConfig, region *corev1.Region) error {
	cm, err := g.getK8sConfigMap(ctx, vutils.K8sNS, "octelium-installation")
	if err != nil && !k8serr.IsNotFound(err) {
		return err
	}
	var annotations map[string]string
	if cm != nil {
		annotations = cm.Annotations
	}
	expected := map[string]string{targetClusterUID: cc.GetMetadata().GetUid(), targetDomain: cc.Status.Domain, targetRegionUID: region.GetMetadata().GetUid(), targetRegionName: region.GetMetadata().GetName()}
	loadNamespace := func() error {
		if expected[targetNamespaceUID] != "" {
			return nil
		}
		namespace, err := k8sretry.Read(ctx, func(ctx context.Context) (*corev1k8s.Namespace, error) {
			return g.k8sC.CoreV1().Namespaces().Get(ctx, vutils.K8sNS, metav1.GetOptions{})
		})
		if err != nil {
			return err
		}
		if namespace.UID == "" || namespace.DeletionTimestamp != nil {
			return errors.New("The target Kubernetes namespace has no live identity")
		}
		expected[targetNamespaceUID] = string(namespace.UID)
		return nil
	}
	for key, env := range map[string]string{targetClusterUID: "OCTELIUM_GENESIS_CLUSTER_UID", targetRegionUID: "OCTELIUM_GENESIS_REGION_UID", targetNamespaceUID: "OCTELIUM_GENESIS_NAMESPACE_UID"} {
		if value := os.Getenv(env); value != "" {
			if key == targetNamespaceUID {
				if err := loadNamespace(); err != nil {
					return err
				}
			}
			if value != expected[key] {
				return errors.Errorf("The submitted genesis identity %s does not match the target Cluster/Region", key)
			}
		}
	}
	validate := func(annotations map[string]string) error {
		if _, ok := annotations[targetNamespaceUID]; ok {
			if err := loadNamespace(); err != nil {
				return err
			}
		}
		return checkTargetAnnotations(annotations, expected)
	}
	if err := validate(annotations); err != nil {
		return err
	}
	for _, name := range []string{"octelium-postgres", "octelium-redis"} {
		secret, err := g.getK8sSecret(ctx, vutils.K8sNS, name)
		if k8serr.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
		if err := validate(secret.Annotations); err != nil {
			return err
		}
		if hash, ok := secret.Annotations[targetStorageHash]; ok && hash != storageTargetHash(secret) {
			return errors.Errorf("The %s storage target changed after installation", name)
		}
	}
	return nil
}
