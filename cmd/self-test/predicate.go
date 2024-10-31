package main

import (
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ predicate.TypedPredicate[*corev1.Pod] = &VPodPredicates{}

type VPodPredicates struct {
	LabelSelector labels.Selector
}

func (V *VPodPredicates) Create(e event.TypedCreateEvent[*corev1.Pod]) bool {
	return V.LabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (V *VPodPredicates) Delete(e event.TypedDeleteEvent[*corev1.Pod]) bool {
	return V.LabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (V *VPodPredicates) Update(e event.TypedUpdateEvent[*corev1.Pod]) bool {
	return V.LabelSelector.Matches(labels.Set(e.ObjectNew.Labels))
}

func (V *VPodPredicates) Generic(e event.TypedGenericEvent[*corev1.Pod]) bool {
	return false
}

var _ predicate.TypedPredicate[*corev1.Node] = &VNodePredicate{}

type VNodePredicate struct {
	VNodeLabelSelector labels.Selector
}

type ModuleDeploymentPredicates struct {
	LabelSelector labels.Selector
}

func (V *VNodePredicate) Create(e event.TypedCreateEvent[*corev1.Node]) bool {
	return V.VNodeLabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (V *VNodePredicate) Delete(e event.TypedDeleteEvent[*corev1.Node]) bool {
	return V.VNodeLabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (V *VNodePredicate) Update(e event.TypedUpdateEvent[*corev1.Node]) bool {
	return V.VNodeLabelSelector.Matches(labels.Set(e.ObjectNew.Labels))
}

func (V *VNodePredicate) Generic(e event.TypedGenericEvent[*corev1.Node]) bool {
	return V.VNodeLabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (m *ModuleDeploymentPredicates) Create(e event.TypedCreateEvent[*v1.Deployment]) bool {
	return m.LabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (m *ModuleDeploymentPredicates) Delete(e event.TypedDeleteEvent[*v1.Deployment]) bool {
	return m.LabelSelector.Matches(labels.Set(e.Object.Labels))
}

func (m *ModuleDeploymentPredicates) Update(e event.TypedUpdateEvent[*v1.Deployment]) bool {
	return m.LabelSelector.Matches(labels.Set(e.ObjectNew.Labels))
}

func (m *ModuleDeploymentPredicates) Generic(e event.TypedGenericEvent[*v1.Deployment]) bool {
	return m.LabelSelector.Matches(labels.Set(e.Object.Labels))
}
