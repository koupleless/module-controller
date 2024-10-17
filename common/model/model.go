package model

import (
	"github.com/koupleless/arkctl/v1/service/ark"
)

// ArkMqttMsg is the response of mqtt message payload.
type ArkMqttMsg[T any] struct {
	PublishTimestamp int64 `json:"publishTimestamp"`
	Data             T     `json:"data"`
}

type Metadata struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// HeartBeatData is the data of base heart beat.
type HeartBeatData struct {
	State         string      `json:"state"`
	MasterBizInfo Metadata    `json:"masterBizInfo"`
	NetworkInfo   NetworkInfo `json:"networkInfo"`
}

type NetworkInfo struct {
	LocalIP       string `json:"localIP"`
	LocalHostName string `json:"localHostName"`
	ArkletPort    int    `json:"arkletPort"`
}

type BizOperationResponse struct {
	Command    string              `json:"command"`
	BizName    string              `json:"bizName"`
	BizVersion string              `json:"bizVersion"`
	Response   ark.ArkResponseBase `json:"response"`
}

// QueryBaselineRequest is the request parameters of query baseline func
type QueryBaselineRequest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	CustomLabels map[string]string `json:"customLabels"`
}

type BuildModuleDeploymentControllerConfig struct {
	Env string `json:"env"`
}

type ArkSimpleAllBizInfoData []ArkSimpleBizInfoData

type ArkSimpleBizInfoData []string
