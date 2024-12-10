package module_deployment_controller

import (
	"context"
	"testing"
)

func TestDeploymentHandler(t *testing.T) {
	c := &ModuleDeploymentController{
		updateToken: make(chan interface{}),
	}

	ctx := context.Background()
	c.deploymentAddHandler(ctx, nil)
	c.deploymentUpdateHandler(ctx, nil, nil)
}
