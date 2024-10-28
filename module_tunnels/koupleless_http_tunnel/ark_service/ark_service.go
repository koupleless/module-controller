package ark_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/koupleless/virtual-kubelet/common/log"

	"github.com/go-resty/resty/v2"
)

type Service struct {
	client *resty.Client
}

func NewService() *Service {
	return &Service{client: resty.New()}
}

func (h *Service) InstallBiz(ctx context.Context, req InstallBizRequest, baseIP string, arkletPort int) (response ArkResponse, err error) {

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(req).
		Post(fmt.Sprintf("http://%s:%d/installBiz", baseIP, arkletPort))

	if err != nil {
		log.G(ctx).WithError(err).Errorf("installBiz request failed")
		return
	}

	if resp == nil {
		err = errors.New("received nil response from the server")
		log.G(ctx).WithError(err).Errorf("installBiz request failed")
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		log.G(ctx).WithError(err).Errorf("Unmarshal InstallBiz response: %s", resp.Body())
		return
	}

	return response, nil
}

func (h *Service) UninstallBiz(ctx context.Context, req UninstallBizRequest, baseIP string, arkletPort int) (response ArkResponse, err error) {

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(req).
		Post(fmt.Sprintf("http://%s:%d/uninstallBiz", baseIP, arkletPort))

	if err != nil {
		log.G(ctx).WithError(err).Errorf("uninstall request failed")
		return
	}

	if resp == nil {
		err = errors.New("received nil response from the server")
		log.G(ctx).WithError(err).Errorf("uninstall request failed")
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		log.G(ctx).WithError(err).Errorf("Unmarshal UnInstallBiz response: %s", resp.Body())
		return
	}

	return response, nil
}

func (h *Service) QueryAllBiz(ctx context.Context, baseIP string, arkletPort int) (response QueryAllBizResponse, err error) {

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(`{}`).
		Post(fmt.Sprintf("http://%s:%d/queryAllBiz", baseIP, arkletPort))

	if err != nil {
		log.G(ctx).WithError(err).Errorf("queryAllBiz request failed")
		return
	}

	if resp == nil {
		err = errors.New("received nil response from the server")
		log.G(ctx).WithError(err).Errorf("queryAllBiz request failed")
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		log.G(ctx).WithError(err).Errorf("Unmarshal QueryAllBiz response: %s", resp.Body())
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
		log.G(ctx).WithError(err).Errorf("health request failed")
		return
	}

	if resp == nil {
		err = errors.New("received nil response from the server")
		log.G(ctx).WithError(err).Errorf("health request failed")
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		log.G(ctx).WithError(err).Errorf("Unmarshal Health response: %s", resp.Body())
		return
	}

	return
}
