package etcd

import (
	"context"
	"os"
	"time"

	"github.com/k3s-io/k3s/pkg/util"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

func registerMetadataHandlers(ctx context.Context, etcd *ETCD) {
	nodes := etcd.config.Runtime.Core.Core().V1().Node()
	h := &metadataHandler{
		etcd:           etcd,
		nodeController: nodes,
		ctx:            ctx,
	}

	logrus.Infof("Starting managed etcd node metadata controller")
	nodes.OnChange(ctx, "managed-etcd-metadata-controller", h.sync)
}

type metadataHandler struct {
	etcd           *ETCD
	nodeController controllerv1.NodeController
	ctx            context.Context
}

func (m *metadataHandler) sync(key string, node *v1.Node) (*v1.Node, error) {
	if node == nil {
		return nil, nil
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		logrus.Debug("waiting for node name to be assigned for managed etcd node metadata controller")
		m.nodeController.EnqueueAfter(key, 5*time.Second)
		return node, nil
	}

	if key == nodeName {
		return m.handleSelf(node)
	}

	return node, nil
}

func (m *metadataHandler) handleSelf(node *v1.Node) (*v1.Node, error) {
	if m.etcd.config.DisableETCD {
		if node.Annotations[NodeNameAnnotation] == "" &&
			node.Annotations[NodeAddressAnnotation] == "" &&
			node.Labels[util.ETCDRoleLabelKey] == "" {
			return node, nil
		}

		node = node.DeepCopy()
		if node.Annotations == nil {
			node.Annotations = map[string]string{}
		}
		if node.Labels == nil {
			node.Labels = map[string]string{}
		}

		delete(node.Annotations, NodeNameAnnotation)
		delete(node.Annotations, NodeAddressAnnotation)
		delete(node.Labels, util.ETCDRoleLabelKey)

		return m.nodeController.Update(node)
	}

	if node.Annotations[NodeNameAnnotation] == m.etcd.name &&
		node.Annotations[NodeAddressAnnotation] == m.etcd.address &&
		node.Labels[util.ETCDRoleLabelKey] == "true" {
		return node, nil
	}

	node = node.DeepCopy()
	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}

	node.Annotations[NodeNameAnnotation] = m.etcd.name
	node.Annotations[NodeAddressAnnotation] = m.etcd.address
	node.Labels[util.ETCDRoleLabelKey] = "true"

	return m.nodeController.Update(node)
}
