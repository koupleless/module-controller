package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_http_tunnel/ark_service"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_mqtt_tunnel/mqtt"
	"github.com/koupleless/virtual-kubelet/common/utils"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"strings"
	"sync"
	"time"
)

type MockMQTTBase struct {
	sync.Mutex
	Env          string
	CurrState    string
	BaseMetadata model.BaseMetadata
	BizInfos     map[string]ark.ArkBizInfo
	Baseline     []ark.BizModel
	client       *mqtt.Client

	exit      chan struct{}
	reachable bool
}

func NewMockMqttBase(baseName, clusterName, version, env string) *MockMQTTBase {
	return &MockMQTTBase{
		Env:       env,
		CurrState: "ACTIVATED",
		BaseMetadata: model.BaseMetadata{
			Identity:    baseName,
			ClusterName: clusterName,
			Version:     version,
		},
		BizInfos:  make(map[string]ark.ArkBizInfo),
		exit:      make(chan struct{}),
		reachable: true,
	}
}

func (b *MockMQTTBase) Exit() {
	select {
	case <-b.exit:
	default:
		close(b.exit)
	}
}

func (b *MockMQTTBase) Start(ctx context.Context, clientID string) error {
	b.exit = make(chan struct{})
	b.CurrState = "ACTIVATED"
	var err error
	b.client, err = mqtt.NewMqttClient(&mqtt.ClientConfig{
		Broker:   "localhost",
		Port:     1883,
		ClientID: clientID,
		Username: "test",
		Password: "",
		OnConnectHandler: func(client paho.Client) {
			client.Subscribe(fmt.Sprintf("koupleless_%s/%s/+", b.Env, b.BaseMetadata.Identity), 1, b.processCommand)
			client.Subscribe(fmt.Sprintf("koupleless_%s/%s/base/baseline", b.Env, b.BaseMetadata.Identity), 1, b.processBaseline)
		},
	})
	if err != nil {
		return err
	}

	b.client.Connect()

	// Start a new goroutine to upload node heart beat data every 10 seconds
	go utils.TimedTaskWithInterval(ctx, time.Second*10, func(ctx context.Context) {
		if b.reachable {
			log.G(ctx).Info("upload node heart beat data from node ", b.BaseMetadata.Identity)
			b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.BaseMetadata.Identity), 1, b.getHeartBeatMsg())
		}
	})

	// send heart beat message
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.BaseMetadata.Identity), 1, b.getHeartBeatMsg())

	select {
	case <-b.exit:
		b.SetCurrState("DEACTIVATED")
		b.client.Disconnect()
	case <-ctx.Done():
	}
	return nil
}

func (b *MockMQTTBase) SetCurrState(state string) {
	b.CurrState = state
	// send heart beat message
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.BaseMetadata.Identity), 1, b.getHeartBeatMsg())
}

func (b *MockMQTTBase) getHeartBeatMsg() []byte {
	msg := model.ArkMqttMsg[model.BaseStatus]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data: model.BaseStatus{
			BaseMetadata:  b.BaseMetadata,
			LocalIP:       "127.0.0.1",
			LocalHostName: "localhost",
			State:         b.CurrState,
		},
	}
	msgBytes, _ := json.Marshal(msg)
	return msgBytes
}

func (b *MockMQTTBase) getHealthMsg() []byte {
	msg := model.ArkMqttMsg[ark.HealthResponse]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data: ark.HealthResponse{
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
							BizName:    b.BaseMetadata.Identity,
							BizState:   b.CurrState,
							BizVersion: b.BaseMetadata.Version,
						},
						Cpu: ark.CpuInfo{
							Count:      1,
							TotalUsed:  20,
							Type:       "intel",
							UserUsed:   2,
							Free:       80,
							SystemUsed: 13,
						},
					},
				},
				Message: "",
			},
		},
	}
	msgBytes, _ := json.Marshal(msg)
	return msgBytes
}

func (b *MockMQTTBase) getQueryAllBizMsg() []byte {

	arkBizInfos := make([]ark.ArkBizInfo, 0)

	for _, bizInfo := range b.BizInfos {
		arkBizInfos = append(arkBizInfos, bizInfo)
	}

	msg := model.ArkMqttMsg[ark.QueryAllArkBizResponse]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data: ark.QueryAllArkBizResponse{
			GenericArkResponseBase: ark.GenericArkResponseBase[[]ark.ArkBizInfo]{
				Code:    "SUCCESS",
				Data:    arkBizInfos,
				Message: "",
			},
		},
	}
	msgBytes, _ := json.Marshal(msg)
	return msgBytes
}

func (b *MockMQTTBase) processCommand(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	if b.reachable {
		split := strings.Split(msg.Topic(), "/")
		command := split[len(split)-1]
		switch command {
		case model.CommandHealth:
			go b.processHealth()
		case model.CommandInstallBiz:
			go b.processInstallBiz(msg.Payload())
		case model.CommandUnInstallBiz:
			go b.processUnInstallBiz(msg.Payload())
		case model.CommandQueryAllBiz:
			go b.processQueryAllBiz()
		}
	}
}

func (b *MockMQTTBase) processBaseline(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	var data []ark.BizModel
	json.Unmarshal(msg.Payload(), &data)
	for _, bizModel := range data {
		identity := getBizIdentity(bizModel)
		b.BizInfos[identity] = ark.ArkBizInfo{
			BizName:         bizModel.BizName,
			BizState:        "ACTIVATED",
			BizVersion:      bizModel.BizVersion,
			BizStateRecords: []ark.ArkBizStateRecord{},
		}
	}
	b.Baseline = data
}

func (b *MockMQTTBase) processHealth() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/health", b.Env, b.BaseMetadata.Identity), 1, b.getHealthMsg())
}

func (b *MockMQTTBase) processInstallBiz(msg []byte) {
	b.Lock()
	defer b.Unlock()
	request := ark.BizModel{}
	json.Unmarshal(msg, &request)
	identity := getBizIdentity(request)
	_, has := b.BizInfos[identity]
	if !has {
		b.BizInfos[identity] = ark.ArkBizInfo{
			BizName:         request.BizName,
			BizState:        "RESOLVED",
			BizVersion:      request.BizVersion,
			BizStateRecords: []ark.ArkBizStateRecord{},
		}
	}
}

func (b *MockMQTTBase) processUnInstallBiz(msg []byte) {
	b.Lock()
	defer b.Unlock()
	request := ark.BizModel{}
	json.Unmarshal(msg, &request)
	delete(b.BizInfos, getBizIdentity(request))
	// send to response
	resp := model.ArkMqttMsg[model.BizOperationResponse]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data: model.BizOperationResponse{
			Command:    model.CommandUnInstallBiz,
			BizName:    request.BizName,
			BizVersion: request.BizVersion,
			Response: ark_service.ArkResponse[ark.ArkResponseData]{
				Code: "SUCCESS",
				Data: ark.ArkResponseData{
					ArkClientResponse: ark.ArkClientResponse{
						Code:     "SUCCESS",
						Message:  "",
						BizInfos: nil,
					},
					ElapsedSpace: 0,
				},
				Message:         "",
				ErrorStackTrace: "",
			},
		},
	}
	respBytes, _ := json.Marshal(resp)
	b.client.Pub(fmt.Sprintf(model.BaseBizOperationResponseTopic, b.Env, b.BaseMetadata.Identity), 1, respBytes)
}

func (b *MockMQTTBase) SendInvalidMessage() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/health", b.Env, b.BaseMetadata.Identity), 1, []byte(""))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/biz", b.Env, b.BaseMetadata.Identity), 1, []byte(""))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.BaseMetadata.Identity), 1, []byte(""))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/queryBaseline", b.Env, b.BaseMetadata.Identity), 1, []byte(""))
}

func (b *MockMQTTBase) SendTimeoutMessage() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/health", b.Env, b.BaseMetadata.Identity), 1, []byte("{\"publishTimestamp\":0}"))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/biz", b.Env, b.BaseMetadata.Identity), 1, []byte("{\"publishTimestamp\":0}"))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.BaseMetadata.Identity), 1, []byte("{\"publishTimestamp\":0}"))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/queryBaseline", b.Env, b.BaseMetadata.Identity), 1, []byte("{\"publishTimestamp\":0}"))
}

func (b *MockMQTTBase) SendFailedMessage() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/health", b.Env, b.BaseMetadata.Identity), 1, []byte(fmt.Sprintf("{\"publishTimestamp\":%d, \"data\" : {\"code\":\"\"}}", time.Now().UnixMilli())))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/biz", b.Env, b.BaseMetadata.Identity), 1, []byte(fmt.Sprintf("{\"publishTimestamp\":%d, \"data\" : {\"code\":\"\"}}", time.Now().UnixMilli())))
}

func (b *MockMQTTBase) QueryBaseline() {
	queryBaselineBytes, _ := json.Marshal(model.ArkMqttMsg[model.BaseMetadata]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data:             b.BaseMetadata,
	})
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/queryBaseline", b.Env, b.BaseMetadata.Identity), 1, queryBaselineBytes)
}

func (b *MockMQTTBase) SetBizState(bizIdentity, state, reason, message string) {
	b.Lock()
	defer b.Unlock()
	info := b.BizInfos[bizIdentity]
	info.BizState = state
	info.BizStateRecords = append(info.BizStateRecords, ark.ArkBizStateRecord{
		ChangeTime: 1234,
		State:      state,
		Reason:     reason,
		Message:    message,
	})
	b.BizInfos[bizIdentity] = info
	if state == "ACTIVATED" {
		// send to response
		resp := model.ArkMqttMsg[model.BizOperationResponse]{
			PublishTimestamp: time.Now().UnixMilli(),
			Data: model.BizOperationResponse{
				Command:    model.CommandInstallBiz,
				BizName:    info.BizName,
				BizVersion: info.BizVersion,
				Response: ark_service.ArkResponse[ark.ArkResponseData]{
					Code: "SUCCESS",
					Data: ark.ArkResponseData{
						ArkClientResponse: ark.ArkClientResponse{
							Code:     "SUCCESS",
							Message:  "",
							BizInfos: nil,
						},
						ElapsedSpace: 0,
					},
					Message:         "",
					ErrorStackTrace: "",
				},
			},
		}
		respBytes, _ := json.Marshal(resp)
		b.client.Pub(fmt.Sprintf(model.BaseBizOperationResponseTopic, b.Env, b.BaseMetadata.Identity), 1, respBytes)
	}
	// send simple all biz data
	arkBizInfos := make(model.ArkSimpleAllBizInfoData, 0)

	for _, bizInfo := range b.BizInfos {
		stateIndex := "4"
		if state == "ACTIVATED" {
			stateIndex = "3"
		}
		arkBizInfos = append(arkBizInfos, model.ArkSimpleBizInfoData{
			Name:    bizInfo.BizName,
			Version: bizInfo.BizVersion,
			State:   stateIndex,
		})
	}

	msg := model.ArkMqttMsg[model.ArkSimpleAllBizInfoData]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data:             arkBizInfos,
	}
	msgBytes, _ := json.Marshal(msg)
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/simpleBiz", b.Env, b.BaseMetadata.Identity), 1, msgBytes)
}

func (b *MockMQTTBase) processQueryAllBiz() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/biz", b.Env, b.BaseMetadata.Identity), 1, b.getQueryAllBizMsg())
}

func getBizIdentity(bizModel ark.BizModel) string {
	return fmt.Sprintf("%s:%s", bizModel.BizName, bizModel.BizVersion)
}
