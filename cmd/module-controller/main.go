/**
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/google/uuid"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/common/zaplogger"
	"github.com/koupleless/module_controller/controller/module_deployment_controller"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_http_tunnel"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_mqtt_tunnel"
	"github.com/koupleless/module_controller/report_server"
	"github.com/koupleless/module_controller/staging/kubelet_proxy"
	"github.com/koupleless/virtual-kubelet/common/tracker"
	"github.com/koupleless/virtual-kubelet/common/utils"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	"github.com/koupleless/virtual-kubelet/vnode_controller"
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"github.com/virtual-kubelet/virtual-kubelet/trace/opencensus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

const (
	certFilePath                 = "/etc/virtual-kubelet/tls/tls.crt"
	keyFilePath                  = "/etc/virtual-kubelet/tls/tls.key"
	DefaultKubeletHttpListenAddr = "10250" // Default listen address for the HTTP server
)

// Main function for the module controller
// Responsibilities:
// 1. Sets up signal handling for graceful shutdown
// 2. Initializes reporting server
// 3. Configures logging and tracing
// 4. Sets up controller manager and caches
// 5. Initializes tunnels (MQTT and HTTP) based on env vars
// 6. Creates and configures the VNode controller
// 7. Optionally creates module deployment controller
// 8. Starts all tunnels and the manager
// 9. Starts the kubelet proxy server(if enabled)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	go report_server.InitReportServer()

	log.L = logruslogger.FromLogrus(logrus.NewEntry(logrus.StandardLogger()))
	trace.T = opencensus.Adapter{}

	// Get configuration from environment variables
	clientID := utils.GetEnv(model.EnvKeyOfClientID, uuid.New().String())
	env := utils.GetEnv("ENV", "dev")

	zlogger := zaplogger.GetLogger()
	ctx = zaplogger.WithLogger(ctx, zlogger)

	// Parse configuration with defaults
	isCluster := utils.GetEnv(model.EnvKeyOfClusterModeEnabled, "") == "true"
	workloadMaxLevel, err := strconv.Atoi(utils.GetEnv(model.EnvKeyOfWorkloadMaxLevel, "3"))

	if err != nil {
		zlogger.Error(err, "failed to parse WORKLOAD_MAX_LEVEL, will be set to 3 default")
		workloadMaxLevel = 3
	}

	vnodeWorkerNum, err := strconv.Atoi(utils.GetEnv(model.EnvKeyOfVNodeWorkerNum, "8"))
	if err != nil {
		zlogger.Error(err, "failed to parse VNODE_WORKER_NUM, will be set to 8 default")
		vnodeWorkerNum = 8
	}

	deployNamespace := utils.GetEnv(model.EnvKeyOfNamespace, "default")

	// Initialize controller manager
	kubeConfig := config.GetConfigOrDie()
	// TODO: should support to set from parameter
	kubeConfig.QPS = 100
	kubeConfig.Burst = 200

	zlogger.Info("start to start manager")
	ctrl.SetLogger(zlogger)
	k8sControllerManager, err := manager.New(kubeConfig, manager.Options{
		Cache:                  cache.Options{},
		HealthProbeBindAddress: ":8081",
		Metrics: server.Options{
			BindAddress: ":9090",
		},
	})
	if err != nil {
		zlogger.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	tracker.SetTracker(&tracker.DefaultTracker{})

	k8sClientSet := kubernetes.NewForConfigOrDie(kubeConfig)

	var moduleControllerServiceIP string
	var kubeletProxyEnabled bool
	if os.Getenv(model.EnvKeyOfKubeletProxyEnabled) == "true" {
		moduleControllerServiceIP, err = lookupProxyServiceIP(ctx, deployNamespace, k8sClientSet)
		if err != nil {
			log.G(ctx).Fatalf("Failed to lookup kubelet proxy service IP: %v", err)
		}
		kubeletProxyEnabled = true
	}

	// Configure and create VNode controller
	vNodeControllerConfig := vkModel.BuildVNodeControllerConfig{
		ClientID:         clientID,
		Env:              env,
		VPodType:         model.ComponentModule,
		IsCluster:        isCluster,
		WorkloadMaxLevel: workloadMaxLevel,
		VNodeWorkerNum:   vnodeWorkerNum,
		// vnode ip will fall back to the ip of the base pod if not set
		PseudoNodeIP: moduleControllerServiceIP,
	}

	moduleDeploymentController, err := module_deployment_controller.NewModuleDeploymentController(env)
	if err != nil {
		zlogger.Error(err, "unable to set up module_deployment_controller")
		return
	}

	err = moduleDeploymentController.SetupWithManager(ctx, k8sControllerManager)
	if err != nil {
		zlogger.Error(err, "unable to setup module_deployment_controller")
		return
	}

	tunnel := startTunnels(ctx, clientID, env, k8sControllerManager, moduleDeploymentController)

	vNodeController, err := vnode_controller.NewVNodeController(&vNodeControllerConfig, tunnel)
	if err != nil {
		zlogger.Error(err, "unable to set up VNodeController")
		return
	}

	err = vNodeController.SetupWithManager(ctx, k8sControllerManager)
	if err != nil {
		zlogger.Error(err, "unable to setup vnode controller")
		return
	}

	if err := k8sControllerManager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		zlogger.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := k8sControllerManager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		zlogger.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	if kubeletProxyEnabled {
		zlogger.Info("starting kubelet proxy server")
		err := kubelet_proxy.StartKubeletProxy(
			ctx,
			certFilePath,
			keyFilePath,
			":"+utils.GetEnv(model.EnvKeyOfKubeletProxyPort, DefaultKubeletHttpListenAddr),
			k8sClientSet,
		)
		if err != nil {
			zlogger.Error(err, "failed to start kubelet proxy server")
			os.Exit(1)
		}
	}

	zlogger.Info("Module controller running")
	err = k8sControllerManager.Start(signals.SetupSignalHandler())
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to start manager")
	}
}

func startTunnels(ctx context.Context, clientId string, env string, mgr manager.Manager,
	moduleDeploymentController *module_deployment_controller.ModuleDeploymentController) tunnel.Tunnel {
	zlogger := zaplogger.FromContext(ctx)
	// Initialize tunnels based on configuration
	tunnels := make([]tunnel.Tunnel, 0)

	mqttTunnelEnable := utils.GetEnv("ENABLE_MQTT_TUNNEL", "false")
	if mqttTunnelEnable == "true" {
		mqttTl := koupleless_mqtt_tunnel.NewMqttTunnel(ctx, env, mgr.GetClient(), moduleDeploymentController)
		tunnels = append(tunnels, &mqttTl)
	}

	httpTunnelEnable := utils.GetEnv("ENABLE_HTTP_TUNNEL", "false")
	if httpTunnelEnable == "true" {
		httpTunnelListenPort, err := strconv.Atoi(utils.GetEnv("HTTP_TUNNEL_LISTEN_PORT", "7777"))

		if err != nil {
			log.G(ctx).WithError(err).Error("failed to parse HTTP_TUNNEL_LISTEN_PORT, set default port 7777")
			httpTunnelListenPort = 7777
		}

		httpTl := koupleless_http_tunnel.NewHttpTunnel(ctx, env, mgr.GetClient(), moduleDeploymentController, httpTunnelListenPort)
		tunnels = append(tunnels, &httpTl)
	}

	// Start all tunnels
	successTunnelCount := 0
	startFailedCount := 0
	for _, t := range tunnels {
		err := t.Start(clientId, env)
		if err != nil {
			zlogger.Error(err, "failed to start tunnel "+t.Key())
			startFailedCount++
		} else {
			zlogger.Info("Tunnel started: " + t.Key())
			successTunnelCount++
		}
	}

	if startFailedCount > 0 {
		panic(errors.New(fmt.Sprintf("failed to start %d tunnels", startFailedCount)))
	} else if successTunnelCount == 0 {
		panic(errors.New(fmt.Sprintf("successfully started 0 tunnels")))
	}
	// we only using one tunnel for now
	return tunnels[0]
}

// lookupProxyServiceIP retrieves the ClusterIP of the kubelet proxy service in the specified namespace.
func lookupProxyServiceIP(ctx context.Context, namespace string, clientSet kubernetes.Interface) (string, error) {
	svcList, err := clientSet.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", model.LabelKeyOfKubeletProxyService, "true"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to list services in namespace %s: %w", namespace, err)
	}
	if len(svcList.Items) == 0 {
		return "", fmt.Errorf("no kubelet proxy service found in namespace %s", namespace)
	}
	if len(svcList.Items) > 1 {
		return "", fmt.Errorf("multiple kubelet proxy services deteched in namespace %s, expected only one", namespace)
	}

	firstSvc := svcList.Items[0]
	if firstSvc.Spec.ClusterIP == "" || firstSvc.Spec.ClusterIP == "None" {
		return "", fmt.Errorf("kubelet proxy service %s in namespace %s has no valid ClusterIP", firstSvc.Name, namespace)
	}
	return firstSvc.Spec.ClusterIP, nil
}
