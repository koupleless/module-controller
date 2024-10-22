package ark_service

import "github.com/koupleless/arkctl/v1/service/ark"

type InstallBizRequest struct {
	ark.BizModel `json:",inline"`
}

type UninstallBizRequest struct {
	ark.BizModel `json:",inline"`
}

type ArkResponse struct {
	ark.ArkResponseBase
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
