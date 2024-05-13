/**
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package node

import (
	"context"
	"encoding/json"
	"os"

	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"
)

type NodeProvider interface {
	node.NodeProvider

	// Register configure node on first attempt
	Register(ctx context.Context, node *corev1.Node) error
}

var _ NodeProvider = &VirtualKubeletNode{}

type VirtualKubeletNode struct {
	NodeName     string
	NodeJsonPath string // poc only
}

func (v *VirtualKubeletNode) Register(_ context.Context, node *corev1.Node) error {
	bytes, err := os.ReadFile(v.NodeJsonPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, node)
}

func (v *VirtualKubeletNode) Ping(_ context.Context) error {
	// todo: do health check to base instance
	return nil
}

func (v VirtualKubeletNode) NotifyNodeStatus(_ context.Context, _ func(*corev1.Node)) {
	// todo: sync base status to k8s
}
