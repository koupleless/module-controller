package suite

import (
	"context"
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_mqtt_tunnel/mqtt"
	"strings"
	"sync"
	"time"
)

type MockBase struct {
	sync.Mutex
	ID        string
	Env       string
	CurrState string
	Metadata  model.Metadata
	BizInfos  map[string]ark.ArkBizInfo
	Baseline  []ark.BizModel
	client    *mqtt.Client

	exit chan struct{}
}

func NewMockBase(name, version, id, env string) *MockBase {
	return &MockBase{
		ID:        id,
		Env:       env,
		CurrState: "ACTIVATED",
		Metadata: model.Metadata{
			Name:    name,
			Version: version,
		},
		BizInfos: make(map[string]ark.ArkBizInfo),
		exit:     make(chan struct{}),
	}
}

func (b *MockBase) Exit() {
	select {
	case <-b.exit:
	default:
		close(b.exit)
	}
}

func (b *MockBase) Start(ctx context.Context) error {
	var err error
	b.client, err = mqtt.NewMqttClient(&mqtt.ClientConfig{
		Broker:   "localhost",
		Port:     1883,
		ClientID: b.ID,
		Username: "test",
		Password: "",
		OnConnectHandler: func(client paho.Client) {
			client.Subscribe(fmt.Sprintf("koupleless_%s/%s/+", b.Env, b.ID), 1, b.processCommand)
			client.Subscribe(fmt.Sprintf("koupleless_%s/%s/base/baseline", b.Env, b.ID), 1, b.processBaseline)
		},
	})
	if err != nil {
		return err
	}

	b.client.Connect()

	go func() {
		// send heart beat message
		b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.ID), 1, b.getHeartBeatMsg())
	}()

	select {
	case <-b.exit:
		b.SetCurrState("DEACTIVATED")
		b.client.Disconnect()
	case <-ctx.Done():
	}
	return nil
}

func (b *MockBase) SetCurrState(state string) {
	b.CurrState = state
	// send heart beat message
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.ID), 1, b.getHeartBeatMsg())
}

func (b *MockBase) getHeartBeatMsg() []byte {
	msg := model.ArkMqttMsg[model.HeartBeatData]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data: model.HeartBeatData{
			State:         b.CurrState,
			MasterBizInfo: b.Metadata,
			NetworkInfo: model.NetworkInfo{
				LocalIP:       "127.0.0.1",
				LocalHostName: "localhost",
			},
		},
	}
	msgBytes, _ := json.Marshal(msg)
	return msgBytes
}

func (b *MockBase) getHealthMsg() []byte {
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
							BizName:    b.Metadata.Name,
							BizState:   b.CurrState,
							BizVersion: b.Metadata.Version,
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

func (b *MockBase) getQueryAllBizMsg() []byte {

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

func (b *MockBase) processCommand(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
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

func (b *MockBase) processBaseline(_ paho.Client, msg paho.Message) {
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

func (b *MockBase) processHealth() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/health", b.Env, b.ID), 1, b.getHealthMsg())
}

func (b *MockBase) processInstallBiz(msg []byte) {
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

func (b *MockBase) processUnInstallBiz(msg []byte) {
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
			Response: ark.ArkResponseBase{
				Code: "SUCCESS",
				Data: ark.ArkResponseData{
					Code:    "SUCCESS",
					Message: "",
				},
				Message:         "",
				ErrorStackTrace: "",
			},
		},
	}
	respBytes, _ := json.Marshal(resp)
	b.client.Pub(fmt.Sprintf(model.BaseBizOperationResponseTopic, b.Env, b.ID), 1, respBytes)
}

func (b *MockBase) SendInvalidMessage() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/health", b.Env, b.ID), 1, []byte(""))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/biz", b.Env, b.ID), 1, []byte(""))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.ID), 1, []byte(""))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/queryBaseline", b.Env, b.ID), 1, []byte(""))
}

func (b *MockBase) SendTimeoutMessage() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/health", b.Env, b.ID), 1, []byte("{\"publishTimestamp\":0}"))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/biz", b.Env, b.ID), 1, []byte("{\"publishTimestamp\":0}"))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/heart", b.Env, b.ID), 1, []byte("{\"publishTimestamp\":0}"))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/queryBaseline", b.Env, b.ID), 1, []byte("{\"publishTimestamp\":0}"))
}

func (b *MockBase) SendFailedMessage() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/health", b.Env, b.ID), 1, []byte(fmt.Sprintf("{\"publishTimestamp\":%d, \"data\" : {\"code\":\"\"}}", time.Now().UnixMilli())))
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/biz", b.Env, b.ID), 1, []byte(fmt.Sprintf("{\"publishTimestamp\":%d, \"data\" : {\"code\":\"\"}}", time.Now().UnixMilli())))
}

func (b *MockBase) QueryBaseline() {
	queryBaselineBytes, _ := json.Marshal(model.ArkMqttMsg[model.Metadata]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data:             b.Metadata,
	})
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/queryBaseline", b.Env, b.ID), 1, queryBaselineBytes)
}

func (b *MockBase) SetBizState(bizIdentity, state, reason, message string) {
	b.Lock()
	defer b.Unlock()
	info := b.BizInfos[bizIdentity]
	info.BizState = state
	info.BizStateRecords = append(info.BizStateRecords, ark.ArkBizStateRecord{
		ChangeTime: time.Now().Format("2006-01-02 15:04:05.000"),
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
				Response: ark.ArkResponseBase{
					Code: "SUCCESS",
					Data: ark.ArkResponseData{
						Code:    "SUCCESS",
						Message: "",
					},
					Message:         "",
					ErrorStackTrace: "",
				},
			},
		}
		respBytes, _ := json.Marshal(resp)
		b.client.Pub(fmt.Sprintf(model.BaseBizOperationResponseTopic, b.Env, b.ID), 1, respBytes)
	}
	// send simple all biz data
	arkBizInfos := make(model.ArkSimpleAllBizInfoData, 0)

	for _, bizInfo := range b.BizInfos {
		stateIndex := "4"
		if state == "ACTIVATED" {
			stateIndex = "3"
		}
		arkBizInfos = append(arkBizInfos, []string{
			bizInfo.BizName,
			bizInfo.BizVersion,
			stateIndex,
		})
	}

	msg := model.ArkMqttMsg[model.ArkSimpleAllBizInfoData]{
		PublishTimestamp: time.Now().UnixMilli(),
		Data:             arkBizInfos,
	}
	msgBytes, _ := json.Marshal(msg)
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/simpleBiz", b.Env, b.ID), 1, msgBytes)
}

func (b *MockBase) processQueryAllBiz() {
	b.client.Pub(fmt.Sprintf("koupleless_%s/%s/base/biz", b.Env, b.ID), 1, b.getQueryAllBizMsg())
}

func getBizIdentity(bizModel ark.BizModel) string {
	return fmt.Sprintf("%s:%s", bizModel.BizName, bizModel.BizVersion)
}
