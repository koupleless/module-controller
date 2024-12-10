package module_deployment_controller

import (
	"context"
	"errors"
	"fmt"
	"github.com/koupleless/module_controller/common/zaplogger"
	"github.com/koupleless/virtual-kubelet/common/tracker"
	"github.com/koupleless/virtual-kubelet/common/utils"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"sort"

	"github.com/koupleless/module_controller/common/model"
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

	updateToken chan interface{} // A channel for signaling updates.
}

// Reconcile is the main reconciliation function for the controller.
func (moduleDeploymentController *ModuleDeploymentController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	zaplogger.FromContext(ctx).Info("Reconciling module deployment", "request", request)
	// This function is a placeholder for actual reconciliation logic.
	return reconcile.Result{}, nil
}

// NewModuleDeploymentController creates a new instance of the controller.
func NewModuleDeploymentController(env string) (*ModuleDeploymentController, error) {
	return &ModuleDeploymentController{
		env:         env,
		updateToken: make(chan interface{}, 1),
	}, nil
}

// SetupWithManager sets up the controller with a manager.
func (moduleDeploymentController *ModuleDeploymentController) SetupWithManager(ctx context.Context, mgr manager.Manager) (err error) {
	logger := zaplogger.FromContext(ctx)
	moduleDeploymentController.updateToken <- nil
	moduleDeploymentController.client = mgr.GetClient()
	moduleDeploymentController.cache = mgr.GetCache()

	logger.Info("Setting up module deployment controller")

	customController, err := controller.New("module-deployment-controller", mgr, controller.Options{
		Reconciler: moduleDeploymentController,
	})
	if err != nil {
		logger.Error(err, "unable to set up module-deployment controller")
		return err
	}

	envRequirement, _ := labels.NewRequirement(vkModel.LabelKeyOfEnv, selection.In, []string{moduleDeploymentController.env})

	// first sync node cache
	nodeRequirement, _ := labels.NewRequirement(vkModel.LabelKeyOfComponent, selection.In, []string{vkModel.ComponentVNode})
	vnodeSelector := labels.NewSelector().Add(*nodeRequirement, *envRequirement)

	// sync deployment cache
	deploymentRequirement, _ := labels.NewRequirement(vkModel.LabelKeyOfComponent, selection.In, []string{model.ComponentModuleDeployment})
	deploymentSelector := labels.NewSelector().Add(*deploymentRequirement, *envRequirement)

	go func() {
		syncd := moduleDeploymentController.cache.WaitForCacheSync(ctx)
		if !syncd {
			logger.Error(nil, "failed to wait for cache sync")
			return
		}
		// init
		vnodeList := &corev1.NodeList{}
		err = moduleDeploymentController.cache.List(ctx, vnodeList, &client.ListOptions{
			LabelSelector: vnodeSelector,
		})
		if err != nil {
			err = errors.New("failed to list vnode")
			return
		}

		// init deployments
		depList := &appsv1.DeploymentList{}
		err = moduleDeploymentController.cache.List(ctx, depList, &client.ListOptions{
			LabelSelector: deploymentSelector,
		})

		if err != nil {
			err = errors.New("failed to list deployments")
			return
		}

		moduleDeploymentController.updateDeploymentReplicas(ctx, depList.Items)
	}()

	var vnodeEventHandler = handler.TypedFuncs[*corev1.Node, reconcile.Request]{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			moduleDeploymentController.vnodeCreateHandler(ctx, e.Object)
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			moduleDeploymentController.vnodeUpdateHandler(ctx, e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			moduleDeploymentController.vnodeDeleteHandler(ctx, e.Object)
		},
		GenericFunc: func(ctx context.Context, e event.TypedGenericEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			logger.WithValues("node_name", e.Object.Name).Info("Generic func call")
		},
	}

	if err = customController.Watch(source.Kind(mgr.GetCache(), &corev1.Node{}, vnodeEventHandler, &VNodePredicates{LabelSelector: vnodeSelector})); err != nil {
		logger.Error(err, "unable to watch nodes")
		return err
	}

	var deploymentEventHandler = handler.TypedFuncs[*appsv1.Deployment, reconcile.Request]{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			moduleDeploymentController.deploymentAddHandler(ctx, e.Object)
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			moduleDeploymentController.deploymentUpdateHandler(ctx, e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(ctx context.Context, e event.TypedDeleteEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		},
		GenericFunc: func(ctx context.Context, e event.TypedGenericEvent[*appsv1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			logger.WithValues("deployment_name", e.Object.Name).Info("Generic func call")
		},
	}

	if err = customController.Watch(source.Kind(mgr.GetCache(), &appsv1.Deployment{}, &deploymentEventHandler, &ModuleDeploymentPredicates{LabelSelector: deploymentSelector})); err != nil {
		logger.Error(err, "unable to watch module Deployments")
		return err
	}

	logger.Info("module-deployment controller ready")
	return nil
}

// QueryContainerBaseline queries the baseline for a given container.
func (moduleDeploymentController *ModuleDeploymentController) QueryContainerBaseline(ctx context.Context, req model.QueryBaselineRequest) []corev1.Container {
	logger := zaplogger.FromContext(ctx)
	labelMap := map[string]string{
		// TODO: should add those label to deployments by module controller
		vkModel.LabelKeyOfEnv: moduleDeploymentController.env,
	}
	allDeploymentList := appsv1.DeploymentList{}
	err := moduleDeploymentController.cache.List(context.Background(), &allDeploymentList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labelMap),
	})
	if err != nil {
		logger.Error(err, "failed to list deployments")
		return []corev1.Container{}
	}

	// get relate containers of related deployments
	sort.Slice(allDeploymentList.Items, func(i, j int) bool {
		return allDeploymentList.Items[i].CreationTimestamp.UnixMilli() < allDeploymentList.Items[j].CreationTimestamp.UnixMilli()
	})
	// record last version of biz model with same name
	containers := make([]corev1.Container, 0)
	containerNames := []string{}
	for _, deployment := range allDeploymentList.Items {
		clusterName := getClusterNameFromDeployment(&deployment)
		if clusterName != "" && clusterName == req.ClusterName {
			for _, container := range deployment.Spec.Template.Spec.Containers {
				containers = append(containers, container)
				containerNames = append(containerNames, utils.GetBizUniqueKey(&container))
			}
		}
	}
	logger.Info(fmt.Sprintf("query base line got: %s", containerNames))
	return containers
}

// vnodeCreateHandler handles the creation of a new vnode.
func (moduleDeploymentController *ModuleDeploymentController) vnodeCreateHandler(ctx context.Context, vnode *corev1.Node) {
	relatedDeploymentsByNode := moduleDeploymentController.GetRelatedDeploymentsByNode(ctx, vnode)
	moduleDeploymentController.updateDeploymentReplicas(ctx, relatedDeploymentsByNode)
}

// vnodeUpdateHandler handles the update of an existing vnode.
func (moduleDeploymentController *ModuleDeploymentController) vnodeUpdateHandler(ctx context.Context, _, vnode *corev1.Node) {
	relatedDeploymentsByNode := moduleDeploymentController.GetRelatedDeploymentsByNode(ctx, vnode)
	moduleDeploymentController.updateDeploymentReplicas(ctx, relatedDeploymentsByNode)
}

// vnodeDeleteHandler handles the deletion of a vnode.
func (moduleDeploymentController *ModuleDeploymentController) vnodeDeleteHandler(ctx context.Context, vnode *corev1.Node) {
	vnodeCopy := vnode.DeepCopy()
	relatedDeploymentsByNode := moduleDeploymentController.GetRelatedDeploymentsByNode(ctx, vnodeCopy)
	moduleDeploymentController.updateDeploymentReplicas(ctx, relatedDeploymentsByNode)
}

func (moduleDeploymentController *ModuleDeploymentController) GetRelatedDeploymentsByNode(ctx context.Context, node *corev1.Node) []appsv1.Deployment {
	logger := zaplogger.FromContext(ctx)
	matchedDeployments := make([]appsv1.Deployment, 0)

	clusterName := getClusterNameFromNode(node)
	if clusterName == "" {
		logger.Info(fmt.Sprintf("failed to get cluster name of node %s", node.Name))
		return matchedDeployments
	}

	deploymentList := appsv1.DeploymentList{}
	err := moduleDeploymentController.cache.List(ctx, &deploymentList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			model.LabelKeyOfVPodDeploymentStrategy: string(model.VPodDeploymentStrategyPeer),
		}),
	})

	if err != nil {
		logger.Error(err, "failed to list deployments")
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
func (moduleDeploymentController *ModuleDeploymentController) deploymentAddHandler(ctx context.Context, dep *appsv1.Deployment) {
	if dep == nil {
		return
	}

	moduleDeploymentController.updateDeploymentReplicas(ctx, []appsv1.Deployment{*dep})
}

// deploymentUpdateHandler handles the update of an existing deployment.
func (moduleDeploymentController *ModuleDeploymentController) deploymentUpdateHandler(ctx context.Context, _, newDep *appsv1.Deployment) {
	if newDep == nil {
		return
	}

	moduleDeploymentController.updateDeploymentReplicas(ctx, []appsv1.Deployment{*newDep})
}

// updateDeploymentReplicas updates the replicas of deployments based on node count.
func (moduleDeploymentController *ModuleDeploymentController) updateDeploymentReplicas(ctx context.Context, deployments []appsv1.Deployment) {
	logger := zaplogger.FromContext(ctx)

	// TODO Implement this function.
	<-moduleDeploymentController.updateToken
	defer func() {
		moduleDeploymentController.updateToken <- nil
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
			logger.Info(fmt.Sprintf("failed to get cluster name of deployment %s, skip to update replicas", deployment.Name))
			continue
		}

		sameClusterNodeCount, err := moduleDeploymentController.getReadyNodeCount(ctx, clusterName)
		if err != nil {
			logger.Error(err, fmt.Sprintf("failed to get nodes of cluster %s, skip to update relicas", clusterName))
			continue
		}

		if int32(sameClusterNodeCount) != *deployment.Spec.Replicas {
			err := tracker.G().FuncTrack(deployment.Labels[vkModel.LabelKeyOfTraceID], vkModel.TrackSceneVPodDeploy, model.TrackEventVPodPeerDeploymentReplicaModify, deployment.Labels, func() (error, vkModel.ErrorCode) {
				return moduleDeploymentController.updateDeploymentReplicasOfKubernetes(ctx, sameClusterNodeCount, deployment)
			})
			if err != nil {
				logger.Error(err, fmt.Sprintf("failed to update deployment replicas of %s", deployment.Name))
			}
		}
	}
}

func getClusterNameFromDeployment(deployment *appsv1.Deployment) string {
	if clusterName, has := deployment.Labels[vkModel.LabelKeyOfBaseClusterName]; has {
		return clusterName
	}

	if deployment.Spec.Template.Spec.NodeSelector != nil {
		if clusterName, has := deployment.Spec.Template.Spec.NodeSelector[vkModel.LabelKeyOfBaseClusterName]; has {
			return clusterName
		}
	}

	if deployment.Spec.Template.Spec.Affinity != nil &&
		deployment.Spec.Template.Spec.Affinity.NodeAffinity != nil &&
		deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil &&
		deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms != nil {
		for _, term := range deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
			for _, expr := range term.MatchExpressions {
				if expr.Key == vkModel.LabelKeyOfBaseClusterName {
					return expr.Values[0]
				}
			}
		}
	}

	return ""
}

func getClusterNameFromNode(node *corev1.Node) string {
	if clusterName, has := node.Labels[vkModel.LabelKeyOfBaseClusterName]; has {
		return clusterName
	}

	return ""
}

func (moduleDeploymentController *ModuleDeploymentController) getReadyNodeCount(ctx context.Context, clusterName string) (int, error) {
	logger := zaplogger.FromContext(ctx)
	nodeList := &corev1.NodeList{}
	err := moduleDeploymentController.cache.List(ctx, nodeList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{vkModel.LabelKeyOfBaseClusterName: clusterName}),
	})

	if err != nil {
		logger.Error(err, fmt.Sprintf("failed to list nodes of cluster %s", clusterName))
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
func (moduleDeploymentController *ModuleDeploymentController) updateDeploymentReplicasOfKubernetes(ctx context.Context, replicas int, deployment appsv1.Deployment) (error, vkModel.ErrorCode) {
	old := deployment.DeepCopy()
	patch := client.MergeFrom(old)

	deployment.Spec.Replicas = ptr.To[int32](int32(replicas))
	err := moduleDeploymentController.client.Patch(ctx, &deployment, patch)
	if err != nil && !errors2.IsNotFound(err) {
		return err, model.CodeKubernetesOperationFailed
	}
	return nil, vkModel.CodeSuccess
}
