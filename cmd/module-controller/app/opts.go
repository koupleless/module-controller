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
	"github.com/koupleless/virtual-kubelet/common/utils"
	"os"
	"strconv"
	"time"
)

// Defaults for root command options
const (
	DefaultOperatingSystem      = "linux"
	DefaultInformerResyncPeriod = 1 * time.Minute
	DefaultPodSyncWorkers       = 4
	DefaultENV                  = "dev"
	DefaultPrometheusPort       = "9090"
)

type Opts struct {
	// Path to the kubeconfig to use to connect to the Kubernetes API server.
	KubeConfigPath string
	// Operating system to run pods for
	OperatingSystem string
	// Env is the env of running
	Env string
	// EnableTracker is the flag of using default tracker
	EnableTracker bool
	// EnableInspection is the flag of using inspection
	EnableInspection bool
	// EnablePrometheus is the flag of using prometheus
	EnablePrometheus bool
	// PrometheusPort is the port of prometheus metric endpoint
	PrometheusPort int

	// Number of workers to use to handle pod notifications
	PodSyncWorkers       int
	InformerResyncPeriod time.Duration

	// Tunnel config
	EnableMqttTunnel bool

	Version string
}

// SetDefaultOpts sets default options for unset values on the passed in option struct.
// Fields tht are already set will not be modified.
func SetDefaultOpts(c *Opts) error {
	if c.OperatingSystem == "" {
		c.OperatingSystem = DefaultOperatingSystem
	}

	if c.InformerResyncPeriod == 0 {
		c.InformerResyncPeriod = DefaultInformerResyncPeriod
	}

	if c.PodSyncWorkers == 0 {
		c.PodSyncWorkers = DefaultPodSyncWorkers
	}

	if c.KubeConfigPath == "" {
		c.KubeConfigPath = os.Getenv("KUBE_CONFIG_PATH")
	}

	if c.PrometheusPort == 0 {
		portStr := utils.GetEnv("PROMETHEUS_PORT", DefaultPrometheusPort)
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return err
		}
		c.PrometheusPort = port
	}

	if c.Env == "" {
		c.Env = utils.GetEnv("ENV", DefaultENV)
	}

	return nil
}
