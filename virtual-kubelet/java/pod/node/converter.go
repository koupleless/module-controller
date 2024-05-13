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
	"strconv"

	"github.com/koupleless/module-controller/virtual-kubelet/common"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Converter struct {
	common.PodUtils
}

// Convert a pod contains a koupleless jvm base to a virtual node inside kubernetes cluster
func (c Converter) Convert(pod *v1.Pod) *v1.Node {

	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: VIRTUAL_NODE_NAME_PREFIX + pod.Namespace + "-" + pod.Name,
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{
				{
					Key:   TAINT_KUBE_NODE_READY,
					Value: strconv.FormatBool(c.IsPodReady(pod)),
				},
			},
		},
		Status: v1.NodeStatus{
			Capacity: map[v1.ResourceName]resource.Quantity{
				v1.ResourceCPU:    resource.MustParse(pod.Labels[LABEL_VIRTUAL_KUBE_NODE_CPU]),
				v1.ResourceMemory: resource.MustParse(pod.Labels[LABEL_VIRTUAL_KUBE_NODE_MEM]),
			},
			Phase: v1.NodeRunning,
			Addresses: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: pod.Status.PodIP,
				},
			},
		},
	}
}
