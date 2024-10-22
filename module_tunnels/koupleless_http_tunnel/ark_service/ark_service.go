package ark_service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-resty/resty/v2"
)

type Service struct {
	client *resty.Client
}

func NewService() *Service {
	return &Service{client: resty.New()}
}

func (h *Service) InstallBiz(ctx context.Context, req InstallBizRequest, baseIP string, arkletPort int) (response *ArkResponse, err error) {

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(req).
		Post(fmt.Sprintf("http://%s:%d/installBiz", baseIP, arkletPort))

	if err != nil {
		return
	}

	var installResponse *ArkResponse

	if err = json.Unmarshal(resp.Body(), installResponse); err != nil {
		return
	}

	return installResponse, nil
}

func (h *Service) UninstallBiz(ctx context.Context, req UninstallBizRequest, baseIP string, arkletPort int) (response *ArkResponse, err error) {

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(req).
		Post(fmt.Sprintf("http://%s:%d/uninstallBiz", baseIP, arkletPort))

	if err != nil {
		return
	}

	var uninstallResponse *ArkResponse

	if err = json.Unmarshal(resp.Body(), uninstallResponse); err != nil {
		return
	}

	return uninstallResponse, nil
}

func (h *Service) QueryAllBiz(ctx context.Context, baseIP string, arkletPort int) (response QueryAllBizResponse, err error) {

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(`{}`).
		Post(fmt.Sprintf("http://%s:%d/queryAllBiz", baseIP, arkletPort))

	if err != nil {
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		return
	}

	return
}

func (h *Service) Health(ctx context.Context, baseIP string, arkletPort int) (response HealthResponse, err error) {

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(`{}`).
		Post(fmt.Sprintf("http://%s:%d/health", baseIP, arkletPort))

	if err != nil {
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		return
	}

	return
}
