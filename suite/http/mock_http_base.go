package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_http_tunnel/ark_service"
	"github.com/koupleless/virtual-kubelet/common/log"
	"github.com/koupleless/virtual-kubelet/common/utils"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"sync"
	"time"
)

type MockHttpBase struct {
	sync.Mutex
	Env       string
	CurrState string
	Metadata  model.BaseMetadata
	port      int
	BizInfos  map[string]ark.ArkBizInfo
	Baseline  []ark.BizModel

	exit      chan struct{}
	reachable bool
}

func NewMockHttpBase(name, clusterName, version, env string, port int) *MockHttpBase {
	return &MockHttpBase{
		Env:       env,
		CurrState: "ACTIVATED",
		Metadata: model.BaseMetadata{
			Identity:    name,
			ClusterName: clusterName,
			Version:     version,
		},
		BizInfos:  make(map[string]ark.ArkBizInfo),
		exit:      make(chan struct{}),
		reachable: true,
		port:      port,
	}
}

func (b *MockHttpBase) Exit() {
	select {
	case <-b.exit:
	default:
		close(b.exit)
	}
}

func (b *MockHttpBase) Start(ctx context.Context, clientID string) error {
	b.exit = make(chan struct{})
	b.CurrState = "ACTIVATED"
	// start a http server to mock base
	mux := http.NewServeMux()

	server := http.Server{
		Addr:    fmt.Sprintf(":%d", b.port),
		Handler: mux,
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if b.reachable {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(b.getHealthMsg())
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
	})

	mux.HandleFunc("/queryAllBiz", func(w http.ResponseWriter, r *http.Request) {
		if b.reachable {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(b.getQueryAllBizMsg())
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
	})

	mux.HandleFunc("/installBiz", func(w http.ResponseWriter, r *http.Request) {
		if b.reachable {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")

			defer r.Body.Close()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				logrus.Errorf("error reading body: %s", err)
				return
			}

			w.Write(b.processInstallBiz(body))
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}

	})

	mux.HandleFunc("/uninstallBiz", func(w http.ResponseWriter, r *http.Request) {
		if b.reachable {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")

			defer r.Body.Close()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				logrus.Errorf("error reading body: %s", err)
				return
			}

			w.Write(b.processUnInstallBiz(body))
		} else {
			w.WriteHeader(http.StatusBadGateway)
		}
	})

	go server.ListenAndServe()

	// Start a new goroutine to upload node heart beat data every 10 seconds
	go utils.TimedTaskWithInterval(ctx, time.Second*10, func(ctx context.Context) {
		if b.reachable {
			log.G(ctx).Info("upload node heart beat data from node ", b.Metadata.Identity)
			_, err := http.Post("http://127.0.0.1:7777/heartbeat", "application/json", bytes.NewBuffer(b.getHeartBeatMsg()))
			if err != nil {
				logrus.Errorf("error calling heartbeat: %s", err)
			}
		}
	})

	_, err := http.Post("http://127.0.0.1:7777/heartbeat", "application/json", bytes.NewBuffer(b.getHeartBeatMsg()))
	if err != nil {
		logrus.Errorf("error calling heartbeat: %s", err)
		return err
	}

	go func() {
		select {
		case <-ctx.Done():
		case <-b.exit:
		}
		b.CurrState = "DEACTIVATED"
		_, err = http.Post("http://127.0.0.1:7777/heartbeat", "application/json", bytes.NewBuffer(b.getHeartBeatMsg()))
		time.Sleep(2 * time.Second)
		server.Shutdown(ctx)
		if err != nil {
			logrus.Errorf("error calling heartbeat: %s", err)
		}
	}()

	return nil
}

func (b *MockHttpBase) getHeartBeatMsg() []byte {
	msg := model.BaseStatus{
		BaseMetadata:  b.Metadata,
		LocalIP:       "127.0.0.1",
		LocalHostName: "localhost",
		Port:          b.port,
		State:         b.CurrState,
	}
	msgBytes, _ := json.Marshal(msg)
	return msgBytes
}

func (b *MockHttpBase) getHealthMsg() []byte {
	msg := ark_service.HealthResponse{
		GenericArkResponseBase: ark.GenericArkResponseBase[ark.HealthInfo]{
			Code: "SUCCESS",
			Data: ark.HealthInfo{
				HealthData: ark.HealthData{
					Jvm: ark.JvmInfo{
						JavaUsedMetaspace:      10240,
						JavaMaxMetaspace:       1024,
						JavaCommittedMetaspace: 1024,
					},
					MasterBizInfo: ark.MasterBizInfo{
						BizName:    b.Metadata.Identity,
						BizState:   b.CurrState,
						BizVersion: b.Metadata.Version,
					},
				},
			},
			Message: "",
		},
	}
	msgBytes, _ := json.Marshal(msg)
	return msgBytes
}

func (b *MockHttpBase) getQueryAllBizMsg() []byte {

	arkBizInfos := make([]ark.ArkBizInfo, 0)

	for _, bizInfo := range b.BizInfos {
		arkBizInfos = append(arkBizInfos, bizInfo)
	}

	msg := ark_service.QueryAllBizResponse{
		GenericArkResponseBase: ark.GenericArkResponseBase[[]ark.ArkBizInfo]{
			Code:    "SUCCESS",
			Data:    arkBizInfos,
			Message: "",
		},
	}
	msgBytes, _ := json.Marshal(msg)
	return msgBytes
}

func (b *MockHttpBase) processInstallBiz(msg []byte) []byte {
	b.Lock()
	defer b.Unlock()
	request := ark_service.InstallBizRequest{}
	json.Unmarshal(msg, &request)
	identity := getBizIdentity(request.BizModel)
	logrus.Infof("install biz %s from http base", identity)
	_, has := b.BizInfos[identity]
	if !has {
		b.BizInfos[identity] = ark.ArkBizInfo{
			BizName:    request.BizName,
			BizState:   "ACTIVATED",
			BizVersion: request.BizVersion,
			BizStateRecords: []ark.ArkBizStateRecord{
				{
					ChangeTime: time.Now().Format("2006-01-02 15:04:05.000"),
					State:      "ACTIVATED",
				},
			},
		}
	}
	response := ark_service.ArkResponse{
		Code: "SUCCESS",
		Data: ark.ArkResponseData{
			Code:         "SUCCESS",
			Message:      "",
			ElapsedSpace: 0,
			BizInfos:     nil,
		},
		Message:         "",
		ErrorStackTrace: "",
		BaseIdentity:    b.Metadata.Identity,
	}
	respBytes, _ := json.Marshal(response)
	return respBytes
}

func (b *MockHttpBase) processUnInstallBiz(msg []byte) []byte {
	b.Lock()
	defer b.Unlock()
	request := ark_service.UninstallBizRequest{}
	json.Unmarshal(msg, &request)
	identity := getBizIdentity(request.BizModel)
	logrus.Infof("uninstall biz %s from http base", identity)
	delete(b.BizInfos, identity)
	// send to response
	response := ark_service.ArkResponse{
		Code: "SUCCESS",
		Data: ark.ArkResponseData{
			Code:         "SUCCESS",
			Message:      "",
			ElapsedSpace: 0,
			BizInfos:     nil,
		},
		Message:         "",
		ErrorStackTrace: "",
		BaseIdentity:    b.Metadata.Identity,
	}
	respBytes, _ := json.Marshal(response)
	return respBytes
}

func getBizIdentity(bizModel ark.BizModel) string {
	return fmt.Sprintf("%s:%s", bizModel.BizName, bizModel.BizVersion)
}
