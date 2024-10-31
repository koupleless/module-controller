package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/koupleless/virtual-kubelet/common/log"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"
)

/**
1. apply examples self-test base.yaml
2. run main
*/

var k8sClient client.Client
var k8sCache cache.Cache

var moduleDeploymentTemplate string

func init() {
	content, err := os.ReadFile("examples/self-test/module.yaml")
	if err != nil {
		fmt.Println("Error reading module yaml")
		os.Exit(1)
	}
	moduleDeploymentTemplate = string(content)
}

var podNameToCreateTime = make(map[string]time.Time)
var podNameToReadyTime = make(map[string]time.Time)

var deploymentNameToCreateTime = make(map[string]time.Time)
var deploymentNameToReadyTime = make(map[string]time.Time)

// base num list
var baseNumList = []int{
	1,
	5, 10, 20, 40, 60, 80, 100,
}

// module num list
var moduleNumList = []int{
	1,
	5, 10, 20, 50, 100, 200, 500, 1000,
}

type TestResult struct {
	// PodMin represents the minimum time (in milliseconds) for a pod to become ready
	PodMin int64
	// PodMax represents the maximum time (in milliseconds) for a pod to become ready
	PodMax int64
	// PodAvg represents the average time (in milliseconds) for pods to become ready
	PodAvg int64
	// DeployMin represents the minimum time (in milliseconds) for a deployment to become ready
	DeployMin int64
	// DeployMax represents the maximum time (in milliseconds) for a deployment to become ready
	DeployMax int64
	// DeployAvg represents the average time (in milliseconds) for deployments to become ready
	DeployAvg int64
}

func constructModuleDeployment(baseNum int, bizIndex int, bizVersion string) v1.Deployment {
	template := strings.ReplaceAll(moduleDeploymentTemplate, "{baseNum}", strconv.Itoa(baseNum))
	template = strings.ReplaceAll(template, "{bizIndex}", strconv.Itoa(bizIndex))
	template = strings.ReplaceAll(template, "{bizVersion}", bizVersion)
	ret := v1.Deployment{}
	err := yaml.Unmarshal([]byte(template), &ret)
	if err != nil {
		panic(err)
	}
	return ret
}

func updateReplicas(newReplicas int32) error {
	obj := &v1.Deployment{}
	err := k8sClient.Get(context.TODO(), types.NamespacedName{
		Name:      "base",
		Namespace: "default",
	}, obj)
	if err != nil {
		fmt.Println("Error getting base:", err)
	} else {
		if obj.Spec.Replicas != nil && *obj.Spec.Replicas == newReplicas {
			return nil
		}
		obj.Spec.Replicas = ptr.To[int32](newReplicas)
		err = k8sClient.Update(context.TODO(), obj)
		if err != nil {
			fmt.Println("Error update base:", err)
			return err
		}
	}

	allReady := false
	for !allReady {
		fmt.Println("check base ready: ", newReplicas)
		list := corev1.NodeList{}
		err = k8sClient.List(context.TODO(), &list)
		if err != nil {
			fmt.Println("Error listing nodes:", err.Error())
			continue
		}
		readyNum := 0
		for _, node := range list.Items {
			if !strings.HasPrefix(node.Name, "vnode") {
				continue
			}
			nodeReady := false
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					nodeReady = true
					break
				}
			}
			if nodeReady {
				readyNum++
			}
		}
		if readyNum == int(newReplicas) {
			allReady = true
		} else {
			time.Sleep(2 * time.Second)
		}
	}
	return nil
}

func statistic() TestResult {
	ret := TestResult{}
	minMillis := int64(1000 * 60 * 100)
	maxMillis := int64(0)
	totalMillis := int64(0)
	totalValidNum := int64(0)
	for podName, readyTime := range podNameToReadyTime {
		createTime, has := podNameToCreateTime[podName]
		if !has {
			fmt.Println("podName: ", podName, ", not find create time")
			continue
		}
		totalValidNum++
		readyCost := readyTime.Sub(createTime).Milliseconds()
		if readyCost > maxMillis {
			maxMillis = readyCost
		}
		if readyCost < minMillis {
			minMillis = readyCost
		}
		totalMillis += readyCost
	}
	ret.PodMin = minMillis
	ret.PodMax = maxMillis
	ret.PodAvg = totalMillis / totalValidNum

	minMillis = int64(1000 * 60 * 100)
	maxMillis = int64(0)
	totalMillis = int64(0)
	totalValidNum = int64(0)
	for deploymentName, readyTime := range deploymentNameToReadyTime {
		createTime, has := deploymentNameToCreateTime[deploymentName]
		if !has {
			fmt.Println("deploymentName: ", deploymentName, ", not find create time")
			continue
		}
		totalValidNum++
		readyCost := readyTime.Sub(createTime).Milliseconds()
		if readyCost > maxMillis {
			maxMillis = readyCost
		}
		if readyCost < minMillis {
			minMillis = readyCost
		}
		totalMillis += readyCost
	}
	ret.DeployMin = minMillis
	ret.DeployMax = maxMillis
	ret.DeployAvg = totalMillis / totalValidNum
	return ret
}

func test(baseNum, moduleNum int) (create TestResult, update *TestResult) {

	fmt.Println("start test: ", baseNum, moduleNum)

	podNameToReadyTime = make(map[string]time.Time)
	podNameToCreateTime = make(map[string]time.Time)
	deploymentNameToCreateTime = make(map[string]time.Time)
	deploymentNameToReadyTime = make(map[string]time.Time)
	// 1. 更新基座deployment，等待所有的 node 上线
	err := updateReplicas(int32(baseNum))
	if err != nil {
		os.Exit(2)
	}
	// 2. 按序发布模块deployment，replicas 设置为base数量
	for i := 1; i <= moduleNum; i++ {
		deployment := constructModuleDeployment(baseNum, i, "0.0.1")
		err = k8sClient.Create(context.TODO(), &deployment)
		if err != nil {
			fmt.Println("Error creating module deployment:", err.Error())
			os.Exit(2)
		}
		deploymentNameToCreateTime[deployment.Name] = time.Now()
	}
	// 3. 等待所有的deployment都变成ready，对数据进行统计，存到create
	for len(deploymentNameToReadyTime) != moduleNum {
		fmt.Println("waiting for all module deployment ready in creating: ", len(deploymentNameToReadyTime), "/", moduleNum)
		time.Sleep(time.Second * 5)
	}

	time.Sleep(time.Second * 5)

	create = statistic()

	return
}

type Result struct {
	Create TestResult
	Update *TestResult `json:"Update,omitempty"`
}

func cleanResources() {

	// clean resources
	list := v1.DeploymentList{}
	err := k8sClient.List(context.TODO(), &list, client.InNamespace("module"))
	if err != nil {
		fmt.Println("Error listing deployments:", err.Error())
		os.Exit(1)
	}

	for _, deployment := range list.Items {
		err = k8sClient.Delete(context.TODO(), &deployment)
		if err != nil {
			fmt.Println("Error listing deployments:", err.Error())
			os.Exit(1)
		}
	}

	// check all pod has been released
	existModulePod := true
	for existModulePod {
		fmt.Println("waiting for module pod to delete")
		podList := &corev1.PodList{}
		err = k8sClient.List(context.TODO(), podList, client.InNamespace("module"))
		if err != nil {
			fmt.Println("Error listing pods:", err.Error())
			os.Exit(1)
		}
		if len(podList.Items) == 0 {
			existModulePod = false
		}
		time.Sleep(time.Second * 5)
	}
}

func pressureTest() {
	cleanResources()

	res := make(map[int]map[int]Result)
	dir := "test_results"
	filename := "test_result_http_tunnel_worker8_netty8.json"

	filename = path.Join(dir, filename)

	_, err := os.Stat(filename)
	if err != nil {
		os.Create(filename)
		os.WriteFile(filename, []byte("{}"), 0644)
	}

	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	err = json.Unmarshal(content, &res)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	for _, baseNum := range baseNumList {
		val, has := res[baseNum]
		if !has {
			val = make(map[int]Result)
		}
		for _, moduleNum := range moduleNumList {
			_, done := val[moduleNum]
			if done {
				continue
			}
			create, update := test(baseNum, moduleNum)
			val[moduleNum] = Result{
				Create: create,
				Update: update,
			}
			res[baseNum] = val
			// save result
			marshal, err := json.MarshalIndent(res, "", "    ")
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			err = os.WriteFile(filename, marshal, 0644)
			if err != nil {
				fmt.Println(err.Error())
				return
			}
			cleanResources()
		}
	}

}

type Controller struct{}

func (c Controller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

var _ reconcile.TypedReconciler[reconcile.Request] = Controller{}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	kubeConfig := config.GetConfigOrDie()
	mgr, err := manager.New(kubeConfig, manager.Options{
		Cache: cache.Options{},
		Metrics: server.Options{
			BindAddress: "0",
		},
	})

	if err != nil {
		fmt.Println(err.Error(), "unable to set up overall controller manager")
		os.Exit(1)
	}

	k8sClient = mgr.GetClient()
	k8sCache = mgr.GetCache()

	c, err := controller.New("vnode-controller", mgr, controller.Options{
		Reconciler: Controller{},
	})
	if err != nil {
		fmt.Println(err.Error(), "unable to set up overall controller manager")
		os.Exit(1)
	}

	vpodRequirement, _ := labels.NewRequirement("virtual-kubelet.koupleless.io/component", selection.In, []string{"module"})

	podHandler := handler.TypedFuncs[*corev1.Pod, reconcile.Request]{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*corev1.Pod], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			podNameToCreateTime[e.Object.Name] = time.Now()
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*corev1.Pod], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if e.ObjectNew.DeletionTimestamp != nil {
				// 已删除
				return
			}
			_, has := podNameToReadyTime[e.ObjectNew.Name]
			if has {
				return
			}
			isReady := false
			for _, condition := range e.ObjectNew.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					isReady = true
					break
				}
			}
			if !isReady {
				return
			}
			podNameToReadyTime[e.ObjectNew.Name] = time.Now()
		},
	}

	if err = c.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{}, &podHandler, &VPodPredicates{
		LabelSelector: labels.NewSelector().Add(*vpodRequirement),
	})); err != nil {
		fmt.Println(err.Error(), "unable to watch Pods")
		os.Exit(1)
	}

	nodeHandler := handler.TypedFuncs[*corev1.Node, reconcile.Request]{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*corev1.Node], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			log.G(ctx).Info("vnode created ", e.Object.Name)
		},
	}

	vnodeRequirement, _ := labels.NewRequirement("virtual-kubelet.koupleless.io/component", selection.In, []string{"vnode"})
	envRequirement, _ := labels.NewRequirement("virtual-kubelet.koupleless.io/env", selection.In, []string{"test"})

	if err = c.Watch(source.Kind(mgr.GetCache(), &corev1.Node{}, &nodeHandler, &VNodePredicate{
		VNodeLabelSelector: labels.NewSelector().Add(*vnodeRequirement, *envRequirement),
	})); err != nil {
		fmt.Println(err.Error(), "unable to watch vnodes")
		os.Exit(1)
	}

	// sync deployment cache
	deploymentRequirement, _ := labels.NewRequirement("virtual-kubelet.koupleless.io/component", selection.In, []string{"module-deployment"})
	deploymentSelector := labels.NewSelector().Add(*deploymentRequirement)

	var deploymentEventHandler = handler.TypedFuncs[*v1.Deployment, reconcile.Request]{
		CreateFunc: func(ctx context.Context, e event.TypedCreateEvent[*v1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
		},
		UpdateFunc: func(ctx context.Context, e event.TypedUpdateEvent[*v1.Deployment], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			if e.ObjectNew.DeletionTimestamp != nil {
				return
			}
			if e.ObjectNew.Status.ReadyReplicas == *e.ObjectNew.Spec.Replicas {
				deploymentNameToReadyTime[e.ObjectNew.Name] = time.Now()
			}
		},
	}

	if err = c.Watch(source.Kind(mgr.GetCache(), &v1.Deployment{}, &deploymentEventHandler, &ModuleDeploymentPredicates{LabelSelector: deploymentSelector})); err != nil {
		fmt.Println(err.Error(), "unable to watch deployments")
		os.Exit(1)
	}

	go func() {
		err = mgr.Start(signals.SetupSignalHandler())
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to start manager")
		}
	}()

	sync := k8sCache.WaitForCacheSync(ctx)
	if !sync {
		fmt.Println("unable to sync cache")
		os.Exit(1)
	}

	pressureTest()
}
