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
	"github.com/google/uuid"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/controller/module_deployment_controller"
	"github.com/koupleless/module_controller/module_tunnels"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_mqtt_tunnel"
	"github.com/koupleless/module_controller/report_server"
	"github.com/koupleless/virtual-kubelet/common/log"
	logruslogger "github.com/koupleless/virtual-kubelet/common/log/logrus"
	"github.com/koupleless/virtual-kubelet/common/trace"
	"github.com/koupleless/virtual-kubelet/common/trace/opencensus"
	"github.com/koupleless/virtual-kubelet/common/tracker"
	"github.com/koupleless/virtual-kubelet/common/utils"
	"github.com/koupleless/virtual-kubelet/controller/vnode_controller"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"syscall"
)

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

	clientID := uuid.New().String()
	env := utils.GetEnv("ENV", "dev")

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"clientID": clientID,
		"env":      env,
	}))

	kubeConfig := config.GetConfigOrDie()
	mgr, err := manager.New(kubeConfig, manager.Options{
		Cache: cache.Options{},
		Metrics: server.Options{
			BindAddress: ":9090",
		},
		PprofBindAddress: ":9091",
	})

	if err != nil {
		log.G(ctx).Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	tracker.SetTracker(&tracker.DefaultTracker{})

	tl := &koupleless_mqtt_tunnel.MqttTunnel{
		Cache:  mgr.GetCache(),
		Client: mgr.GetClient(),
	}

	rcc := vkModel.BuildVNodeControllerConfig{
		ClientID:     clientID,
		Env:          env,
		VPodIdentity: model.ComponentModule,
	}

	vc, err := vnode_controller.NewVNodeController(&rcc, []tunnel.Tunnel{
		tl,
	})
	if err != nil {
		log.G(ctx).Error(err, "unable to set up VNodeController")
		return
	}

	err = vc.SetupWithManager(ctx, mgr)
	if err != nil {
		log.G(ctx).WithError(err).Error("unable to setup vnode controller")
		return
	}

	enableModuleDeploymentController := utils.GetEnv("ENABLE_MODULE_DEPLOYMENT_CONTROLLER", "false")

	if enableModuleDeploymentController == "true" {
		mdc, err := module_deployment_controller.NewModuleDeploymentController(env, []module_tunnels.ModuleTunnel{
			tl,
		})
		if err != nil {
			log.G(ctx).Error(err, "unable to set up module_deployment_controller")
			return
		}

		err = mdc.SetupWithManager(ctx, mgr)
		if err != nil {
			log.G(ctx).WithError(err).Error("unable to setup vnode controller")
			return
		}
	}

	err = tl.Start(ctx, clientID, env)
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to start tunnel", tl.Key())
	} else {
		log.G(ctx).Info("Tunnel started: ", tl.Key())
	}

	log.G(ctx).Info("Module controller running")

	err = mgr.Start(signals.SetupSignalHandler())
	if err != nil {
		log.G(ctx).WithError(err).Error("failed to start manager")
	}
}
