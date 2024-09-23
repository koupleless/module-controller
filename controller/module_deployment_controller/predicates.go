package module_deployment_controller

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ predicate.TypedPredicate[*appsv1.Deployment] = &ModuleDeploymentPredicates{}
var _ predicate.TypedPredicate[*corev1.Node] = &VNodePredicates{}

type ModuleDeploymentPredicates struct {
	LabelSelector labels.Selector
}

type VNodePredicates struct {
	LabelSelector labels.Selector
}

func (V *VNodePredicates) Create(e event.TypedCreateEvent[*corev1.Node]) bool {
	return V.LabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (V *VNodePredicates) Delete(e event.TypedDeleteEvent[*corev1.Node]) bool {
	return V.LabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (V *VNodePredicates) Update(e event.TypedUpdateEvent[*corev1.Node]) bool {
	return V.LabelSelector.Matches(labels.Set(e.ObjectNew.Labels))
}

func (V *VNodePredicates) Generic(e event.TypedGenericEvent[*corev1.Node]) bool {
	return false
}

func (m *ModuleDeploymentPredicates) Create(e event.TypedCreateEvent[*appsv1.Deployment]) bool {
	return m.LabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (m *ModuleDeploymentPredicates) Delete(e event.TypedDeleteEvent[*appsv1.Deployment]) bool {
	return m.LabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (m *ModuleDeploymentPredicates) Update(e event.TypedUpdateEvent[*appsv1.Deployment]) bool {
	return m.LabelSelector.Matches(labels.Set(e.ObjectNew.Labels))
}

func (m *ModuleDeploymentPredicates) Generic(e event.TypedGenericEvent[*appsv1.Deployment]) bool {
	return m.LabelSelector.Matches(labels.Set(e.Object.Labels))
}
