package module_deployment_controller

import "testing"

func TestDeploymentHandler(t *testing.T) {
	c := &ModuleDeploymentController{}

	c.deploymentAddHandler(nil)
	c.deploymentUpdateHandler(nil, nil)
	c.deploymentDeleteHandler(nil)
}
