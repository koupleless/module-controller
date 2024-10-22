package suite

import (
	"context"
	"encoding/json"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"sync"
	"time"
)

type MockHttpBase struct {
	sync.Mutex
	ID        string
	Env       string
	CurrState string
	Metadata  model.Metadata
	Port      int
	BizInfos  map[string]ark.ArkBizInfo
	Baseline  []ark.BizModel

	exit        chan struct{}
	unreachable chan struct{}
}

func NewMockHttpBase(name, version, id, env string) *MockHttpBase {
	return &MockHttpBase{
		ID:        id,
		Env:       env,
		CurrState: "ACTIVATED",
		Metadata: model.Metadata{
			Name:    name,
			Version: version,
		},
		BizInfos:    make(map[string]ark.ArkBizInfo),
		exit:        make(chan struct{}),
		unreachable: make(chan struct{}),
	}
}

func (b *MockHttpBase) Exit() {
	select {
	case <-b.exit:
	default:
		close(b.exit)
	}
}

func (b *MockHttpBase) Unreachable() {
	select {
	case <-b.unreachable:
	default:
		close(b.unreachable)
	}
}

func (b *MockHttpBase) Start(ctx context.Context) error {
	return nil
}

func (b *MockHttpBase) SetCurrState(state string) {
	b.CurrState = state
}

func (b *MockHttpBase) getHeartBeatMsg() []byte {
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

func (b *MockHttpBase) getHealthMsg() []byte {
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

func (b *MockHttpBase) getQueryAllBizMsg() []byte {

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

func (b *MockHttpBase) processBaseline(_ paho.Client, msg paho.Message) {
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

func (b *MockHttpBase) processInstallBiz(msg []byte) {
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

func (b *MockHttpBase) SetBizState(bizIdentity, state, reason, message string) {
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
}
