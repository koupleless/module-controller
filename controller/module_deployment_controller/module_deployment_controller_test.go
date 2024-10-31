package module_deployment_controller

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDeploymentHandler(t *testing.T) {
	c := &ModuleDeploymentController{
		runtimeStorage: NewRuntimeInfoStore(),
		updateToken:    make(chan interface{}),
	}

	c.deploymentAddHandler(nil)

	assert.Equal(t, 0, len(c.runtimeStorage.peerDeploymentMap))

	c.deploymentUpdateHandler(nil, nil)

	assert.Equal(t, 0, len(c.runtimeStorage.peerDeploymentMap))

	c.deploymentDeleteHandler(nil)

	assert.Equal(t, 0, len(c.runtimeStorage.peerDeploymentMap))
}
