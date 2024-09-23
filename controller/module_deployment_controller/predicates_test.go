package module_deployment_controller

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"testing"
)

func TestVNodePredicates(t *testing.T) {
	labelSelector := labels.SelectorFromSet(labels.Set{"env": "test"})
	vNodePredicates := &VNodePredicates{LabelSelector: labelSelector}

	nodeWithLabels := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"env": "test"}}}
	nodeWithoutLabels := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"env": "prod"}}}

	// Test Create
	createEvent := event.TypedCreateEvent[*corev1.Node]{Object: nodeWithLabels}
	if !vNodePredicates.Create(createEvent) {
		t.Errorf("Expected Create to return true, got false")
	}

	createEvent = event.TypedCreateEvent[*corev1.Node]{Object: nodeWithoutLabels}
	if vNodePredicates.Create(createEvent) {
		t.Errorf("Expected Create to return false, got true")
	}

	// Test Delete
	deleteEvent := event.TypedDeleteEvent[*corev1.Node]{Object: nodeWithLabels}
	if !vNodePredicates.Delete(deleteEvent) {
		t.Errorf("Expected Delete to return true, got false")
	}

	deleteEvent = event.TypedDeleteEvent[*corev1.Node]{Object: nodeWithoutLabels}
	if vNodePredicates.Delete(deleteEvent) {
		t.Errorf("Expected Delete to return false, got true")
	}

	// Test Update
	updateEvent := event.TypedUpdateEvent[*corev1.Node]{ObjectNew: nodeWithLabels}
	if !vNodePredicates.Update(updateEvent) {
		t.Errorf("Expected Update to return true, got false")
	}

	updateEvent = event.TypedUpdateEvent[*corev1.Node]{ObjectNew: nodeWithoutLabels}
	if vNodePredicates.Update(updateEvent) {
		t.Errorf("Expected Update to return false, got true")
	}

	// Test Generic
	genericEvent := event.TypedGenericEvent[*corev1.Node]{Object: nodeWithLabels}
	if vNodePredicates.Generic(genericEvent) {
		t.Errorf("Expected Generic to return false, got true")
	}
}

func TestModuleDeploymentPredicates(t *testing.T) {
	labelSelector := labels.SelectorFromSet(labels.Set{"env": "test"})
	moduleDeploymentPredicates := &ModuleDeploymentPredicates{LabelSelector: labelSelector}

	deploymentWithLabels := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"env": "test"}}}
	deploymentWithoutLabels := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"env": "prod"}}}

	// Test Create
	createEvent := event.TypedCreateEvent[*appsv1.Deployment]{Object: deploymentWithLabels}
	if !moduleDeploymentPredicates.Create(createEvent) {
		t.Errorf("Expected Create to return true, got false")
	}

	createEvent = event.TypedCreateEvent[*appsv1.Deployment]{Object: deploymentWithoutLabels}
	if moduleDeploymentPredicates.Create(createEvent) {
		t.Errorf("Expected Create to return false, got true")
	}

	// Test Delete
	deleteEvent := event.TypedDeleteEvent[*appsv1.Deployment]{Object: deploymentWithLabels}
	if !moduleDeploymentPredicates.Delete(deleteEvent) {
		t.Errorf("Expected Delete to return true, got false")
	}

	deleteEvent = event.TypedDeleteEvent[*appsv1.Deployment]{Object: deploymentWithoutLabels}
	if moduleDeploymentPredicates.Delete(deleteEvent) {
		t.Errorf("Expected Delete to return false, got true")
	}

	// Test Update
	updateEvent := event.TypedUpdateEvent[*appsv1.Deployment]{ObjectNew: deploymentWithLabels}
	if !moduleDeploymentPredicates.Update(updateEvent) {
		t.Errorf("Expected Update to return true, got false")
	}

	updateEvent = event.TypedUpdateEvent[*appsv1.Deployment]{ObjectNew: deploymentWithoutLabels}
	if moduleDeploymentPredicates.Update(updateEvent) {
		t.Errorf("Expected Update to return false, got true")
	}

	// Test Generic
	genericEvent := event.TypedGenericEvent[*appsv1.Deployment]{Object: deploymentWithLabels}
	if !moduleDeploymentPredicates.Generic(genericEvent) {
		t.Errorf("Expected Generic to return true, got false")
	}
}
