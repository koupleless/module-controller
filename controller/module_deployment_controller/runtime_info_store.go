package module_deployment_controller

import (
	"fmt"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// RuntimeInfoStore provides in-memory runtime information storage with thread-safe access.
// Key components:
// - Deployment maps: Stores peer and non-peer deployments
// - Node map: Stores node information
// - Label maps: Tracks relationships between deployments, nodes and their labels
// - Thread safety: Uses RWMutex for concurrent access

// Main operations:
// 1. Deployment Management
//    - PutDeployment: Adds/updates deployment and its label selectors
//    - DeleteDeployment: Removes deployment and its label mappings
//
// 2. Node Management
//    - PutNode: Adds/updates node and its labels, tracks label changes
//    - DeleteNode: Removes node and updates label mappings
//
// 3. Label Management
//    - updateDeploymentSelectorLabelMap: Maintains deployment->label mappings
//    - updateNodeLabelMap: Maintains node->label mappings
//    - Tracks both equality and non-equality label requirements
//
// 4. Matching Logic
//    - GetRelatedDeploymentsByNode: Finds deployments matching a node's labels
//    - GetMatchedNodeNum: Counts nodes matching a deployment's label selectors
//    - isNodeFitDep: Checks if node labels satisfy deployment requirements
//
// 5. Helper Functions
//    - Label set operations: union, intersection, subtraction
//    - Label diff calculation
//    - Deployment label extraction

// RuntimeInfoStore provide the in memory runtime information.
type RuntimeInfoStore struct {
	sync.RWMutex
	peerDeploymentMap            map[string]appsv1.Deployment
	nonPeerDeploymentMap         map[string]appsv1.Deployment
	nodeMap                      map[string]*corev1.Node
	depKeyToEqLabels             map[string]map[string]map[string]interface{}
	depKeyToNeLabels             map[string]map[string]map[string]interface{}
	labelKeyToValueToNodeKeysMap map[string]map[string]map[string]interface{}
}

func NewRuntimeInfoStore() *RuntimeInfoStore {
	return &RuntimeInfoStore{
		RWMutex:                      sync.RWMutex{},
		peerDeploymentMap:            make(map[string]appsv1.Deployment),
		nonPeerDeploymentMap:         make(map[string]appsv1.Deployment),
		nodeMap:                      make(map[string]*corev1.Node),
		depKeyToEqLabels:             make(map[string]map[string]map[string]interface{}),
		depKeyToNeLabels:             make(map[string]map[string]map[string]interface{}),
		labelKeyToValueToNodeKeysMap: make(map[string]map[string]map[string]interface{}),
	}
}

// getResourceKey generates a unique key for a k8s resource by combining namespace and name
func (r *RuntimeInfoStore) getResourceKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// PutDeployment adds or updates a deployment in the store
// - Stores deployment in peerDeploymentMap
// - Updates label selector mappings
func (r *RuntimeInfoStore) PutDeployment(deployment appsv1.Deployment) {
	r.Lock()
	defer r.Unlock()
	depKey := r.getResourceKey(deployment.Namespace, deployment.Name)
	r.peerDeploymentMap[depKey] = deployment
	r.updateDeploymentSelectorLabelMap(depKey, deployment.DeepCopy())
}

// DeleteDeployment removes a deployment and its label mappings from the store
// - Removes from peerDeploymentMap
// - Cleans up label selector mappings
func (r *RuntimeInfoStore) DeleteDeployment(deployment appsv1.Deployment) {
	r.Lock()
	defer r.Unlock()

	depKey := r.getResourceKey(deployment.Namespace, deployment.Name)
	// check has old version
	_, has := r.peerDeploymentMap[depKey]
	if has {
		delete(r.peerDeploymentMap, depKey)
		delete(r.depKeyToEqLabels, depKey)
		delete(r.depKeyToNeLabels, depKey)
	}
}

// PutNode adds or updates a node in the store
// - Stores node in nodeMap
// - Tracks label changes between old and new node
// - Updates label mappings
// Returns true if labels changed
func (r *RuntimeInfoStore) PutNode(node *corev1.Node) (labelChanged bool) {
	r.Lock()
	defer r.Unlock()
	nodeKey := r.getResourceKey(node.Namespace, node.Name)
	// check has old version
	var sub, plus labels.Set

	var oldLabels labels.Set

	oldNode, has := r.nodeMap[nodeKey]
	if has && oldNode != nil {
		oldLabels = oldNode.DeepCopy().Labels
	}
	r.nodeMap[nodeKey] = node
	sub, plus = getLabelDiff(oldLabels, node.DeepCopy().Labels)
	r.updateNodeLabelMap(nodeKey, sub, plus)
	return len(sub) != 0 || len(plus) != 0
}

// DeleteNode removes a node and its label mappings from the store
// - Removes from nodeMap
// - Updates label mappings
func (r *RuntimeInfoStore) DeleteNode(node *corev1.Node) {
	r.Lock()
	defer r.Unlock()

	nodeKey := r.getResourceKey(node.Namespace, node.Name)
	// check has old version
	var oldLabels labels.Set

	oldNode, has := r.nodeMap[nodeKey]
	if has && oldNode != nil {
		oldLabels = oldNode.DeepCopy().Labels
	}
	delete(r.nodeMap, nodeKey)
	r.updateNodeLabelMap(nodeKey, oldLabels, nil)
}

// updateDeploymentSelectorLabelMap maintains deployment->label mappings
// - Extracts equality and non-equality label requirements
// - Updates internal maps
func (r *RuntimeInfoStore) updateDeploymentSelectorLabelMap(depKey string, newDep *appsv1.Deployment) {
	newEqLabels, newNeLabels := getDeploymentMatchLabels(*newDep)

	r.depKeyToEqLabels[depKey] = newEqLabels
	r.depKeyToNeLabels[depKey] = newNeLabels
}

// updateNodeLabelMap maintains node->label mappings
// - Removes old label mappings (sub)
// - Adds new label mappings (plus)
func (r *RuntimeInfoStore) updateNodeLabelMap(nodeKey string, sub, plus labels.Set) {
	// delete map item
	for key, value := range sub {
		valueMap, has := r.labelKeyToValueToNodeKeysMap[key]
		if !has {
			continue
		}
		keyMap, has := valueMap[value]
		if !has {
			continue
		}
		delete(keyMap, nodeKey)
		valueMap[value] = keyMap
		r.labelKeyToValueToNodeKeysMap[key] = valueMap
	}

	// add map item
	for key, value := range plus {
		valueMap, has := r.labelKeyToValueToNodeKeysMap[key]
		if !has {
			valueMap = make(map[string]map[string]interface{})
		}
		keyMap, has := valueMap[value]
		if !has {
			keyMap = make(map[string]interface{})
		}
		keyMap[nodeKey] = nil
		valueMap[value] = keyMap
		r.labelKeyToValueToNodeKeysMap[key] = valueMap
	}
}

// GetRelatedDeploymentsByNode finds deployments matching a node's labels
// - Checks each deployment against node labels
// - Returns matching deployments
func (r *RuntimeInfoStore) GetRelatedDeploymentsByNode(node *corev1.Node) []appsv1.Deployment {
	r.Lock()
	defer r.Unlock()

	matchedDeployments := make([]appsv1.Deployment, 0)

	for depKey := range r.peerDeploymentMap {
		if r.isNodeFitDep(node.Labels, depKey) {
			matchedDeployments = append(matchedDeployments, r.peerDeploymentMap[depKey])
		}
	}

	return matchedDeployments
}

// isNodeFitDep checks if node labels satisfy deployment requirements
// - Verifies equality label matches
// - Verifies non-equality label matches
func (r *RuntimeInfoStore) isNodeFitDep(nodeLabels labels.Set, depKey string) bool {
	eqLabels := r.depKeyToEqLabels[depKey]
	neLabels := r.depKeyToNeLabels[depKey]
	for key, labelValues := range eqLabels {
		value, has := nodeLabels[key]
		if !has {
			return false
		}
		_, has = labelValues[value]
		if !has {
			return false
		}
	}
	for key, labelValues := range neLabels {
		value, has := nodeLabels[key]
		if !has {
			continue
		}
		_, has = labelValues[value]
		if has {
			return false
		}
	}
	return true
}

// GetMatchedNodeNum counts nodes matching a deployment's label selectors
// - Checks equality and non-equality label requirements
// - Returns count of matching nodes
func (r *RuntimeInfoStore) GetMatchedNodeNum(deployment appsv1.Deployment) int {
	r.Lock()
	defer r.Unlock()

	matchedNodeKeys := make(map[string]interface{})

	eqLabels, neLabels := getDeploymentMatchLabels(deployment)

	for key, labelValues := range eqLabels {
		valueMap, has := r.labelKeyToValueToNodeKeysMap[key]
		if !has {
			// no label matched
			return 0
		}
		validNodeKeys := make(map[string]interface{})
		for value := range labelValues {
			validNodeKeys = union(validNodeKeys, valueMap[value])
		}
		if len(matchedNodeKeys) == 0 {
			matchedNodeKeys = validNodeKeys
		} else {
			matchedNodeKeys = intersection(matchedNodeKeys, validNodeKeys)
		}
		if len(matchedNodeKeys) == 0 {
			// no matched deployments, return directly
			return 0
		}
	}

	for key, labelValues := range neLabels {
		valueMap := r.labelKeyToValueToNodeKeysMap[key]
		if valueMap == nil {
			valueMap = make(map[string]map[string]interface{})
		}
		invalidNodeKeys := make(map[string]interface{})
		for value := range labelValues {
			invalidNodeKeys = union(invalidNodeKeys, valueMap[value])
		}
		matchedNodeKeys = subKeys(matchedNodeKeys, invalidNodeKeys)
		if len(matchedNodeKeys) == 0 {
			// no matched deployments, return directly
			return 0
		}
	}

	nodeNum := 0
	for key := range matchedNodeKeys {
		node := r.nodeMap[key]
		if node == nil {
			continue
		}
		nodeNum++
	}
	return nodeNum
}

// getLabelDiff calculates differences between old and new label sets
// Returns:
// - sub: labels removed
// - plus: labels added
func getLabelDiff(oldLabels, newLabels labels.Set) (sub labels.Set, plus labels.Set) {
	sub = labels.Set{}
	plus = labels.Set{}

	for key, value := range oldLabels {
		newValue, has := newLabels[key]
		if !has || newValue != value {
			sub[key] = value
		} else {
			delete(newLabels, key)
		}
	}

	for key, value := range newLabels {
		plus[key] = value
	}

	return
}

// subKeys returns keys in keyList1 that are not in keyList2
func subKeys(keyList1, keyList2 map[string]interface{}) map[string]interface{} {
	sub := make(map[string]interface{})
	for key := range keyList1 {
		_, has := keyList2[key]
		if has {
			continue
		}
		sub[key] = keyList1[key]
	}
	return sub
}

// unionLabels adds labels2 entries to srcMap
func unionLabels(srcMap map[string]map[string]interface{}, labels2 labels.Set) {
	for key, value := range labels2 {
		valueMap := srcMap[key]
		if valueMap == nil {
			valueMap = make(map[string]interface{})
		}
		valueMap[value] = nil
		srcMap[key] = valueMap
	}
}

// intersection returns keys present in both maps
func intersection(src, target map[string]interface{}) map[string]interface{} {
	ret := map[string]interface{}{}

	for key := range target {
		_, has := src[key]
		if !has {
			continue
		}
		ret[key] = nil
	}
	return ret
}

// union returns keys present in either map
func union(src, target map[string]interface{}) map[string]interface{} {
	ret := map[string]interface{}{}

	for key := range target {
		ret[key] = nil
	}
	for key := range src {
		ret[key] = nil
	}
	return ret
}

// getDeploymentMatchLabels extracts label requirements from deployment spec
// Returns:
// - eqLabels: equality requirements (In/Exists)
// - neLabels: non-equality requirements (NotIn/DoesNotExist)
func getDeploymentMatchLabels(dep appsv1.Deployment) (eqLabels, neLabels map[string]map[string]interface{}) {

	eqLabels = make(map[string]map[string]interface{})
	neLabels = make(map[string]map[string]interface{})

	affinity := dep.Spec.Template.Spec.Affinity
	if affinity != nil && affinity.NodeAffinity != nil {
		nodeAffinity := affinity.NodeAffinity
		for _, term := range nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			for _, expressions := range term.MatchExpressions {
				if expressions.Operator == corev1.NodeSelectorOpIn || expressions.Operator == corev1.NodeSelectorOpExists {
					eqValues := make(map[string]interface{})
					for _, value := range expressions.Values {
						eqValues[value] = nil
					}
					oldValues, has := eqLabels[expressions.Key]
					if !has {
						eqLabels[expressions.Key] = eqValues
					} else {
						eqLabels[expressions.Key] = intersection(oldValues, eqValues)
					}
				} else if expressions.Operator == corev1.NodeSelectorOpNotIn || expressions.Operator == corev1.NodeSelectorOpDoesNotExist {
					neValues := make(map[string]interface{})
					for _, value := range expressions.Values {
						neValues[value] = nil
					}
					oldValues, has := neLabels[expressions.Key]
					if !has {
						neLabels[expressions.Key] = neValues
					} else {
						neLabels[expressions.Key] = union(oldValues, neValues)
					}
				}
			}
		}
	}

	unionLabels(eqLabels, dep.Spec.Template.Spec.NodeSelector)

	return
}
