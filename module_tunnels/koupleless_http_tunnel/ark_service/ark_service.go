package ark_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/zaplogger"
	"net/http"

	"github.com/go-resty/resty/v2"
)

type Service struct {
	client *resty.Client
}

func NewService() *Service {
	return &Service{client: resty.New()}
}

func (h *Service) InstallBiz(ctx context.Context, req InstallBizRequest, baseIP string, arkletPort int) (response ArkResponse[ark.ArkResponseData], err error) {
	logger := zaplogger.FromContext(ctx)

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(req).
		Post(fmt.Sprintf("http://%s:%d/installBiz", baseIP, arkletPort))

	if err != nil {
		logger.Error(err, "installBiz request failed")
		return
	}

	if resp == nil {
		err = errors.New("received nil response from the server")
		logger.Error(err, "installBiz request failed")
		return
	}

	if resp.StatusCode() != http.StatusOK {
		err = errors.New(fmt.Sprintf("response status: %d", resp.StatusCode()))
		logger.Error(err, "installBiz request failed")
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		logger.Error(err, fmt.Sprintf("Unmarshal InstallBiz response: %s", resp.Body()))
		return
	}

	return response, nil
}

func (h *Service) UninstallBiz(ctx context.Context, req UninstallBizRequest, baseIP string, arkletPort int) (response ArkResponse[ark.ArkResponseData], err error) {
	logger := zaplogger.FromContext(ctx)

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(req).
		Post(fmt.Sprintf("http://%s:%d/uninstallBiz", baseIP, arkletPort))

	if err != nil {
		logger.Error(err, "uninstall request failed")
		return
	}

	if resp == nil {
		err = errors.New("received nil response from the server")
		logger.Error(err, "uninstall request failed")
		return
	}

	if resp.StatusCode() != http.StatusOK {
		err = errors.New(fmt.Sprintf("response status: %d", resp.StatusCode()))
		logger.Error(err, "uninstall request failed")
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		logger.Error(err, fmt.Sprintf("Unmarshal UnInstallBiz response: %s", resp.Body()))
		return
	}

	return response, nil
}

func (h *Service) QueryAllBiz(ctx context.Context, baseIP string, port int) (response QueryAllBizResponse, err error) {
	logger := zaplogger.FromContext(ctx)

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(`{}`).
		Post(fmt.Sprintf("http://%s:%d/queryAllBiz", baseIP, port))

	if err != nil {
		logger.Error(err, "queryAllBiz request failed")
		return
	}

	if resp == nil {
		err = errors.New("received nil response from the server")
		logger.Error(err, "queryAllBiz request failed")
		return
	}

	if resp.StatusCode() != http.StatusOK {
		err = errors.New(fmt.Sprintf("response status: %d", resp.StatusCode()))
		logger.Error(err, "queryAllBiz request failed")
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		logger.Error(err, fmt.Sprintf("Unmarshal QueryAllBiz response: %s", resp.Body()))
		return
	}

	return
}

func (h *Service) Health(ctx context.Context, baseIP string, arkletPort int) (response HealthResponse, err error) {
	logger := zaplogger.FromContext(ctx)

	resp, err := h.client.R().
		SetContext(ctx).
		SetBody(`{}`).
		Post(fmt.Sprintf("http://%s:%d/health", baseIP, arkletPort))

	if err != nil {
		logger.Error(err, "health request failed")
		return
	}

	if resp == nil {
		err = errors.New("received nil response from the server")
		logger.Error(err, "health request failed")
		return
	}

	if resp.StatusCode() != http.StatusOK {
		err = errors.New(fmt.Sprintf("response status: %d", resp.StatusCode()))
		logger.Error(err, "health request failed")
		return
	}

	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		logger.Error(err, fmt.Sprintf("Unmarshal Health response: %s", resp.Body()))
		return
	}

	return
}
