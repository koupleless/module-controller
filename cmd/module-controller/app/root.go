// Copyright © 2017 The virtual-kubelet authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"github.com/google/uuid"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_mqtt_tunnel"
	"github.com/koupleless/virtual-kubelet/common/log"
	"github.com/koupleless/virtual-kubelet/common/tracker"
	"github.com/koupleless/virtual-kubelet/controller/vnode_controller"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	"github.com/spf13/cobra"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// NewCommand creates a new top-level command.
// This command is used to start the virtual-kubelet daemon
func NewCommand(ctx context.Context, c Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use: "run",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModuleControllerCommand(ctx, c)
		},
	}

	installFlags(cmd.Flags(), &c)
	return cmd
}

func runModuleControllerCommand(ctx context.Context, c Opts) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	clientID := uuid.New().String()

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"operatingSystem": c.OperatingSystem,
		"clientID":        clientID,
		"env":             c.Env,
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

	if c.EnableTracker {
		tracker.SetTracker(&tracker.DefaultTracker{})
	}

	tunnels := []tunnel.Tunnel{
		&koupleless_mqtt_tunnel.MqttTunnel{},
	}

	rcc := vkModel.BuildVNodeControllerConfig{
		ClientID:     clientID,
		Env:          c.Env,
		VPodIdentity: model.ComponentModule,
	}

	vc, err := vnode_controller.NewVNodeController(&rcc, tunnels)
	if err != nil {
		return err
	}

	err = vc.SetupWithManager(ctx, mgr)
	if err != nil {
		return err
	}

	for _, t := range tunnels {
		err = t.Start(ctx, clientID, c.Env)
		if err != nil {
			log.G(ctx).WithError(err).Error("failed to start tunnel", t.Key())
		} else {
			log.G(ctx).Info("Tunnel started: ", t.Key())
		}
	}

	log.G(ctx).Info("Module controller running")

	return mgr.Start(signals.SetupSignalHandler())
}
