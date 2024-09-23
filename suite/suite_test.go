package suite

import (
	"context"
	model2 "github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/controller/module_deployment_controller"
	"github.com/koupleless/module_controller/module_tunnels"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_mqtt_tunnel"
	"github.com/koupleless/virtual-kubelet/common/log"
	"github.com/koupleless/virtual-kubelet/controller/vnode_controller"
	"github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wind-c/comqtt/v2/mqtt"
	"github.com/wind-c/comqtt/v2/mqtt/hooks/auth"
	"github.com/wind-c/comqtt/v2/mqtt/listeners"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var testEnv *envtest.Environment
var k8sClient client.Client
var tl koupleless_mqtt_tunnel.MqttTunnel
var mqttServer *mqtt.Server

const (
	clientID = "suite-test"
	env      = "suite"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Module Controller Suite")
}

var _ = BeforeSuite(func() {

	os.Setenv("MQTT_BROKER", "localhost")
	os.Setenv("MQTT_PORT", "1883")

	os.Setenv("MQTT_USERNAME", "test")
	os.Setenv("MQTT_PASSWORD", "")
	os.Setenv("MQTT_CLIENT_PREFIX", "suite-test")

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping suite environment")
	testEnv = &envtest.Environment{}

	var err error

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// start embedded mqtt broker
	// Create the new MQTT Server.
	mqttServer = mqtt.New(nil)

	// Allow all connections.
	_ = mqttServer.AddHook(new(auth.AllowHook), nil)

	// Create a TCP listener on a standard port.
	tcp := listeners.NewTCP("t1", ":1883", nil)
	err = mqttServer.AddListener(tcp)
	Expect(err).NotTo(HaveOccurred())

	go mqttServer.Serve()

	err = scheme.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	tunnels := []tunnel.Tunnel{
		&tl,
	}

	ctx := context.Background()

	vnodeController, err := vnode_controller.NewVNodeController(&model.BuildVNodeControllerConfig{
		ClientID:     clientID,
		Env:          env,
		VPodIdentity: model2.ComponentModule,
	}, tunnels)
	Expect(err).ToNot(HaveOccurred())

	err = vnodeController.SetupWithManager(ctx, k8sManager)

	moduleDeploymentController, err := module_deployment_controller.NewModuleDeploymentController(env, []module_tunnels.ModuleTunnel{
		&tl,
	})
	Expect(err).ToNot(HaveOccurred())

	err = moduleDeploymentController.SetupWithManager(ctx, k8sManager)

	Expect(err).ToNot(HaveOccurred())

	for _, t := range tunnels {
		err = t.Start(ctx, clientID, env)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to start tunnel", t.Key())
		} else {
			log.G(ctx).Info("Tunnel started: ", t.Key())
		}
	}

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	time.Sleep(5 * time.Second)
})

var _ = AfterSuite(func() {
	By("tearing down the suite environment")
	testEnv.Stop()
	mqttServer.Close()
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
					Image: "suite-biz",
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
