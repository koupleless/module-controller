package module_deployment_controller

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDeploymentHandler(t *testing.T) {
	c := &ModuleDeploymentController{
		runtimeStorage: NewRuntimeInfoStore(),
		updateToken:    make(chan interface{}),
	}

	ctx := context.Background()
	c.deploymentAddHandler(ctx, nil)

	assert.Equal(t, 0, len(c.runtimeStorage.peerDeploymentMap))

	c.deploymentUpdateHandler(ctx, nil, nil)

	assert.Equal(t, 0, len(c.runtimeStorage.peerDeploymentMap))
}
