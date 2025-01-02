package ark_service

import "github.com/koupleless/arkctl/v1/service/ark"

// InstallBizRequest represents a request to install a business service
type InstallBizRequest struct {
	ark.BizModel `json:",inline"`
}

// UninstallBizRequest represents a request to uninstall a business service
type UninstallBizRequest struct {
	ark.BizModel `json:",inline"`
}

// ArkResponse is a generic response structure for Ark service operations
type ArkResponse[T any] struct {
	// Code is the response code indicating the outcome of the operation
	Code string `json:"code"`

	// Data is the response data, which can vary depending on the operation
	Data T `json:"data"`

	// Message is the error message in case of an error
	Message string `json:"message"`

	// ErrorStackTrace is the error stack trace in case of an error
	ErrorStackTrace string `json:"errorStackTrace"`

	// BaseIdentity is a unique identifier for the base service
	BaseIdentity string `json:"baseIdentity"`
}

// QueryAllBizResponse represents the response for querying all business services
type QueryAllBizResponse struct {
	ark.GenericArkResponseBase[[]ark.ArkBizInfo]
}

// HealthResponse represents the response for health checks
type HealthResponse struct {
	ark.GenericArkResponseBase[ark.HealthInfo]
}
