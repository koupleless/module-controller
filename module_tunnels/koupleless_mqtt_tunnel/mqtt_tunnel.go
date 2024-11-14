package koupleless_mqtt_tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/common/utils"
	"github.com/koupleless/module_controller/module_tunnels"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_mqtt_tunnel/mqtt"
	"github.com/koupleless/virtual-kubelet/common/log"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

var _ module_tunnels.ModuleTunnel = &MqttTunnel{}

type MqttTunnel struct {
	sync.Mutex

	mqttClient *mqtt.Client
	cache.Cache
	client.Client
	env string

	ready bool

	onBaseDiscovered      tunnel.OnBaseDiscovered
	onHealthDataArrived   tunnel.OnBaseStatusArrived
	onAllBizStatusArrived tunnel.OnAllBizStatusArrived
	onOneBizDataArrived   tunnel.OnSingleBizStatusArrived
	queryBaseline         module_tunnels.QueryBaseline

	onlineNode map[string]bool
}

func (mqttTunnel *MqttTunnel) Ready() bool {
	return mqttTunnel.ready
}

func (mqttTunnel *MqttTunnel) GetBizUniqueKey(container *corev1.Container) string {
	return utils.GetBizIdentity(container.Name, utils.GetBizVersionFromContainer(container))
}

func (mqttTunnel *MqttTunnel) OnNodeStart(ctx context.Context, nodeID string, _ vkModel.NodeInfo) {
	mqttTunnel.mqttClient.Sub(fmt.Sprintf(model.BaseHealthTopic, mqttTunnel.env, nodeID), mqtt.Qos1, mqttTunnel.healthMsgCallback)

	mqttTunnel.mqttClient.Sub(fmt.Sprintf(model.BaseSimpleBizTopic, mqttTunnel.env, nodeID), mqtt.Qos1, mqttTunnel.bizMsgCallback)

	mqttTunnel.mqttClient.Sub(fmt.Sprintf(model.BaseAllBizTopic, mqttTunnel.env, nodeID), mqtt.Qos1, mqttTunnel.allBizMsgCallback)

	mqttTunnel.mqttClient.Sub(fmt.Sprintf(model.BaseBizOperationResponseTopic, mqttTunnel.env, nodeID), mqtt.Qos1, mqttTunnel.bizOperationResponseCallback)
	mqttTunnel.Lock()
	defer mqttTunnel.Unlock()
	mqttTunnel.onlineNode[nodeID] = true
}

func (mqttTunnel *MqttTunnel) OnNodeStop(ctx context.Context, nodeID string) {
	mqttTunnel.mqttClient.UnSub(fmt.Sprintf(model.BaseHealthTopic, mqttTunnel.env, nodeID))

	mqttTunnel.mqttClient.UnSub(fmt.Sprintf(model.BaseSimpleBizTopic, mqttTunnel.env, nodeID))

	mqttTunnel.mqttClient.UnSub(fmt.Sprintf(model.BaseAllBizTopic, mqttTunnel.env, nodeID))

	mqttTunnel.mqttClient.UnSub(fmt.Sprintf(model.BaseBizOperationResponseTopic, mqttTunnel.env, nodeID))

	mqttTunnel.Lock()
	defer mqttTunnel.Unlock()
	delete(mqttTunnel.onlineNode, nodeID)
}

func (mqttTunnel *MqttTunnel) OnNodeNotReady(ctx context.Context, info vkModel.UnreachableNodeInfo) {
	utils.OnBaseUnreachable(ctx, info, mqttTunnel.env, mqttTunnel.Client)
}

func (mqttTunnel *MqttTunnel) Key() string {
	return "mqtt_tunnel_provider"
}

func (mqttTunnel *MqttTunnel) RegisterCallback(onBaseDiscovered tunnel.OnBaseDiscovered, onHealthDataArrived tunnel.OnBaseStatusArrived, onAllBizStatusArrived tunnel.OnAllBizStatusArrived, onOneBizDataArrived tunnel.OnSingleBizStatusArrived) {
	mqttTunnel.onBaseDiscovered = onBaseDiscovered

	mqttTunnel.onHealthDataArrived = onHealthDataArrived

	mqttTunnel.onAllBizStatusArrived = onAllBizStatusArrived

	mqttTunnel.onOneBizDataArrived = onOneBizDataArrived
}

func (mqttTunnel *MqttTunnel) RegisterQuery(queryBaseline module_tunnels.QueryBaseline) {
	mqttTunnel.queryBaseline = queryBaseline
}

func (mqttTunnel *MqttTunnel) Start(ctx context.Context, clientID, env string) (err error) {
	c := &MqttConfig{}
	c.init()
	clientID = fmt.Sprintf("%s@@@%s", c.MqttClientPrefix, clientID)
	mqttTunnel.onlineNode = make(map[string]bool)
	mqttTunnel.env = env
	mqttTunnel.mqttClient, err = mqtt.NewMqttClient(&mqtt.ClientConfig{
		Broker:        c.MqttBroker,
		Port:          c.MqttPort,
		ClientID:      clientID,
		Username:      c.MqttUsername,
		Password:      c.MqttPassword,
		CAPath:        c.MqttCAPath,
		ClientCrtPath: c.MqttClientCrtPath,
		ClientKeyPath: c.MqttClientKeyPath,
		CleanSession:  true,
		OnConnectHandler: func(client paho.Client) {
			log.G(ctx).Info("MQTT client connected :", clientID)
			client.Subscribe(fmt.Sprintf(model.BaseHeartBeatTopic, mqttTunnel.env), mqtt.Qos1, mqttTunnel.heartBeatMsgCallback)
			client.Subscribe(fmt.Sprintf(model.BaseQueryBaselineTopic, mqttTunnel.env), mqtt.Qos1, mqttTunnel.queryBaselineMsgCallback)
			for nodeId, _ := range mqttTunnel.onlineNode {
				mqttTunnel.OnNodeStart(ctx, nodeId, vkModel.NodeInfo{})
			}
		},
	})

	err = mqttTunnel.mqttClient.Connect()
	if err != nil {
		log.G(ctx).WithError(err).Error("mqtt connect error")
		return
	}

	go func() {
		<-ctx.Done()
		mqttTunnel.mqttClient.Disconnect()
	}()
	mqttTunnel.ready = true
	return
}

func (mqttTunnel *MqttTunnel) heartBeatMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()

	logrus.Infof("query health beat callback for %s: %s", msg.Topic(), msg.Payload())

	nodeID := utils.GetBaseIDFromTopic(msg.Topic())
	var data model.ArkMqttMsg[model.HeartBeatData]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		logrus.Errorf("Error unmarshalling heart beat data: %v", err)
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}
	if mqttTunnel.onBaseDiscovered != nil {
		mqttTunnel.onBaseDiscovered(nodeID, utils.TranslateHeartBeatDataToNodeInfo(data.Data), mqttTunnel)
	}
}

func (mqttTunnel *MqttTunnel) queryBaselineMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()

	logrus.Infof("query baseline callback for %s: %s", msg.Topic(), msg.Payload())

	nodeID := utils.GetBaseIDFromTopic(msg.Topic())
	var data model.ArkMqttMsg[model.Metadata]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		logrus.Errorf("Error unmarshalling queryBaseline data: %v", err)
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}
	if mqttTunnel.queryBaseline != nil {
		baseline := mqttTunnel.queryBaseline(utils.TranslateHeartBeatDataToBaselineQuery(data.Data))
		go func() {
			baselineBizs := make([]ark.BizModel, 0)
			for _, container := range baseline {
				baselineBizs = append(baselineBizs, utils.TranslateCoreV1ContainerToBizModel(&container))
			}
			baselineBytes, _ := json.Marshal(baselineBizs)
			err = mqttTunnel.mqttClient.Pub(utils.FormatBaselineResponseTopic(mqttTunnel.env, nodeID), mqtt.Qos1, baselineBytes)
			if err != nil {
				logrus.WithError(err).Errorf("Error publishing baseline response data")
			}
		}()
	}
}

func (mqttTunnel *MqttTunnel) healthMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()

	logrus.Infof("query base health status callback for %s: %s", msg.Topic(), msg.Payload())

	nodeID := utils.GetBaseIDFromTopic(msg.Topic())
	var data model.ArkMqttMsg[ark.HealthResponse]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		logrus.Errorf("Error unmarshalling health response: %v", err)
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}
	if data.Data.Code != "SUCCESS" {
		return
	}
	if mqttTunnel.onHealthDataArrived != nil {
		mqttTunnel.onHealthDataArrived(nodeID, utils.TranslateHealthDataToNodeStatus(data.Data.Data.HealthData))
	}
}

func (mqttTunnel *MqttTunnel) bizMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()

	logrus.Infof("query all simple biz status callback for %s: %s", msg.Topic(), msg.Payload())

	nodeID := utils.GetBaseIDFromTopic(msg.Topic())
	var data model.ArkMqttMsg[model.ArkSimpleAllBizInfoData]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		logrus.Errorf("Error unmarshalling biz response: %v", err)
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}

	if mqttTunnel.onAllBizStatusArrived != nil {
		bizInfos := utils.TranslateSimpleBizDataToBizInfos(data.Data)
		// 更新 vNode 上 stats
		mqttTunnel.onAllBizStatusArrived(nodeID, utils.TranslateBizInfosToContainerStatuses(bizInfos, data.PublishTimestamp))
	}
}

func (mqttTunnel *MqttTunnel) allBizMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	logrus.Infof("query all biz status callback for %s: %s", msg.Topic(), msg.Payload())

	nodeID := utils.GetBaseIDFromTopic(msg.Topic())
	var data model.ArkMqttMsg[ark.QueryAllArkBizResponse]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		logrus.Errorf("Error unmarshalling biz response: %v", err)
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}

	if mqttTunnel.onAllBizStatusArrived != nil {
		mqttTunnel.onAllBizStatusArrived(nodeID, utils.TranslateBizInfosToContainerStatuses(data.Data.GenericArkResponseBase.Data, data.PublishTimestamp))
	}
}

func (mqttTunnel *MqttTunnel) bizOperationResponseCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()

	logrus.Infof("query biz operation status callback for %s: %s", msg.Topic(), msg.Payload())

	nodeID := utils.GetBaseIDFromTopic(msg.Topic())
	var data model.ArkMqttMsg[model.BizOperationResponse]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		logrus.Errorf("Error unmarshalling biz response: %v", err)
		return
	}

	if data.Data.Command == model.CommandInstallBiz && data.Data.Response.Code == "SUCCESS" {
		mqttTunnel.onOneBizDataArrived(nodeID, vkModel.BizStatusData{
			Key:  utils.GetBizIdentity(data.Data.BizName, data.Data.BizVersion),
			Name: data.Data.BizName,
			// fille PodKey when using
			// PodKey:     vkModel.PodKeyAll,
			State:      string(vkModel.BizStateActivated),
			ChangeTime: time.UnixMilli(data.PublishTimestamp),
		})
	} else if data.Data.Command == model.CommandUnInstallBiz && data.Data.Response.Code == "SUCCESS" {
		mqttTunnel.onOneBizDataArrived(nodeID, vkModel.BizStatusData{
			Key:  utils.GetBizIdentity(data.Data.BizName, data.Data.BizVersion),
			Name: data.Data.BizName,
			// fille PodKey when using
			// PodKey:     vkModel.PodKeyAll,
			State:      string(vkModel.BizStateStopped),
			ChangeTime: time.UnixMilli(data.PublishTimestamp),
		})
	}
}

func (mqttTunnel *MqttTunnel) FetchHealthData(_ context.Context, nodeID string) error {
	return mqttTunnel.mqttClient.Pub(utils.FormatArkletCommandTopic(mqttTunnel.env, nodeID, model.CommandHealth), mqtt.Qos0, []byte("{}"))
}

func (mqttTunnel *MqttTunnel) QueryAllBizStatusData(_ context.Context, nodeID string) error {
	return mqttTunnel.mqttClient.Pub(utils.FormatArkletCommandTopic(mqttTunnel.env, nodeID, model.CommandQueryAllBiz), mqtt.Qos0, []byte("{}"))
}

func (mqttTunnel *MqttTunnel) StartBiz(ctx context.Context, nodeID, _ string, container *corev1.Container) error {
	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	logger := log.G(ctx).WithField("bizName", bizModel.BizName).WithField("bizVersion", bizModel.BizVersion)
	logger.Info("InstallModule")
	installBizRequestBytes, _ := json.Marshal(bizModel)
	return mqttTunnel.mqttClient.Pub(utils.FormatArkletCommandTopic(mqttTunnel.env, nodeID, model.CommandInstallBiz), mqtt.Qos0, installBizRequestBytes)
}

func (mqttTunnel *MqttTunnel) StopBiz(ctx context.Context, nodeID, _ string, container *corev1.Container) error {
	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	unInstallBizRequestBytes, _ := json.Marshal(bizModel)
	logger := log.G(ctx).WithField("bizName", bizModel.BizName).WithField("bizVersion", bizModel.BizVersion)
	logger.Info("UninstallModule")
	return mqttTunnel.mqttClient.Pub(utils.FormatArkletCommandTopic(mqttTunnel.env, nodeID, model.CommandUnInstallBiz), mqtt.Qos0, unInstallBizRequestBytes)
}
