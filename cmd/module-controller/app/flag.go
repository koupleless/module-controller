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
	"flag"
	"github.com/spf13/pflag"
	klog "k8s.io/klog/v2"
)

func installFlags(flags *pflag.FlagSet, c *Opts) {
	flags.StringVar(&c.KubeConfigPath, "kubeconfig", c.KubeConfigPath, "kube config file to use for connecting to the Kubernetes API server")
	flags.StringVar(&c.OperatingSystem, "os", c.OperatingSystem, "Operating System (Linux/Windows)")

	flags.IntVar(&c.PodSyncWorkers, "pod-sync-workers", c.PodSyncWorkers, `set the number of pod synchronization workers`)

	flags.DurationVar(&c.InformerResyncPeriod, "full-resync-period", c.InformerResyncPeriod, "how often to perform a full resync of pods between kubernetes and the provider")

	flags.BoolVar(&c.EnableMqttTunnel, "enable-mqtt-tunnel", c.EnableMqttTunnel, "mqtt tunnel enable flag")
	flags.BoolVar(&c.EnableTracker, "enable-tracker", c.EnableTracker, "default tracker enable flag")
	flags.BoolVar(&c.EnablePrometheus, "enable-prometheus", c.EnablePrometheus, "prometheus enable flag")
	flags.BoolVar(&c.EnableInspection, "enable-inspection", c.EnableInspection, "inspection enable flag")
	flags.IntVar(&c.PrometheusPort, "prometheus-port", c.PrometheusPort, `set the port of prometheus metrics endpoint`)

	flags.StringVar(&c.Env, "env", c.Env, "env config")

	flagset := flag.NewFlagSet("klog", flag.PanicOnError)
	klog.InitFlags(flagset)
	flagset.VisitAll(func(f *flag.Flag) {
		f.Name = "klog." + f.Name
		flags.AddGoFlag(f)
	})
}
