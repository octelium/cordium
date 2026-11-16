package scenario

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/octelium/octelium/cluster/e2e/scenario"
	"github.com/pkg/errors"
	k8scorev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func stepClusterDNS(ctx context.Context, r *scenario.Runner) error {
	if r.Scenario.Domain != "localhost" {
		return nil
	}

	k8sC, err := r.K8sC()
	if err != nil {
		return err
	}

	if err := configureClusterDNS(ctx, k8sC, r.State.ExternalIP); err != nil {
		return err
	}

	return r.Bash(ctx, `
kubectl -n kube-system rollout restart deployment/coredns
kubectl -n kube-system rollout status deployment/coredns --timeout=120s
`)
}

func configureClusterDNS(ctx context.Context, k8sC kubernetes.Interface, address string) error {
	addr, err := netip.ParseAddr(address)
	if err != nil || !addr.Is4() || !addr.IsGlobalUnicast() {
		return errors.Errorf("The e2e DNS address %q must be a non-loopback unicast IPv4 address", address)
	}

	config := fmt.Sprintf(`localhost:53 {
    errors
    cache 5
    template IN A {
        answer "{{ .Name }} 5 IN A %s"
    }
    template IN ANY {
        rcode NOERROR
    }
}
`, addr)

	configMaps := k8sC.CoreV1().ConfigMaps("kube-system")
	cm, err := configMaps.Get(ctx, "coredns-custom", k8smetav1.GetOptions{})
	if k8serr.IsNotFound(err) {
		_, err = configMaps.Create(ctx, &k8scorev1.ConfigMap{
			ObjectMeta: k8smetav1.ObjectMeta{
				Name:      "coredns-custom",
				Namespace: "kube-system",
			},
			Data: map[string]string{"cordium.server": config},
		}, k8smetav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	if cm.Data["cordium.server"] == config {
		return nil
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data["cordium.server"] = config

	_, err = configMaps.Update(ctx, cm, k8smetav1.UpdateOptions{})
	return err
}
