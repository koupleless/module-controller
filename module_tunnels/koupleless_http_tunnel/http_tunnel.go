package koupleless_http_tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"errors"

	"github.com/koupleless/module_controller/common/zaplogger"
	"github.com/koupleless/module_controller/controller/module_deployment_controller"
	utils2 "github.com/koupleless/virtual-kubelet/common/utils"

	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/common/utils"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_http_tunnel/ark_service"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ tunnel.Tunnel = &HttpTunnel{}

type HttpTunnel struct {
	ctx context.Context
	sync.Mutex

	arkService *ark_service.Service

	kubeClient client.Client
	env        string
	port       int

	ready bool

	onBaseDiscovered         tunnel.OnBaseDiscovered
	onHealthDataArrived      tunnel.OnBaseStatusArrived
	onQueryAllBizDataArrived tunnel.OnAllBizStatusArrived
	onOneBizDataArrived      tunnel.OnSingleBizStatusArrived

	onlineNode map[string]bool

	nodeIdToBaseStatusMap map[string]model.BaseStatus

	queryAllBizLock         sync.Mutex
	queryAllBizDataOutdated bool

	moduleDeploymentController *module_deployment_controller.ModuleDeploymentController
}

func NewHttpTunnel(ctx context.Context, env string, kubeClient client.Client, moduleDeploymentController *module_deployment_controller.ModuleDeploymentController, port int) HttpTunnel {
	return HttpTunnel{
		ctx:                        ctx,
		env:                        env,
		kubeClient:                 kubeClient,
		moduleDeploymentController: moduleDeploymentController,
		port:                       port,
	}
}

// Ready returns the current status of the tunnel
func (h *HttpTunnel) Ready() bool {
	return h.ready
}

// GetBizUniqueKey returns a unique key for the container
func (h *HttpTunnel) GetBizUniqueKey(container *corev1.Container) string {
	return utils.GetBizIdentity(container.Name, utils.GetBizVersionFromContainer(container))
}

// RegisterNode is called when a new node starts
func (h *HttpTunnel) RegisterNode(initData vkModel.NodeInfo) error {
	h.Lock()
	defer h.Unlock()

	// check base network info, if not exist, extract from initData
	nodeID := utils2.ExtractNodeIDFromNodeName(initData.Metadata.Name)
	_, has := h.nodeIdToBaseStatusMap[nodeID]
	if !has {
		h.nodeIdToBaseStatusMap[nodeID] = utils.ConvertBaseStatusFromNodeInfo(initData)
	}
	return nil
}

// UnRegisterNode is called when a node stops
func (h *HttpTunnel) UnRegisterNode(nodeName string) {
	h.Lock()
	defer h.Unlock()
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	delete(h.nodeIdToBaseStatusMap, nodeID)
}

// OnNodeNotReady is called when a node is not ready
func (h *HttpTunnel) OnNodeNotReady(nodeName string) {
	utils.OnBaseUnreachable(h.ctx, nodeName, h.kubeClient)
}

// Key returns the key of the tunnel
func (h *HttpTunnel) Key() string {
	return "http_tunnel_provider"
}

// RegisterCallback registers the callback functions for the tunnel
func (h *HttpTunnel) RegisterCallback(onBaseDiscovered tunnel.OnBaseDiscovered, onHealthDataArrived tunnel.OnBaseStatusArrived, onQueryAllBizDataArrived tunnel.OnAllBizStatusArrived, onOneBizDataArrived tunnel.OnSingleBizStatusArrived) {
	h.onBaseDiscovered = onBaseDiscovered

	h.onHealthDataArrived = onHealthDataArrived

	h.onQueryAllBizDataArrived = onQueryAllBizDataArrived

	h.onOneBizDataArrived = onOneBizDataArrived
}

// Start starts the tunnel
func (h *HttpTunnel) Start(clientID, env string) (err error) {
	h.onlineNode = make(map[string]bool)
	h.nodeIdToBaseStatusMap = make(map[string]model.BaseStatus)
	h.env = env

	h.arkService = ark_service.NewService()

	h.ready = true

	// add base discovery
	go h.startBaseDiscovery(h.ctx)

	return
}

// startBaseDiscovery starts the base discovery server
func (h *HttpTunnel) startBaseDiscovery(ctx context.Context) {
	logger := zaplogger.FromContext(ctx)
	// start a simple http server to handle base discovery, exit when ctx done
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", h.port),
		Handler: mux,
	}

	// handle heartbeat post request
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		heartbeatData := model.BaseStatus{}
		err := json.NewDecoder(r.Body).Decode(&heartbeatData)
		if err != nil {
			logger.Error(err, "failed to unmarshal heartbeat data")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		h.Lock()
		h.nodeIdToBaseStatusMap[heartbeatData.BaseMetadata.Identity] = heartbeatData
		h.Unlock()
		h.onBaseDiscovered(utils.ConvertBaseStatusToNodeInfo(heartbeatData, h.env))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("SUCCESS"))
	})

	mux.HandleFunc("/queryBaseline", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		baseMetadata := model.BaseMetadata{}
		err := json.NewDecoder(r.Body).Decode(&baseMetadata)
		if err != nil {
			logger.Error(err, "failed to unmarshal baseMetadata data")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		baselineBizs := make([]ark.BizModel, 0)
		baseline := h.moduleDeploymentController.QueryContainerBaseline(h.ctx, utils.ConvertBaseMetadataToBaselineQuery(baseMetadata))
		for _, container := range baseline {
			baselineBizs = append(baselineBizs, utils.TranslateCoreV1ContainerToBizModel(&container))
		}

		jsonData, _ := json.Marshal(baselineBizs)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData)
	})

	go func() {
		if err := server.ListenAndServe(); err != nil {
			logger.Error(err, "error starting http base discovery server")
		}
	}()

	logger.Info(fmt.Sprintf("http base discovery server started, listening on port %d", h.port))

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error(err, "error shutting down http server")
		}
	}()
	<-ctx.Done()
}

// healthMsgCallback is the callback function for health messages
func (h *HttpTunnel) healthMsgCallback(nodeID string, data ark_service.HealthResponse) {
	if data.Code != "SUCCESS" {
		return
	}
	if h.onHealthDataArrived != nil {
		h.onHealthDataArrived(utils2.FormatNodeName(nodeID, h.env), utils.ConvertHealthDataToNodeStatus(data.Data.HealthData))
	}
}

// allBizMsgCallback is the callback function for all business messages
func (h *HttpTunnel) allBizMsgCallback(nodeID string, data ark_service.QueryAllBizResponse) {
	if data.Code != "SUCCESS" {
		return
	}
	if h.onQueryAllBizDataArrived != nil {
		h.onQueryAllBizDataArrived(utils2.FormatNodeName(nodeID, h.env), utils.TranslateBizInfosToContainerStatuses(data.GenericArkResponseBase.Data, time.Now().UnixMilli()))
	}
}

// bizOperationResponseCallback is the callback function for business operation responses
func (httpTunnel *HttpTunnel) bizOperationResponseCallback(nodeID string, data model.BizOperationResponse) {
	logger := zaplogger.FromContext(httpTunnel.ctx)
	nodeName := utils2.FormatNodeName(nodeID, httpTunnel.env)
	if data.Response.Code == "SUCCESS" {
		if data.Command == model.CommandInstallBiz {
			logger.Info("install biz success: ", data.BizName, data.BizVersion)
			httpTunnel.onOneBizDataArrived(nodeName, vkModel.BizStatusData{
				Key:        utils.GetBizIdentity(data.BizName, data.BizVersion),
				Name:       data.BizName,
				State:      string(vkModel.BizStateActivated),
				ChangeTime: time.Now(),
				Reason:     fmt.Sprintf("%s:%s %s succeed", data.BizName, data.BizVersion, data.Command),
				Message:    data.Response.Data.Message,
			})
			return
		} else if data.Command == model.CommandUnInstallBiz {
			logger.Info("uninstall biz success: ", data.BizName, data.BizVersion)
			return
		} else {
			logger.Error(nil, fmt.Sprintf("biz operation failed: %s:%s %s\n%s\n%s\n%s", data.BizName, data.BizVersion, data.Command, data.Response.Message, data.Response.ErrorStackTrace, data.Response.Data.Message))
		}
	}
	httpTunnel.onOneBizDataArrived(nodeName, vkModel.BizStatusData{
		Key:  utils.GetBizIdentity(data.BizName, data.BizVersion),
		Name: data.BizName,
		// fille PodKey when using
		// PodKey:     vkModel.PodKeyAll,
		State:      string(vkModel.BizStateBroken),
		ChangeTime: time.Now(),
		Reason:     data.Response.Code,
		Message:    data.Response.Message,
	})
}

// FetchHealthData fetches health data from the node
func (h *HttpTunnel) FetchHealthData(nodeName string) error {
	h.Lock()
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	baseStatus, ok := h.nodeIdToBaseStatusMap[nodeID]
	h.Unlock()
	if !ok {
		return errors.New("network info not found")
	}

	healthData, err := h.arkService.Health(h.ctx, baseStatus.LocalIP, baseStatus.Port)

	if err != nil {
		return err
	}

	h.healthMsgCallback(nodeID, healthData)

	return nil
}

// QueryAllBizStatusData queries all container status data from the node
func (h *HttpTunnel) QueryAllBizStatusData(nodeName string) error {
	// add a signal to check
	success := h.queryAllBizLock.TryLock()
	if !success {
		// a query is processing
		h.queryAllBizDataOutdated = true
		return nil
	}
	h.queryAllBizDataOutdated = false
	defer func() {
		h.queryAllBizLock.Unlock()
		if h.queryAllBizDataOutdated {
			go h.QueryAllBizStatusData(nodeName)
		}
	}()

	h.Lock()
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	baseStatus, ok := h.nodeIdToBaseStatusMap[nodeID]
	h.Unlock()
	if !ok {
		return errors.New("network info not found")
	}

	allBizData, err := h.arkService.QueryAllBiz(h.ctx, baseStatus.LocalIP, baseStatus.Port)

	if err != nil {
		return err
	}

	h.allBizMsgCallback(nodeID, allBizData)

	return nil
}

// StartBiz starts a container on the node
func (h *HttpTunnel) StartBiz(nodeName, podKey string, container *corev1.Container) error {

	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	h.Lock()
	baseStatus, ok := h.nodeIdToBaseStatusMap[nodeID]
	h.Unlock()
	if !ok {
		return errors.New("network info not found")
	}

	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	bizModel.BizModelVersion = podKey
	logger := zaplogger.FromContext(h.ctx).WithValues("bizName", bizModel.BizName, "bizVersion", bizModel.BizVersion, "bizModelVersion", bizModel.BizModelVersion)
	logger.Info("InstallModule")

	// install current version
	bizOperationResponse := model.BizOperationResponse{
		Command:         model.CommandInstallBiz,
		BizName:         bizModel.BizName,
		BizVersion:      bizModel.BizVersion,
		BizModelVersion: podKey,
	}

	response, err := h.arkService.InstallBiz(h.ctx, ark_service.InstallBizRequest{
		BizModel: bizModel,
	}, baseStatus.LocalIP, baseStatus.Port)

	bizOperationResponse.Response = response

	h.bizOperationResponseCallback(nodeID, bizOperationResponse)

	return err
}

// StopBiz shuts down a container on the node
func (h *HttpTunnel) StopBiz(nodeName, podKey string, container *corev1.Container) error {
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	h.Lock()
	baseStatus, ok := h.nodeIdToBaseStatusMap[nodeID]
	h.Unlock()
	if !ok {
		return errors.New("network info not found")
	}

	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	bizModel.BizModelVersion = podKey
	logger := zaplogger.FromContext(h.ctx).WithValues("bizName", bizModel.BizName, "bizVersion", bizModel.BizVersion, "bizModelVersion", bizModel.BizModelVersion)
	logger.Info("UninstallModule")

	bizOperationResponse := model.BizOperationResponse{
		Command:         model.CommandUnInstallBiz,
		BizName:         bizModel.BizName,
		BizVersion:      bizModel.BizVersion,
		BizModelVersion: podKey,
	}

	response, err := h.arkService.UninstallBiz(h.ctx, ark_service.UninstallBizRequest{
		BizModel: bizModel,
	}, baseStatus.LocalIP, baseStatus.Port)

	bizOperationResponse.Response = response

	h.bizOperationResponseCallback(nodeID, bizOperationResponse)

	return err
}
