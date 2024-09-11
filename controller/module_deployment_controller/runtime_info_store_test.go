package module_deployment_controller

import (
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"testing"
)

func TestUnionLabels(t *testing.T) {
	srcMap := map[string]map[string]interface{}{}
	unionLabels(srcMap, labels.Set{
		"foo": "bar",
	})
	_, exist := srcMap["foo"]["bar"]
	assert.True(t, exist)
}

func TestGetDeploymentMatchLabels(t *testing.T) {
	eqLabels, neLabels := getDeploymentMatchLabels(appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"foo": "bar1"},
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "foo",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"bar2"},
											},
											{
												Key:      "foo",
												Operator: v1.NodeSelectorOpExists,
												Values:   []string{"bar3"},
											},
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpNotIn,
												Values:   []string{"bar"},
											},
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpDoesNotExist,
												Values:   []string{"bar2"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	assert.Equal(t, len(eqLabels), 1)
	assert.Equal(t, len(eqLabels["foo"]), 1)
	assert.Equal(t, len(neLabels), 1)
	assert.Equal(t, len(neLabels["baz"]), 2)
}

func TestSubKeys(t *testing.T) {
	ret := subKeys(map[string]interface{}{
		"key1": nil,
		"key2": nil,
	}, map[string]interface{}{
		"key2": nil,
		"key3": nil,
	})
	assert.Equal(t, len(ret), 1)
}

func TestGetLabelDiff(t *testing.T) {
	sub, plus := getLabelDiff(labels.Set{
		"foo": "bar",
		"bar": "baz",
		"baz": "qux",
	}, labels.Set{
		"foo": "bar1",
		"bar": "baz",
	})
	assert.Equal(t, len(sub), 2)
	assert.Equal(t, len(plus), 1)
}

func TestRuntimeInfoStore_Deployment(t *testing.T) {
	runtimeInfoStore := NewRuntimeInfoStore()
	deployment := appsv1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"foo": "bar1"},
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "foo",
												Operator: v1.NodeSelectorOpIn,
												Values:   []string{"bar2"},
											},
											{
												Key:      "foo",
												Operator: v1.NodeSelectorOpExists,
												Values:   []string{"bar3"},
											},
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpNotIn,
												Values:   []string{"bar"},
											},
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpDoesNotExist,
												Values:   []string{"bar2"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	runtimeInfoStore.PutDeployment(deployment)

	assert.Equal(t, len(runtimeInfoStore.peerDeploymentMap), 1)
	assert.Equal(t, len(runtimeInfoStore.depKeyToEqLabels["test/test"]["foo"]), 1)
	assert.Equal(t, len(runtimeInfoStore.depKeyToNeLabels["test/test"]["baz"]), 2)

	// Delete exist
	runtimeInfoStore.DeleteDeployment(deployment)
	assert.Equal(t, len(runtimeInfoStore.peerDeploymentMap), 0)
	assert.Equal(t, len(runtimeInfoStore.depKeyToEqLabels), 0)
	assert.Equal(t, len(runtimeInfoStore.depKeyToNeLabels), 0)
}

func TestRuntimeInfoStore_Node(t *testing.T) {
	runtimeInfoStore := NewRuntimeInfoStore()
	runtimeInfoStore.PutNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	})
	assert.Equal(t, len(runtimeInfoStore.nodeMap), 1)
	assert.Equal(t, len(runtimeInfoStore.labelKeyToValueToNodeKeysMap), 1)
	// update node
	runtimeInfoStore.PutNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"foo":  "bar",
				"foo2": "bar2",
			},
		},
	})
	assert.Equal(t, len(runtimeInfoStore.nodeMap), 1)
	assert.Equal(t, len(runtimeInfoStore.labelKeyToValueToNodeKeysMap), 2)

	// update node
	runtimeInfoStore.PutNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"foo":  "bar",
				"foo2": "bar3",
			},
		},
	})
	assert.Equal(t, len(runtimeInfoStore.nodeMap), 1)
	assert.Equal(t, len(runtimeInfoStore.labelKeyToValueToNodeKeysMap), 2)

	runtimeInfoStore.DeleteNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"foo":  "bar",
				"foo2": "bar3",
			},
		},
	})
}

func TestRuntimeInfoStore_NodeMatchDeployment(t *testing.T) {
	runtimeInfoStore := NewRuntimeInfoStore()
	node := &v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	}
	runtimeInfoStore.PutNode(node)
	assert.Equal(t, len(runtimeInfoStore.nodeMap), 1)
	assert.Equal(t, len(runtimeInfoStore.labelKeyToValueToNodeKeysMap), 1)
	runtimeInfoStore.PutDeployment(appsv1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "test1",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"foo": "bar"},
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpNotIn,
												Values:   []string{"bar"},
											},
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpDoesNotExist,
												Values:   []string{"bar2"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	runtimeInfoStore.PutDeployment(appsv1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "test2",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"foo1": "bar1"},
				},
			},
		},
	})

	runtimeInfoStore.PutDeployment(appsv1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "test3",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"foo": "bar1"},
				},
			},
		},
	})

	runtimeInfoStore.PutDeployment(appsv1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "test4",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "foo",
												Operator: v1.NodeSelectorOpNotIn,
												Values:   []string{"bar"},
											},
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpDoesNotExist,
												Values:   []string{"bar2"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	runtimeInfoStore.GetRelatedDeploymentsByNode(node)
	assert.Equal(t, len(runtimeInfoStore.nodeMap), 1)
}

func TestRuntimeInfoStore_DeploymentMatchNode(t *testing.T) {
	runtimeInfoStore := NewRuntimeInfoStore()
	num := runtimeInfoStore.GetMatchedNodeNum(appsv1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "test1",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"foo": "bar"},
				},
			},
		},
	})
	assert.Equal(t, num, 0)
	runtimeInfoStore.PutNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
	})
	runtimeInfoStore.PutNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test0",
			Labels: map[string]string{
				"baz": "bar",
			},
		},
	})
	num = runtimeInfoStore.GetMatchedNodeNum(appsv1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "test1",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{
						"foo": "bar",
						"baz": "bar",
					},
				},
			},
		},
	})
	assert.Equal(t, num, 0)
	runtimeInfoStore.PutNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test1",
			Labels: map[string]string{
				"foo": "bar1",
			},
		},
	})
	runtimeInfoStore.PutNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test2",
			Labels: map[string]string{
				"foo1": "bar1",
			},
		},
	})
	runtimeInfoStore.PutNode(&v1.Node{
		ObjectMeta: v12.ObjectMeta{
			Name: "test3",
			Labels: map[string]string{
				"foo": "bar",
				"baz": "bar",
			},
		},
	})
	num = runtimeInfoStore.GetMatchedNodeNum(appsv1.Deployment{
		ObjectMeta: v12.ObjectMeta{
			Name:      "test1",
			Namespace: "test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					NodeSelector: map[string]string{"foo": "bar"},
					Affinity: &v1.Affinity{
						NodeAffinity: &v1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
								NodeSelectorTerms: []v1.NodeSelectorTerm{
									{
										MatchExpressions: []v1.NodeSelectorRequirement{
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpNotIn,
												Values:   []string{"bar"},
											},
											{
												Key:      "baz",
												Operator: v1.NodeSelectorOpDoesNotExist,
												Values:   []string{"bar2"},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	})
	assert.Equal(t, num, 1)
}
