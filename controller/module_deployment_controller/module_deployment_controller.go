package module_deployment_controller

import (
	"context"
	"errors"
	"sort"

	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/module_tunnels"
	"github.com/koupleless/virtual-kubelet/common/log"
	"github.com/koupleless/virtual-kubelet/common/tracker"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ModuleDeploymentController is a controller that manages the deployment of modules within a specific environment.
type ModuleDeploymentController struct {
	env string // The environment in which the controller operates.

	tunnels []module_tunnels.ModuleTunnel // A list of tunnels for communication with modules.

	client client.Client // The client for interacting with the Kubernetes API.

	cache cache.Cache // The cache for storing and retrieving Kubernetes objects.

	runtimeStorage *RuntimeInfoStore // Storage for runtime information about deployments and nodes.

	updateToken chan interface{} // A channel for signaling updates.
}

// Reconcile is the main reconciliation function for the controller.
func (mdc *ModuleDeploymentController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// This function is a placeholder for actual reconciliation logic.
	return reconcile.Result{}, nil
}

// NewModuleDeploymentController creates a new instance of the controller.
func NewModuleDeploymentController(env string, tunnels []module_tunnels.ModuleTunnel) (*ModuleDeploymentController, error) {
	return &ModuleDeploymentController{
		env:            env,
		tunnels:        tunnels,
		runtimeStorage: NewRuntimeInfoStore(),
		updateToken:    make(chan interface{}, 1),
	}, nil
}

// SetupWithManager sets up the controller with a manager.
func (mdc *ModuleDeploymentController) SetupWithManager(ctx context.Context, mgr manager.Manager) (err error) {
	mdc.updateToken <- nil

	for _, tunnel := range mdc.tunnels {
		tunnel.RegisterQuery(mdc.queryContainerBaseline)
	}

	mdc.client = mgr.GetClient()
	mdc.cache = mgr.GetCache()

	log.G(ctx).Info("Setting up module deployment controller")

	c, err := controller.New("module-deployment-controller", mgr, controller.Options{
		Reconciler: mdc,
	})
	if err != nil {
		log.G(ctx).Error(err, "unable to set up module-deployment controller")
		return err
	}

	envRequirement, _ := labels.NewRequirement(vkModel.LabelKeyOfEnv, selection.In, []string{mdc.env})

	// first sync node cache
	nodeRequirement, _ := labels.NewRequirement(vkModel.LabelKeyOfComponent, selection.In, []string{vkModel.ComponentVNode})
	vnodeSelector := labels.NewSelector().Add(*nodeRequirement, *envRequirement)

	// sync deployment cache
	deploymentRequirement, _ := labels.NewRequirement(vkModel.LabelKeyOfComponent, selection.In, []string{model.ComponentModuleDeployment})
	deploymentSelector := labels.NewSelector().Add(*deploymentRequirement, *envRequirement)

	go func() {
		syncd := mdc.cache.WaitForCacheSync(ctx)
		if !syncd {
			log.G(ctx).Error("failed to wait for cache sync")
			return
		}
		// init
		vnodeList := &corev1.NodeList{}
		err = mdc.cache.List(ctx, vnodeList, &client.ListOptions{
			LabelSelector: vnodeSelector,
		})
		if err != nil {
			err = errors.New("failed to list vnode")
			return
		}

		for _, vnode := range vnodeList.Items {
			// no deployment, just add
			mdc.runtimeStorage.PutNode(vnode.DeepCopy())
		}

		// init deployments
		depList := &appsv1.DeploymentList{}
		err = mdc.cache.List(ctx, depList, &client.ListOptions{
			LabelSelector: deploymentSelector,
		})
		if err != nil {
			err = errors.New("failed to list deployments")
			return
		}

		for _, deployment := range depList.Items {
			mdc.runtimeStorage.PutDeployment(deployment)
		}

		mdc.updateDeploymentReplicas(depList.Items)
	}()

	var vnodeEventHandler = handler.TypedFuncs[*corev1.Node, reconcile.Request]{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.vnodeCreateHandler(e.Object)
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.vnodeUpdateHandler(e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.vnodeDeleteHandler(e.Object)
		},
		GenericFunc: func(ctx context.Context, e event.TypedGenericEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			log.G(ctx).WithField("node_name", e.Object.Name).Warn("Generic func call")
		},
	}

	if err = c.Watch(source.Kind(mgr.GetCache(), &corev1.Node{}, vnodeEventHandler, &VNodePredicates{LabelSelector: vnodeSelector})); err != nil {
		log.G(ctx).WithError(err).Error("unable to watch nodes")
		return err
	}

	var deploymentEventHandler = handler.TypedFuncs[*appsv1.Deployment, reconcile.Request]{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.deploymentAddHandler(e.Object)
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.deploymentUpdateHandler(e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.deploymentDeleteHandler(e.Object)
		},
		GenericFunc: func(ctx context.Context, e event.TypedGenericEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			log.G(ctx).WithField("deployment_name", e.Object.Name).Warn("Generic func call")
		},
	}

	if err = c.Watch(source.Kind(mgr.GetCache(), &appsv1.Deployment{}, &deploymentEventHandler, &ModuleDeploymentPredicates{LabelSelector: deploymentSelector})); err != nil {
		log.G(ctx).WithError(err).Error("unable to watch module Deployments")
		return err
	}

	log.G(ctx).Info("module-deployment controller ready")

	return nil
}

// queryContainerBaseline queries the baseline for a given container.
func (mdc *ModuleDeploymentController) queryContainerBaseline(req model.QueryBaselineRequest) []corev1.Container {
	labelMap := map[string]string{
		vkModel.LabelKeyOfEnv:          mdc.env,
		vkModel.LabelKeyOfVNodeName:    req.Name,
		vkModel.LabelKeyOfVNodeVersion: req.Version,
	}
	for key, value := range req.CustomLabels {
		labelMap[key] = value
	}
	relatedDeploymentsByNode := mdc.runtimeStorage.GetRelatedDeploymentsByNode(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: labelMap,
		},
	})
	// get relate containers of related deployments
	sort.Slice(relatedDeploymentsByNode, func(i, j int) bool {
		return relatedDeploymentsByNode[i].CreationTimestamp.UnixMilli() < relatedDeploymentsByNode[j].CreationTimestamp.UnixMilli()
	})
	// record last version of biz model with same name
	containers := make([]corev1.Container, 0)
	for _, deployment := range relatedDeploymentsByNode {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			containers = append(containers, container)
		}
	}
	log.G(context.Background()).Infof("query base line got: %", containers)
	return containers
}

// vnodeCreateHandler handles the creation of a new vnode.
func (mdc *ModuleDeploymentController) vnodeCreateHandler(vnode *corev1.Node) {
	changed := mdc.runtimeStorage.PutNode(vnode.DeepCopy())
	if changed {
		relatedDeploymentsByNode := mdc.runtimeStorage.GetRelatedDeploymentsByNode(vnode)
		go mdc.updateDeploymentReplicas(relatedDeploymentsByNode)
	}
}

// vnodeUpdateHandler handles the update of an existing vnode.
func (mdc *ModuleDeploymentController) vnodeUpdateHandler(_, vnode *corev1.Node) {
	changed := mdc.runtimeStorage.PutNode(vnode.DeepCopy())
	if changed {
		relatedDeploymentsByNode := mdc.runtimeStorage.GetRelatedDeploymentsByNode(vnode)
		go mdc.updateDeploymentReplicas(relatedDeploymentsByNode)
	}
}

// vnodeDeleteHandler handles the deletion of a vnode.
func (mdc *ModuleDeploymentController) vnodeDeleteHandler(vnode *corev1.Node) {
	vnodeCopy := vnode.DeepCopy()
	mdc.runtimeStorage.DeleteNode(vnodeCopy)
	relatedDeploymentsByNode := mdc.runtimeStorage.GetRelatedDeploymentsByNode(vnodeCopy)
	go mdc.updateDeploymentReplicas(relatedDeploymentsByNode)
}

// deploymentAddHandler handles the addition of a new deployment.
func (mdc *ModuleDeploymentController) deploymentAddHandler(dep *appsv1.Deployment) {
	if dep == nil {
		return
	}

	deploymentCopy := dep.DeepCopy()
	mdc.runtimeStorage.PutDeployment(*deploymentCopy)

	go mdc.updateDeploymentReplicas([]appsv1.Deployment{*deploymentCopy})
}

// deploymentUpdateHandler handles the update of an existing deployment.
func (mdc *ModuleDeploymentController) deploymentUpdateHandler(_, newDep *appsv1.Deployment) {
	if newDep == nil {
		return
	}
	deploymentCopy := newDep.DeepCopy()
	mdc.runtimeStorage.PutDeployment(*deploymentCopy)

	go mdc.updateDeploymentReplicas([]appsv1.Deployment{*deploymentCopy})
}

// deploymentDeleteHandler handles the deletion of a deployment.
func (mdc *ModuleDeploymentController) deploymentDeleteHandler(dep *appsv1.Deployment) {
	if dep == nil {
		return
	}
	mdc.runtimeStorage.DeleteDeployment(*dep.DeepCopy())
}

// updateDeploymentReplicas updates the replicas of deployments based on node count.
func (mdc *ModuleDeploymentController) updateDeploymentReplicas(deployments []appsv1.Deployment) {
	<-mdc.updateToken
	defer func() {
		mdc.updateToken <- nil
	}()
	for _, deployment := range deployments {
		if deployment.Labels[model.LabelKeyOfVPodDeploymentStrategy] != string(model.VPodDeploymentStrategyPeer) || deployment.Labels[model.LabelKeyOfSkipReplicasControl] == "true" {
			continue
		}
		newReplicas := mdc.runtimeStorage.GetMatchedNodeNum(deployment)
		if int32(newReplicas) != *deployment.Spec.Replicas {
			err := tracker.G().FuncTrack(deployment.Labels[vkModel.LabelKeyOfTraceID], vkModel.TrackSceneVPodDeploy, model.TrackEventVPodPeerDeploymentReplicaModify, deployment.Labels, func() (error, vkModel.ErrorCode) {
				return mdc.updateDeploymentReplicasOfKubernetes(newReplicas, deployment)
			})
			if err != nil {
				logrus.WithError(err).Errorf("failed to update deployment replicas of %s", deployment.Name)
			}
		}
	}
}

// updateDeploymentReplicasOfKubernetes updates the replicas of a deployment in Kubernetes.
func (mdc *ModuleDeploymentController) updateDeploymentReplicasOfKubernetes(replicas int, deployment appsv1.Deployment) (error, vkModel.ErrorCode) {
	deployment.Spec.Replicas = ptr.To[int32](int32(replicas))
	err := mdc.client.Update(context.TODO(), &deployment)
	if err != nil {
		return err, model.CodeKubernetesOperationFailed
	}
	return nil, vkModel.CodeSuccess
}
