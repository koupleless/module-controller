package model

import (
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_http_tunnel/ark_service"
)

// ArkMqttMsg is the response of mqtt message payload.
type ArkMqttMsg[T any] struct {
	PublishTimestamp int64 `json:"publishTimestamp"`
	Data             T     `json:"data"`
}

// BaseMetadata contains basic identifying information
type BaseMetadata struct {
	Identity    string `json:"identity"`
	Version     string `json:"version"`     // Version identifier
	ClusterName string `json:"clusterName"` // ClusterName of the resource communicate with base
	Name        string `json:"name"`        // Name of the base
}

// BaseStatus is the data of base heart beat.
// Contains information about the base node's status and network details
type BaseStatus struct {
	BaseMetadata  BaseMetadata `json:"baseMetadata"`  // Master business info metadata
	LocalIP       string       `json:"localIP"`       // Local IP address
	LocalHostName string       `json:"localHostName"` // Local hostname
	Port          int          `json:"port"`          // Port number for arklet service
	State         string       `json:"state"`         // Current state of the base
}

// BizOperationResponse represents the response from a business operation
type BizOperationResponse struct {
	Command    string                                       `json:"command"`    // Operation command executed
	BizName    string                                       `json:"bizName"`    // ClusterName of the business
	BizVersion string                                       `json:"bizVersion"` // Version of the business
	Response   ark_service.ArkResponse[ark.ArkResponseData] `json:"response"`   // Response from ark service
}

type BatchInstallBizResponse struct {
	Command  string                                               `json:"command"`  // Operation command executed
	Response ark_service.ArkResponse[ark.ArkBatchInstallResponse] `json:"response"` // Response from ark service
}

// QueryBaselineRequest is the request parameters of query baseline func
// Used to query baseline configuration with filters
type QueryBaselineRequest struct {
	Identity    string `json:"identity"`    // Identity base to filter by
	ClusterName string `json:"clusterName"` // ClusterName to filter by
	Version     string `json:"version"`     // Version to filter by
}

// BuildModuleDeploymentControllerConfig contains controller configuration
type BuildModuleDeploymentControllerConfig struct {
	Env string `json:"env"` // Environment setting
}

// ArkSimpleAllBizInfoData is a collection of business info data
type ArkSimpleAllBizInfoData []ArkSimpleBizInfoData

// ArkSimpleBizInfoData represents simplified business information
type ArkSimpleBizInfoData struct {
	Name              string                `json:"name"`              // Name of the biz
	Version           string                `json:"version"`           // Version of the biz
	State             string                `json:"state"`             // State of the biz
	LatestStateRecord ark.ArkBizStateRecord `json:"latestStateRecord"` // Latest state record of the biz
}
