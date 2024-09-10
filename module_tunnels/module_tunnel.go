package module_tunnels

import (
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	v1 "k8s.io/api/core/v1"
)

type QueryBaseline func(model.QueryBaselineRequest) []v1.Container

type ModuleTunnel interface {
	tunnel.Tunnel

	RegisterQuery(queryBaseline QueryBaseline)
}
