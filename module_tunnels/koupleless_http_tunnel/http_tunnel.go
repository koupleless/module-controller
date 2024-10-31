package koupleless_http_tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"errors"

	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/common/utils"
	"github.com/koupleless/module_controller/module_tunnels"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_http_tunnel/ark_service"
	"github.com/koupleless/virtual-kubelet/common/log"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ module_tunnels.ModuleTunnel = &HttpTunnel{}

type HttpTunnel struct {
	sync.Mutex

	arkService *ark_service.Service

	cache.Cache
	client.Client
	env  string
	Port int

	ready bool

	onBaseDiscovered         tunnel.OnNodeDiscovered
	onHealthDataArrived      tunnel.OnNodeStatusDataArrived
	onQueryAllBizDataArrived tunnel.OnQueryAllContainerStatusDataArrived
	onOneBizDataArrived      tunnel.OnSingleContainerStatusChanged
	queryBaseline            module_tunnels.QueryBaseline

	onlineNode map[string]bool

	nodeNetworkInfoOfNodeID map[string]model.NetworkInfo

	queryAllBizLock         sync.Mutex
	queryAllBizDataOutdated bool
}

// Ready returns the current status of the tunnel
func (h *HttpTunnel) Ready() bool {
	return h.ready
}

// GetContainerUniqueKey returns a unique key for the container
func (h *HttpTunnel) GetContainerUniqueKey(_ string, container *corev1.Container) string {
	return utils.GetBizIdentity(container.Name, utils.GetBizVersionFromContainer(container))
}

// OnNodeStart is called when a new node starts
func (h *HttpTunnel) OnNodeStart(ctx context.Context, nodeID string, initData vkModel.NodeInfo) {
	h.Lock()
	defer h.Unlock()

	// check base network info, if not exist, extract from initData
	_, has := h.nodeNetworkInfoOfNodeID[nodeID]
	if !has {
		h.nodeNetworkInfoOfNodeID[nodeID] = utils.ExtractNetworkInfoFromNodeInfoData(initData)
	}
}

// OnNodeStop is called when a node stops
func (h *HttpTunnel) OnNodeStop(ctx context.Context, nodeID string) {
	h.Lock()
	defer h.Unlock()
	delete(h.nodeNetworkInfoOfNodeID, nodeID)
}

// OnNodeNotReady is called when a node is not ready
func (h *HttpTunnel) OnNodeNotReady(ctx context.Context, info vkModel.UnreachableNodeInfo) {
	utils.OnBaseUnreachable(ctx, info, h.env, h.Client)
}

// Key returns the key of the tunnel
func (h *HttpTunnel) Key() string {
	return "http_tunnel_provider"
}

// RegisterCallback registers the callback functions for the tunnel
func (h *HttpTunnel) RegisterCallback(onBaseDiscovered tunnel.OnNodeDiscovered, onHealthDataArrived tunnel.OnNodeStatusDataArrived, onQueryAllBizDataArrived tunnel.OnQueryAllContainerStatusDataArrived, onOneBizDataArrived tunnel.OnSingleContainerStatusChanged) {
	h.onBaseDiscovered = onBaseDiscovered

	h.onHealthDataArrived = onHealthDataArrived

	h.onQueryAllBizDataArrived = onQueryAllBizDataArrived

	h.onOneBizDataArrived = onOneBizDataArrived
}

// RegisterQuery registers the query function for the tunnel
func (h *HttpTunnel) RegisterQuery(queryBaseline module_tunnels.QueryBaseline) {
	h.queryBaseline = queryBaseline
}

// Start starts the tunnel
func (h *HttpTunnel) Start(ctx context.Context, clientID, env string) (err error) {
	h.onlineNode = make(map[string]bool)
	h.nodeNetworkInfoOfNodeID = make(map[string]model.NetworkInfo)
	h.env = env

	h.arkService = ark_service.NewService()

	h.ready = true

	// add base discovery
	go h.startBaseDiscovery(ctx)

	return
}

// startBaseDiscovery starts the base discovery server
func (h *HttpTunnel) startBaseDiscovery(ctx context.Context) {
	logger := log.G(ctx)
	// start a simple http server to handle base discovery, exit when ctx done
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", h.Port),
		Handler: mux,
	}

	// handle heartbeat post request
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		heartbeatData := model.HeartBeatData{}
		err := json.NewDecoder(r.Body).Decode(&heartbeatData)
		if err != nil {
			logger.WithError(err).Error("failed to unmarshal heartbeat data")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		h.Lock()
		h.nodeNetworkInfoOfNodeID[heartbeatData.BaseID] = heartbeatData.NetworkInfo
		h.Unlock()
		h.onBaseDiscovered(heartbeatData.BaseID, utils.TranslateHeartBeatDataToNodeInfo(heartbeatData), h)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("SUCCESS"))
	})

	mux.HandleFunc("/queryBaseline", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		queryBaselineRequest := model.Metadata{}
		err := json.NewDecoder(r.Body).Decode(&queryBaselineRequest)
		if err != nil {
			logger.WithError(err).Error("failed to unmarshal queryBaselineRequest data")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		baselineBizs := make([]ark.BizModel, 0)
		if h.queryBaseline != nil {
			baseline := h.queryBaseline(utils.TranslateHeartBeatDataToBaselineQuery(queryBaselineRequest))
			for _, container := range baseline {
				baselineBizs = append(baselineBizs, utils.TranslateCoreV1ContainerToBizModel(&container))
			}
		}

		jsonData, _ := json.Marshal(baselineBizs)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonData)
	})

	go func() {
		if err := server.ListenAndServe(); err != nil {
			logger.WithError(err).Error("error starting http base discovery server")
		}
	}()

	logger.Infof("http base discovery server started, listening on port %d", h.Port)

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.WithError(err).Error("error shutting down http server")
		}
	}()
	<-ctx.Done()

}

// healthMsgCallback is the callback function for health messages
func (h *HttpTunnel) healthMsgCallback(nodeID string, data ark_service.HealthResponse) {
	if data.BaseID != nodeID {
		return
	}
	if data.Code != "SUCCESS" {
		return
	}
	if h.onHealthDataArrived != nil {
		h.onHealthDataArrived(nodeID, utils.TranslateHealthDataToNodeStatus(data.Data.HealthData))
	}
}

// allBizMsgCallback is the callback function for all business messages
func (h *HttpTunnel) allBizMsgCallback(nodeID string, data ark_service.QueryAllBizResponse) {
	if data.BaseID != nodeID {
		return
	}
	if data.Code != "SUCCESS" {
		return
	}
	if h.onQueryAllBizDataArrived != nil {
		h.onQueryAllBizDataArrived(nodeID, utils.TranslateBizInfosToContainerStatuses(data.GenericArkResponseBase.Data, time.Now().UnixMilli()))
	}
}

// bizOperationResponseCallback is the callback function for business operation responses
func (h *HttpTunnel) bizOperationResponseCallback(nodeID string, data model.BizOperationResponse) {
	if data.Response.BaseID != nodeID {
		return
	}
	containerState := vkModel.ContainerStateDeactivated
	if data.Response.Code == "SUCCESS" {
		if data.Command == model.CommandInstallBiz {
			// not update here, update in all biz response callback
			return
		}
	} else {
		// operation failed, log
		logrus.Errorf("biz operation failed: %s\n%s\n%s", data.Response.Message, data.Response.ErrorStackTrace, data.Response.Data.Message)
	}

	h.onOneBizDataArrived(nodeID, vkModel.ContainerStatusData{
		Key:        utils.GetBizIdentity(data.BizName, data.BizVersion),
		Name:       data.BizName,
		PodKey:     vkModel.PodKeyAll,
		State:      containerState,
		ChangeTime: time.Now(),
		Reason:     data.Response.Code,
		Message:    data.Response.Message,
	})
}

// FetchHealthData fetches health data from the node
func (h *HttpTunnel) FetchHealthData(ctx context.Context, nodeID string) error {
	h.Lock()
	networkInfo, ok := h.nodeNetworkInfoOfNodeID[nodeID]
	h.Unlock()
	if !ok {
		return errors.New("network info not found")
	}

	healthData, err := h.arkService.Health(ctx, networkInfo.LocalIP, networkInfo.ArkletPort)

	if err != nil {
		return err
	}

	h.healthMsgCallback(nodeID, healthData)

	return nil
}

// QueryAllContainerStatusData queries all container status data from the node
func (h *HttpTunnel) QueryAllContainerStatusData(ctx context.Context, nodeID string) error {
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
			go h.QueryAllContainerStatusData(ctx, nodeID)
		}
	}()

	h.Lock()
	networkInfo, ok := h.nodeNetworkInfoOfNodeID[nodeID]
	h.Unlock()
	if !ok {
		return errors.New("network info not found")
	}

	allBizData, err := h.arkService.QueryAllBiz(ctx, networkInfo.LocalIP, networkInfo.ArkletPort)

	if err != nil {
		return err
	}

	h.allBizMsgCallback(nodeID, allBizData)

	return nil
}

// StartContainer starts a container on the node
func (h *HttpTunnel) StartContainer(ctx context.Context, nodeID, _ string, container *corev1.Container) error {

	h.Lock()
	networkInfo, ok := h.nodeNetworkInfoOfNodeID[nodeID]
	h.Unlock()
	if !ok {
		return errors.New("network info not found")
	}

	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	logger := log.G(ctx).WithField("bizName", bizModel.BizName).WithField("bizVersion", bizModel.BizVersion)
	logger.Info("InstallModule")

	// install current version
	bizOperationResponse := model.BizOperationResponse{
		Command:    model.CommandInstallBiz,
		BizName:    bizModel.BizName,
		BizVersion: bizModel.BizVersion,
	}

	response, err := h.arkService.InstallBiz(ctx, ark_service.InstallBizRequest{
		BizModel: bizModel,
	}, networkInfo.LocalIP, networkInfo.ArkletPort)

	bizOperationResponse.Response = response

	h.bizOperationResponseCallback(nodeID, bizOperationResponse)

	go h.QueryAllContainerStatusData(ctx, nodeID)

	return err
}

// ShutdownContainer shuts down a container on the node
func (h *HttpTunnel) ShutdownContainer(ctx context.Context, nodeID, _ string, container *corev1.Container) error {

	h.Lock()
	networkInfo, ok := h.nodeNetworkInfoOfNodeID[nodeID]
	h.Unlock()
	if !ok {
		return errors.New("network info not found")
	}

	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	logger := log.G(ctx).WithField("bizName", bizModel.BizName).WithField("bizVersion", bizModel.BizVersion)
	logger.Info("UninstallModule")

	bizOperationResponse := model.BizOperationResponse{
		Command:    model.CommandUnInstallBiz,
		BizName:    bizModel.BizName,
		BizVersion: bizModel.BizVersion,
	}

	response, err := h.arkService.UninstallBiz(ctx, ark_service.UninstallBizRequest{
		BizModel: bizModel,
	}, networkInfo.LocalIP, networkInfo.ArkletPort)

	bizOperationResponse.Response = response

	h.bizOperationResponseCallback(nodeID, bizOperationResponse)

	go h.QueryAllContainerStatusData(ctx, nodeID)

	return err
}
