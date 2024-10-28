package ark_service

import "github.com/koupleless/arkctl/v1/service/ark"

type InstallBizRequest struct {
	ark.BizModel `json:",inline"`
}

type UninstallBizRequest struct {
	ark.BizModel `json:",inline"`
}

type ArkResponse struct {
	// Code is the response code
	Code string `json:"code"`

	// Data is the response data
	Data ark.ArkResponseData `json:"data"`

	// Message is the error message
	Message string `json:"message"`

	// ErrorStackTrace is the error stack trace
	ErrorStackTrace string `json:"errorStackTrace"`

	BaseID string `json:"baseID"`
}

type QueryAllBizResponse struct {
	ark.GenericArkResponseBase[[]ark.ArkBizInfo]
	BaseID string `json:"baseID"`
}

type HealthResponse struct {
	ark.GenericArkResponseBase[ark.HealthInfo]
	BaseID string `json:"baseID"`
}
