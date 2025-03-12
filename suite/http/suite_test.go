package http

import (
	"context"
	"fmt"
	model2 "github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/controller/module_deployment_controller"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_http_tunnel"
	"github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/vnode_controller"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"testing"
	"time"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var testEnv *envtest.Environment
var k8sClient client.Client
var httpTunnel koupleless_http_tunnel.HttpTunnel
var ctx, cancel = context.WithCancel(context.Background())

const (
	clientID = "suite-test"
	env      = "suite"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Module Controller Suite")
}

var _ = BeforeSuite(func() {

	os.Setenv("ENABLE_MODULE_REPLICAS_SYNC_WITH_BASE", "true")

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
	log.L = logruslogger.FromLogrus(logrus.NewEntry(logrus.StandardLogger()))

	By("bootstrapping suite environment")
	//usingExistingCluster := true
	testEnv = &envtest.Environment{
		//UseExistingCluster: &usingExistingCluster,
	}

	var err error

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// Allow all connections.
	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: ":8081",
		},
	})
	Expect(err).ToNot(HaveOccurred())

	moduleDeploymentController, err := module_deployment_controller.NewModuleDeploymentController(env)
	Expect(err).ToNot(HaveOccurred())

	err = moduleDeploymentController.SetupWithManager(ctx, k8sManager)

	Expect(err).ToNot(HaveOccurred())

	httpTunnel = koupleless_http_tunnel.NewHttpTunnel(ctx, env, k8sManager.GetClient(), moduleDeploymentController, 7777)

	err = httpTunnel.Start(clientID, env)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to start tunnel", httpTunnel.Key())
		panic(fmt.Sprintf("failed to start tunnel %s", httpTunnel.Key()))
	} else {
		log.G(ctx).Info("Tunnel started: ", httpTunnel.Key())
	}

	vnodeController, err := vnode_controller.NewVNodeController(&model.BuildVNodeControllerConfig{
		ClientID:       clientID,
		Env:            env,
		VPodType:       model2.ComponentModule,
		VNodeWorkerNum: 4,
	}, &httpTunnel)
	Expect(err).ToNot(HaveOccurred())

	err = vnodeController.SetupWithManager(ctx, k8sManager)

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	go func() {
		err = k8sManager.Start(ctx)
		log.G(ctx).WithError(err).Error("k8sManager Start")
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sManager.GetCache().WaitForCacheSync(ctx)
})

var _ = AfterSuite(func() {
	By("tearing down the suite environment")
	cancel()
	testEnv.Stop()
	time.Sleep(15 * time.Second)
	log.G(ctx).Info("suite for http stopped!")
})

func prepareModulePod(name, namespace, nodeName string) v1.Pod {
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				model.LabelKeyOfComponent: model2.ComponentModule,
			},
		},
		Spec: v1.PodSpec{
			NodeName: nodeName,
			Containers: []v1.Container{
				{
					Name:  "biz",
					Image: "suite-biz.jar",
					Env: []v1.EnvVar{
						{
							Name:  "BIZ_VERSION",
							Value: "0.0.1",
						},
					},
				},
			},
			Tolerations: []v1.Toleration{
				{
					Key:      model.TaintKeyOfVnode,
					Operator: v1.TolerationOpEqual,
					Value:    "True",
					Effect:   v1.TaintEffectNoExecute,
				},
				{
					Key:      model.TaintKeyOfEnv,
					Operator: v1.TolerationOpEqual,
					Value:    env,
					Effect:   v1.TaintEffectNoExecute,
				},
			},
		},
	}
}
