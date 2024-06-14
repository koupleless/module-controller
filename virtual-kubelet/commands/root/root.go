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

package root

import (
	"context"
	"crypto/tls"
	"net/http"
	"runtime"

	podlet "github.com/koupleless/module-controller/virtual-kubelet/java/pod/let"
	podnode "github.com/koupleless/module-controller/virtual-kubelet/java/pod/node"
	"github.com/spf13/cobra"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
)

// NewCommand creates a new top-level command.
// This command is used to start the virtual-kubelet daemon
func NewCommand(ctx context.Context, c Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "run provides a virtual kubelet interface for your kubernetes cluster.",
		Long: `run implements the Kubelet interface with a pluggable
backend implementation allowing users to create kubernetes nodes without running the kubelet.
This allows users to schedule kubernetes workloads on nodes that aren't running Kubernetes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(ctx, c)
		},
	}

	installFlags(cmd.Flags(), &c)
	return cmd
}

func runRootCommand(ctx context.Context, c Opts) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if c.PodSyncWorkers == 0 {
		return errdefs.InvalidInput("pod sync workers must be greater than 0")
	}

	// Ensure API client.
	clientSet, err := nodeutil.ClientsetFromEnv(c.KubeConfigPath)
	if err != nil {
		return err
	}

	// Set-up the node provider.
	mux := http.NewServeMux()
	apiConfig, err := getAPIConfig(c)
	if err != nil {
		return err
	}

	var provider *podlet.BaseProvider
	cm, err := nodeutil.NewNode(
		c.NodeName,
		func(config nodeutil.ProviderConfig) (nodeutil.Provider, node.NodeProvider, error) {
			nodeProvider := podnode.NewVirtualKubeletNode()
			// initialize node spec on bootstrap
			nodeProvider.Register(ctx, config.Node)
			provider = podlet.NewBaseProvider(config.Node.Namespace)
			return provider, nodeProvider, nil
		},
		func(cfg *nodeutil.NodeConfig) error {
			cfg.KubeconfigPath = c.KubeConfigPath
			cfg.Handler = mux
			cfg.InformerResyncPeriod = c.InformerResyncPeriod
			cfg.NodeSpec.Status.NodeInfo.Architecture = runtime.GOARCH
			cfg.NodeSpec.Status.NodeInfo.OperatingSystem = c.OperatingSystem
			cfg.HTTPListenAddr = apiConfig.Addr
			cfg.StreamCreationTimeout = apiConfig.StreamCreationTimeout
			cfg.StreamIdleTimeout = apiConfig.StreamIdleTimeout
			cfg.DebugHTTP = true

			cfg.NumWorkers = c.PodSyncWorkers
			return nil
		},
		nodeutil.WithClient(clientSet),
		//setAuth(c.NodeName, apiConfig),
		//nodeutil.WithTLSConfig(
		//	nodeutil.WithKeyPairFromPath(apiConfig.CertPath, apiConfig.KeyPath),
		//maybeCA(apiConfig.CACertPath),
		//),
		nodeutil.AttachProviderRoutes(mux),
	)
	if err != nil {
		return err
	}

	if err := setupTracing(ctx, c); err != nil {
		return err
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"provider":         c.Provider,
		"operatingSystem":  c.OperatingSystem,
		"node":             c.NodeName,
		"watchedNamespace": c.KubeNamespace,
	}))

	go provider.Run(ctx)
	go cm.Run(ctx) //nolint:errcheck

	defer func() {
		log.G(ctx).Debug("Waiting for controllers to be done")
		cancel()
		<-cm.Done()
	}()

	log.G(ctx).Info("Waiting for controller to be ready")
	if err := cm.WaitReady(ctx, c.StartupTimeout); err != nil {
		return err
	}

	log.G(ctx).Info("Ready")

	select {
	case <-ctx.Done():
	case <-cm.Done():
		return cm.Err()
	}
	return nil
}

func setAuth(node string, apiCfg *apiServerConfig) nodeutil.NodeOpt {
	if apiCfg.CACertPath == "" {
		return func(cfg *nodeutil.NodeConfig) error {
			cfg.Handler = api.InstrumentHandler(nodeutil.WithAuth(nodeutil.NoAuth(), cfg.Handler))
			return nil
		}
	}

	return func(cfg *nodeutil.NodeConfig) error {
		auth, err := nodeutil.WebhookAuth(cfg.Client, node, func(cfg *nodeutil.WebhookAuthConfig) error {
			var err error
			cfg.AuthnConfig.ClientCertificateCAContentProvider, err = dynamiccertificates.NewDynamicCAContentFromFile("ca-cert-bundle", apiCfg.CACertPath)
			return err
		})
		if err != nil {
			return err
		}
		cfg.Handler = api.InstrumentHandler(nodeutil.WithAuth(auth, cfg.Handler))
		return nil
	}
}

func maybeCA(p string) func(*tls.Config) error {
	if p == "" {
		return func(*tls.Config) error { return nil }
	}
	return nodeutil.WithCAFromPath(p)
}
