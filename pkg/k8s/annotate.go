// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/cilium/cilium/pkg/annotation"
	"github.com/cilium/cilium/pkg/cidr"
	"github.com/cilium/cilium/pkg/controller"
	"github.com/cilium/cilium/pkg/logging/logfields"
)

func updateNodeAnnotation(c kubernetes.Interface, nodeName string, encryptKey uint8, v4CIDR, v6CIDR *cidr.CIDR, v4HealthIP, v6HealthIP, v4IngressIP, v6IngressIP, v4CiliumHostIP, v6CiliumHostIP net.IP) error {
	annotations := map[string]string{}

	if v4CIDR != nil {
		annotations[annotation.V4CIDRName] = v4CIDR.String()
	}
	if v6CIDR != nil {
		annotations[annotation.V6CIDRName] = v6CIDR.String()
	}

	if v4HealthIP != nil {
		annotations[annotation.V4HealthName] = v4HealthIP.String()
	}
	if v6HealthIP != nil {
		annotations[annotation.V6HealthName] = v6HealthIP.String()
	}

	if v4IngressIP != nil {
		annotations[annotation.V4IngressName] = v4IngressIP.String()
	}
	if v6IngressIP != nil {
		annotations[annotation.V6IngressName] = v6IngressIP.String()
	}

	if v4CiliumHostIP != nil {
		annotations[annotation.CiliumHostIP] = v4CiliumHostIP.String()
	}

	if v6CiliumHostIP != nil {
		annotations[annotation.CiliumHostIPv6] = v6CiliumHostIP.String()
	}

	if encryptKey != 0 {
		annotations[annotation.CiliumEncryptionKey] = strconv.FormatUint(uint64(encryptKey), 10)
	}

	if len(annotations) == 0 {
		return nil
	}

	raw, err := json.Marshal(annotations)
	if err != nil {
		return err
	}
	patch := []byte(fmt.Sprintf(`{"metadata":{"annotations":%s}}`, raw))

	_, err = c.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.StrategicMergePatchType, patch, metav1.PatchOptions{}, "status")

	return err
}

// AnnotateNode writes v4 and v6 CIDRs and health IPs in the given k8s node name.
// In case of failure while updating the node, this function while spawn a go
// routine to retry the node update indefinitely.
func AnnotateNode(cs kubernetes.Interface, nodeName string, encryptKey uint8, v4CIDR, v6CIDR *cidr.CIDR, v4HealthIP, v6HealthIP, v4IngressIP, v6IngressIP, v4CiliumHostIP, v6CiliumHostIP net.IP) error {
	scopedLog := log.WithFields(logrus.Fields{
		logfields.NodeName:       nodeName,
		logfields.V4Prefix:       v4CIDR,
		logfields.V6Prefix:       v6CIDR,
		logfields.V4HealthIP:     v4HealthIP,
		logfields.V6HealthIP:     v6HealthIP,
		logfields.V4IngressIP:    v4IngressIP,
		logfields.V6IngressIP:    v6IngressIP,
		logfields.V4CiliumHostIP: v4CiliumHostIP,
		logfields.V6CiliumHostIP: v6CiliumHostIP,
		logfields.Key:            encryptKey,
	})
	scopedLog.Debug("Updating node annotations with node CIDRs")

	controller.NewManager().UpdateController("update-k8s-node-annotations",
		controller.ControllerParams{
			DoFunc: func(_ context.Context) error {
				err := updateNodeAnnotation(cs, nodeName, encryptKey, v4CIDR, v6CIDR, v4HealthIP, v6HealthIP, v4IngressIP, v6IngressIP, v4CiliumHostIP, v6CiliumHostIP)
				if err != nil {
					scopedLog.WithFields(logrus.Fields{}).WithError(err).Warn("Unable to patch node resource with annotation")
				}
				return err
			},
		})

	return nil
}
