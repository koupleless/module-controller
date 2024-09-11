package module_deployment_controller

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sync"
)

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

func (r *RuntimeInfoStore) getResourceKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

func (r *RuntimeInfoStore) PutDeployment(deployment appsv1.Deployment) {
	r.Lock()
	defer r.Unlock()
	depKey := r.getResourceKey(deployment.Namespace, deployment.Name)
	r.peerDeploymentMap[depKey] = deployment
	r.updateDeploymentSelectorLabelMap(depKey, deployment.DeepCopy())
}

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

func (r *RuntimeInfoStore) updateDeploymentSelectorLabelMap(depKey string, newDep *appsv1.Deployment) {
	newEqLabels, newNeLabels := getDeploymentMatchLabels(*newDep)

	r.depKeyToEqLabels[depKey] = newEqLabels
	r.depKeyToNeLabels[depKey] = newNeLabels
}

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
					eqLabels[expressions.Key] = intersection(eqLabels[expressions.Key], eqValues)
				} else if expressions.Operator == corev1.NodeSelectorOpNotIn || expressions.Operator == corev1.NodeSelectorOpDoesNotExist {
					neValues := make(map[string]interface{})
					for _, value := range expressions.Values {
						neValues[value] = nil
					}
					neLabels[expressions.Key] = union(neLabels[expressions.Key], neValues)
				}
			}
		}
	}

	unionLabels(eqLabels, dep.Spec.Template.Spec.NodeSelector)

	return
}
