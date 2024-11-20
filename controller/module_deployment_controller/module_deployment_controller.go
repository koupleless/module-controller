package module_deployment_controller

import (
	"context"
	"errors"
	"github.com/koupleless/virtual-kubelet/common/tracker"
	"github.com/koupleless/virtual-kubelet/common/utils"
	"github.com/sirupsen/logrus"
	"sort"

	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/virtual-kubelet/common/log"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
func NewModuleDeploymentController(env string) (*ModuleDeploymentController, error) {
	return &ModuleDeploymentController{
		env:            env,
		runtimeStorage: NewRuntimeInfoStore(),
		updateToken:    make(chan interface{}, 1),
	}, nil
}

// SetupWithManager sets up the controller with a manager.
func (mdc *ModuleDeploymentController) SetupWithManager(ctx context.Context, mgr manager.Manager) (err error) {
	mdc.updateToken <- nil
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

		mdc.updateDeploymentReplicas(ctx, depList.Items)
	}()

	var vnodeEventHandler = handler.TypedFuncs[*corev1.Node, reconcile.Request]{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.vnodeCreateHandler(ctx, e.Object)
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.vnodeUpdateHandler(ctx, e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.vnodeDeleteHandler(ctx, e.Object)
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
			mdc.deploymentAddHandler(ctx, e.Object)
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			mdc.deploymentUpdateHandler(ctx, e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
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

// QueryContainerBaseline queries the baseline for a given container.
func (mdc *ModuleDeploymentController) QueryContainerBaseline(req model.QueryBaselineRequest) []corev1.Container {
	labelMap := map[string]string{
		vkModel.LabelKeyOfEnv:              mdc.env,
		vkModel.LabelKeyOfVNodeVersion:     req.Version,
		vkModel.LabelKeyOfVNodeClusterName: req.ClusterName,
	}
	for key, value := range req.CustomLabels {
		labelMap[key] = value
	}
	relatedDeploymentsByNode := appsv1.DeploymentList{}
	err := mdc.cache.List(context.Background(), &relatedDeploymentsByNode, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelMap),
	})
	if err != nil {
		log.G(context.Background()).WithError(err).Error("failed to list deployments")
		return []corev1.Container{}
	}

	// get relate containers of related deployments
	sort.Slice(relatedDeploymentsByNode.Items, func(i, j int) bool {
		return relatedDeploymentsByNode.Items[i].CreationTimestamp.UnixMilli() < relatedDeploymentsByNode.Items[j].CreationTimestamp.UnixMilli()
	})
	// record last version of biz model with same name
	containers := make([]corev1.Container, 0)
	for _, deployment := range relatedDeploymentsByNode.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			containers = append(containers, container)
		}
	}
	log.G(context.Background()).Infof("query base line got: %", containers)
	return containers
}

// vnodeCreateHandler handles the creation of a new vnode.
func (mdc *ModuleDeploymentController) vnodeCreateHandler(ctx context.Context, vnode *corev1.Node) {
	relatedDeploymentsByNode := mdc.GetRelatedDeploymentsByNode(ctx, vnode)
	mdc.updateDeploymentReplicas(ctx, relatedDeploymentsByNode)
}

// vnodeUpdateHandler handles the update of an existing vnode.
func (mdc *ModuleDeploymentController) vnodeUpdateHandler(ctx context.Context, _, vnode *corev1.Node) {
	relatedDeploymentsByNode := mdc.GetRelatedDeploymentsByNode(ctx, vnode)
	mdc.updateDeploymentReplicas(ctx, relatedDeploymentsByNode)
}

// vnodeDeleteHandler handles the deletion of a vnode.
func (mdc *ModuleDeploymentController) vnodeDeleteHandler(ctx context.Context, vnode *corev1.Node) {
	vnodeCopy := vnode.DeepCopy()
	relatedDeploymentsByNode := mdc.GetRelatedDeploymentsByNode(ctx, vnodeCopy)
	mdc.updateDeploymentReplicas(ctx, relatedDeploymentsByNode)
}

func (mdc *ModuleDeploymentController) GetRelatedDeploymentsByNode(ctx context.Context, node *corev1.Node) []appsv1.Deployment {
	matchedDeployments := make([]appsv1.Deployment, 0)

	clusterName := getClusterNameFromNode(node)
	if clusterName == "" {
		logrus.Warnf("failed to get cluster name of node %s", node.Name)
		return matchedDeployments
	}

	deploymentList := appsv1.DeploymentList{}
	err := mdc.cache.List(ctx, &appsv1.DeploymentList{}, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			model.LabelKeyOfVPodDeploymentStrategy: string(model.VPodDeploymentStrategyPeer),
		}),
	})

	if err != nil {
		logrus.WithError(err).Error("failed to list deployments")
		return matchedDeployments
	}

	for _, deployment := range deploymentList.Items {
		clusterNameFromDeployment := getClusterNameFromDeployment(&deployment)
		if clusterName == clusterNameFromDeployment && clusterNameFromDeployment != "" {
			matchedDeployments = append(matchedDeployments, deployment)
		}
	}

	return matchedDeployments
}

// deploymentAddHandler handles the addition of a new deployment.
func (mdc *ModuleDeploymentController) deploymentAddHandler(ctx context.Context, dep *appsv1.Deployment) {
	if dep == nil {
		return
	}

	mdc.updateDeploymentReplicas(ctx, []appsv1.Deployment{*dep})
}

// deploymentUpdateHandler handles the update of an existing deployment.
func (mdc *ModuleDeploymentController) deploymentUpdateHandler(ctx context.Context, _, newDep *appsv1.Deployment) {
	if newDep == nil {
		return
	}

	mdc.updateDeploymentReplicas(ctx, []appsv1.Deployment{*newDep})
}

// updateDeploymentReplicas updates the replicas of deployments based on node count.
func (mdc *ModuleDeploymentController) updateDeploymentReplicas(ctx context.Context, deployments []appsv1.Deployment) {

	// TODO Implement this function.
	<-mdc.updateToken
	defer func() {
		mdc.updateToken <- nil
	}()

	enableModuleReplicasSameWithBase := utils.GetEnv("ENABLE_MODULE_REPLICAS_SYNC_WITH_BASE", "false")
	if enableModuleReplicasSameWithBase != "true" {
		return
	}

	for _, deployment := range deployments {
		if deployment.Labels[model.LabelKeyOfVPodDeploymentStrategy] != string(model.VPodDeploymentStrategyPeer) || deployment.Labels[model.LabelKeyOfSkipReplicasControl] == "true" {
			continue
		}

		clusterName := getClusterNameFromDeployment(&deployment)
		if clusterName == "" {
			logrus.Warnf("failed to get cluster name of deployment %s, skip to update replicas", deployment.Name)
			continue
		}

		sameClusterNodeCount, err := mdc.getReadyNodeCount(ctx, clusterName)
		if err != nil {
			logrus.WithError(err).Errorf("failed to get nodes of cluster %s, skip to update relicas", clusterName)
			continue
		}

		if int32(sameClusterNodeCount) != *deployment.Spec.Replicas {
			err := tracker.G().FuncTrack(deployment.Labels[vkModel.LabelKeyOfTraceID], vkModel.TrackSceneVPodDeploy, model.TrackEventVPodPeerDeploymentReplicaModify, deployment.Labels, func() (error, vkModel.ErrorCode) {
				return mdc.updateDeploymentReplicasOfKubernetes(sameClusterNodeCount, deployment)
			})
			if err != nil {
				logrus.WithError(err).Errorf("failed to update deployment replicas of %s", deployment.Name)
			}
		}
	}
}

func getClusterNameFromDeployment(deployment *appsv1.Deployment) string {
	if clusterName, has := deployment.Labels[vkModel.LabelKeyOfVNodeClusterName]; has {
		return clusterName
	}

	if deployment.Spec.Template.Spec.NodeSelector != nil {
		if clusterName, has := deployment.Spec.Template.Spec.NodeSelector[vkModel.LabelKeyOfVNodeClusterName]; has {
			return clusterName
		}
	}

	if deployment.Spec.Template.Spec.Affinity != nil &&
		deployment.Spec.Template.Spec.Affinity.NodeAffinity != nil &&
		deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
		deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms != nil {
		for _, term := range deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			for _, expr := range term.MatchExpressions {
				if expr.Key == vkModel.LabelKeyOfVNodeClusterName {
					return expr.Values[0]
				}
			}
		}
	}

	return ""
}

func getClusterNameFromNode(node *corev1.Node) string {
	if clusterName, has := node.Labels[vkModel.LabelKeyOfVNodeClusterName]; has {
		return clusterName
	}

	return ""
}

func (mdc *ModuleDeploymentController) getReadyNodeCount(ctx context.Context, clusterName string) (int, error) {
	nodeList := &corev1.NodeList{}
	err := mdc.cache.List(ctx, nodeList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{vkModel.LabelKeyOfVNodeClusterName: clusterName}),
	})

	if err != nil {
		logrus.WithError(err).Errorf("failed to list nodes of cluster %s", clusterName)
		return 0, err
	}

	readyNodeCnt := 0
	for _, node := range nodeList.Items {
		if node.Status.Phase == corev1.NodeRunning {
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady {
					if cond.Status == corev1.ConditionTrue {
						readyNodeCnt++
						break
					}
				}
			}
		}
	}

	return readyNodeCnt, nil
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
