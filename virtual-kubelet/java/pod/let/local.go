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
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
)

// LocalPodCache vkubelet should hold a cached and observed pod information in local to properly diff and sync with the remote
type LocalPodCache struct {
	*sync.RWMutex
	cache             map[string]*corev1.Pod
	podNameToCacheKey map[string]string
}

func (c *LocalPodCache) GetByPodName(podName string) *corev1.Pod {
	c.RLocker()
	defer c.RUnlock()
	return c.cache[c.podNameToCacheKey[podName]]
}

func (c *LocalPodCache) Get(biz, version string) *corev1.Pod {
	c.RLocker()
	defer c.RUnlock()
	return c.cache[biz+"/"+version]
}

func (c *LocalPodCache) Put(biz, version string, pod *corev1.Pod) error {
	c.Lock()
	defer c.Unlock()

	cached := c.cache[biz+"/"+version]
	if cached != nil {
		return fmt.Errorf("biz %s already exists", biz)
	}
	c.cache[biz+"/"+version] = pod
	c.podNameToCacheKey[pod.Name] = biz + "/" + version
	return nil
}

func (c *LocalPodCache) Remove(biz, version string) {
	c.Lock()
	defer c.Unlock()

	pod := c.cache[biz+"/"+version]
	if pod != nil {
		delete(c.cache, biz+"/"+version)
		delete(c.podNameToCacheKey, pod.Name)
	}
}
