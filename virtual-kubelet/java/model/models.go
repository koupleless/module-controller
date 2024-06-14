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

package model

type BuildVirtualNodeConfig struct {
	// NodeIP is the ip of the node
	NodeIP string `json:"nodeIP"`

	// TechStack is the underlying tech stack of runtime
	TechStack string `json:"techStack"`

	// Version is the version of ths underlying runtime
	Version string `json:"version"`

	// VPodCapacity limits the number of vPods that can be created on this node
	VPodCapacity int `json:"vPodCapacity"`
}
