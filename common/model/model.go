package model

import (
	"github.com/koupleless/module_controller/module_tunnels/koupleless_http_tunnel/ark_service"
)

// ArkMqttMsg is the response of mqtt message payload.
type ArkMqttMsg[T any] struct {
	PublishTimestamp int64 `json:"publishTimestamp"`
	Data             T     `json:"data"`
}

// Metadata contains basic identifying information
type Metadata struct {
	Name    string `json:"name"`    // Name of the resource
	Version string `json:"version"` // Version identifier
}

// HeartBeatData is the data of base heart beat.
// Contains information about the base node's status and network details
type HeartBeatData struct {
	BaseID        string      `json:"baseID"`        // Unique identifier for the base
	State         string      `json:"state"`         // Current state of the base
	MasterBizInfo Metadata    `json:"masterBizInfo"` // Master business info metadata
	NetworkInfo   NetworkInfo `json:"networkInfo"`   // Network configuration details
}

// NetworkInfo contains network-related configuration
type NetworkInfo struct {
	LocalIP       string `json:"localIP"`       // Local IP address
	LocalHostName string `json:"localHostName"` // Local hostname
	ArkletPort    int    `json:"arkletPort"`    // Port number for arklet service
}

// BizOperationResponse represents the response from a business operation
type BizOperationResponse struct {
	Command    string                  `json:"command"`    // Operation command executed
	BizName    string                  `json:"bizName"`    // Name of the business
	BizVersion string                  `json:"bizVersion"` // Version of the business
	Response   ark_service.ArkResponse `json:"response"`   // Response from ark service
}

// QueryBaselineRequest is the request parameters of query baseline func
// Used to query baseline configuration with filters
type QueryBaselineRequest struct {
	Name         string            `json:"name"`         // Name to filter by
	Version      string            `json:"version"`      // Version to filter by
	CustomLabels map[string]string `json:"customLabels"` // Additional label filters
}

// BuildModuleDeploymentControllerConfig contains controller configuration
type BuildModuleDeploymentControllerConfig struct {
	Env string `json:"env"` // Environment setting
}

// ArkSimpleAllBizInfoData is a collection of business info data
type ArkSimpleAllBizInfoData []ArkSimpleBizInfoData

// ArkSimpleBizInfoData represents simplified business information
type ArkSimpleBizInfoData []string
