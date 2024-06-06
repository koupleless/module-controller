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

package let

import (
	"sync"

	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module-controller/virtual-kubelet/java/common"
	corev1 "k8s.io/api/core/v1"
)

// RuntimeInfoStore provide the in memory runtime information.
type RuntimeInfoStore struct {
	sync.RWMutex
	modelUtils common.ModelUtils

	podKeyToPod                map[string]*corev1.Pod
	podKeyToBizModels          map[string][]*ark.BizModel
	bizIdentityToRelatedPodKey map[string]string
}

func NewRuntimeInfoStore() *RuntimeInfoStore {
	return &RuntimeInfoStore{
		RWMutex:                    sync.RWMutex{},
		modelUtils:                 common.ModelUtils{},
		podKeyToPod:                make(map[string]*corev1.Pod),
		podKeyToBizModels:          make(map[string][]*ark.BizModel),
		bizIdentityToRelatedPodKey: make(map[string]string),
	}
}

func (r *RuntimeInfoStore) getPodKey(pod *corev1.Pod) string {
	return pod.Namespace + "/" + pod.Name
}

func (r *RuntimeInfoStore) getBizIdentity(biz *ark.BizModel) string {
	return r.modelUtils.GetBizIdentityFromBizModel(biz)
}

func (r *RuntimeInfoStore) PutPod(pod *corev1.Pod) {
	r.Lock()
	defer r.Unlock()

	podKey := r.getPodKey(pod)

	// todo: create or update
	r.podKeyToPod[podKey] = pod
	r.podKeyToBizModels[podKey] = r.modelUtils.GetBizModelsFromCoreV1Pod(pod)
	for _, bizModels := range r.podKeyToBizModels {
		for _, bizModel := range bizModels {
			// the biz identity naming convention should guarantee there would be no potential conflict
			// for now we use bizName:version as the identity, the constraint cannot be applied.
			// further mechnanism to avoid this is required, for now we just leave the risk here.
			r.bizIdentityToRelatedPodKey[r.getBizIdentity(bizModel)] = podKey
		}
	}
}

func (r *RuntimeInfoStore) GetRelatedPodKeyByBizIdentity(bizIdentity string) string {
	r.RLock()
	defer r.RUnlock()
	return r.bizIdentityToRelatedPodKey[bizIdentity]
}

func (r *RuntimeInfoStore) GetBizModel(bizIdentity string) *ark.BizModel {
	r.RLock()
	defer r.RUnlock()
	podKey := r.GetRelatedPodKeyByBizIdentity(bizIdentity)
	if podKey == "" {
		return nil
	}
	for _, bizModel := range r.podKeyToBizModels[podKey] {
		if r.getBizIdentity(bizModel) == bizIdentity {
			return bizModel
		}
	}
	return nil
}

func (r *RuntimeInfoStore) GetPodByKey(podKey string) *corev1.Pod {
	r.RLock()
	defer r.RUnlock()
	return r.podKeyToPod[podKey]
}

func (r *RuntimeInfoStore) GetRelatedBizModels(podKey string) []*ark.BizModel {
	r.RLock()
	defer r.RUnlock()
	return r.podKeyToBizModels[podKey]
}

func (r *RuntimeInfoStore) GetPods() []*corev1.Pod {
	r.RLock()
	defer r.RUnlock()
	ret := make([]*corev1.Pod, 0, len(r.podKeyToPod))
	for _, pod := range r.podKeyToPod {
		ret = append(ret, pod)
	}
	return ret
}
