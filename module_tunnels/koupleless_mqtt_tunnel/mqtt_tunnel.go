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

	onBaseDiscovered         tunnel.OnNodeDiscovered
	onHealthDataArrived      tunnel.OnNodeStatusDataArrived
	onQueryAllBizDataArrived tunnel.OnQueryAllContainerStatusDataArrived
	onOneBizDataArrived      tunnel.OnSingleContainerStatusChanged
	queryBaseline            module_tunnels.QueryBaseline

	onlineNode map[string]bool
}

func (m *MqttTunnel) Ready() bool {
	return m.ready
}

func (m *MqttTunnel) GetContainerUniqueKey(_ string, container *corev1.Container) string {
	return utils.GetBizIdentity(container.Name, utils.GetBizVersionFromContainer(container))
}

func (m *MqttTunnel) OnNodeStart(ctx context.Context, nodeID string, _ vkModel.NodeInfo) {
	m.mqttClient.Sub(fmt.Sprintf(model.BaseHealthTopic, m.env, nodeID), mqtt.Qos1, m.healthMsgCallback)

	m.mqttClient.Sub(fmt.Sprintf(model.BaseSimpleBizTopic, m.env, nodeID), mqtt.Qos1, m.bizMsgCallback)

	m.mqttClient.Sub(fmt.Sprintf(model.BaseAllBizTopic, m.env, nodeID), mqtt.Qos1, m.allBizMsgCallback)

	m.mqttClient.Sub(fmt.Sprintf(model.BaseBizOperationResponseTopic, m.env, nodeID), mqtt.Qos1, m.bizOperationResponseCallback)
	m.Lock()
	defer m.Unlock()
	m.onlineNode[nodeID] = true
}

func (m *MqttTunnel) OnNodeStop(ctx context.Context, nodeID string) {
	m.mqttClient.UnSub(fmt.Sprintf(model.BaseHealthTopic, m.env, nodeID))

	m.mqttClient.UnSub(fmt.Sprintf(model.BaseSimpleBizTopic, m.env, nodeID))

	m.mqttClient.UnSub(fmt.Sprintf(model.BaseAllBizTopic, m.env, nodeID))

	m.mqttClient.UnSub(fmt.Sprintf(model.BaseBizOperationResponseTopic, m.env, nodeID))

	m.Lock()
	defer m.Unlock()
	delete(m.onlineNode, nodeID)
}

func (m *MqttTunnel) OnNodeNotReady(ctx context.Context, info vkModel.UnreachableNodeInfo) {
	utils.OnBaseUnreachable(ctx, info, m.env, m.Client)
}

func (m *MqttTunnel) Key() string {
	return "mqtt_tunnel_provider"
}

func (m *MqttTunnel) RegisterCallback(onBaseDiscovered tunnel.OnNodeDiscovered, onHealthDataArrived tunnel.OnNodeStatusDataArrived, onQueryAllBizDataArrived tunnel.OnQueryAllContainerStatusDataArrived, onOneBizDataArrived tunnel.OnSingleContainerStatusChanged) {
	m.onBaseDiscovered = onBaseDiscovered

	m.onHealthDataArrived = onHealthDataArrived

	m.onQueryAllBizDataArrived = onQueryAllBizDataArrived

	m.onOneBizDataArrived = onOneBizDataArrived
}

func (m *MqttTunnel) RegisterQuery(queryBaseline module_tunnels.QueryBaseline) {
	m.queryBaseline = queryBaseline
}

func (m *MqttTunnel) Start(ctx context.Context, clientID, env string) (err error) {
	c := &MqttConfig{}
	c.init()
	clientID = fmt.Sprintf("%s@@@%s", c.MqttClientPrefix, clientID)
	m.onlineNode = make(map[string]bool)
	m.env = env
	m.mqttClient, err = mqtt.NewMqttClient(&mqtt.ClientConfig{
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
			client.Subscribe(fmt.Sprintf(model.BaseHeartBeatTopic, m.env), mqtt.Qos1, m.heartBeatMsgCallback)
			client.Subscribe(fmt.Sprintf(model.BaseQueryBaselineTopic, m.env), mqtt.Qos1, m.queryBaselineMsgCallback)
			for nodeId, _ := range m.onlineNode {
				m.OnNodeStart(ctx, nodeId, vkModel.NodeInfo{})
			}
		},
	})

	err = m.mqttClient.Connect()
	if err != nil {
		log.G(ctx).WithError(err).Error("mqtt connect error")
		return
	}

	go func() {
		<-ctx.Done()
		m.mqttClient.Disconnect()
	}()
	m.ready = true
	return
}

func (m *MqttTunnel) heartBeatMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
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
	if m.onBaseDiscovered != nil {
		m.onBaseDiscovered(nodeID, utils.TranslateHeartBeatDataToNodeInfo(data.Data), m)
	}
}

func (m *MqttTunnel) queryBaselineMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
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
	if m.queryBaseline != nil {
		baseline := m.queryBaseline(utils.TranslateHeartBeatDataToBaselineQuery(data.Data))
		go func() {
			baselineBizs := make([]ark.BizModel, 0)
			for _, container := range baseline {
				baselineBizs = append(baselineBizs, utils.TranslateCoreV1ContainerToBizModel(&container))
			}
			baselineBytes, _ := json.Marshal(baselineBizs)
			err = m.mqttClient.Pub(utils.FormatBaselineResponseTopic(m.env, nodeID), mqtt.Qos1, baselineBytes)
			if err != nil {
				logrus.WithError(err).Errorf("Error publishing baseline response data")
			}
		}()
	}
}

func (m *MqttTunnel) healthMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
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
	if m.onHealthDataArrived != nil {
		m.onHealthDataArrived(nodeID, utils.TranslateHealthDataToNodeStatus(data.Data.Data.HealthData))
	}
}

func (m *MqttTunnel) bizMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()

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

	if m.onQueryAllBizDataArrived != nil {
		bizInfos := utils.TranslateSimpleBizDataToBizInfos(data.Data)
		m.onQueryAllBizDataArrived(nodeID, utils.TranslateBizInfosToContainerStatuses(bizInfos, data.PublishTimestamp))
	}
}

func (m *MqttTunnel) allBizMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()

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

	if m.onQueryAllBizDataArrived != nil {
		m.onQueryAllBizDataArrived(nodeID, utils.TranslateBizInfosToContainerStatuses(data.Data.GenericArkResponseBase.Data, data.PublishTimestamp))
	}
}

func (m *MqttTunnel) bizOperationResponseCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()

	nodeID := utils.GetBaseIDFromTopic(msg.Topic())
	var data model.ArkMqttMsg[model.BizOperationResponse]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		logrus.Errorf("Error unmarshalling biz response: %v", err)
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}

	containerState := vkModel.ContainerStateDeactivated
	if data.Data.Response.Code == "SUCCESS" {
		if data.Data.Command == model.CommandInstallBiz {
			// not update here, update in all biz response callback
			return
		}
	} else {
		// operation failed, log
		logrus.Errorf("biz operation failed: %s\n%s\n%s", data.Data.Response.Message, data.Data.Response.ErrorStackTrace, data.Data.Response.Data.Message)
	}

	m.onOneBizDataArrived(nodeID, vkModel.ContainerStatusData{
		Key:        utils.GetBizIdentity(data.Data.BizName, data.Data.BizVersion),
		Name:       data.Data.BizName,
		PodKey:     vkModel.PodKeyAll,
		State:      containerState,
		ChangeTime: time.UnixMilli(data.PublishTimestamp),
		Reason:     data.Data.Response.Data.Code,
		Message:    data.Data.Response.Data.Message,
	})
}

func (m *MqttTunnel) FetchHealthData(_ context.Context, nodeID string) error {
	return m.mqttClient.Pub(utils.FormatArkletCommandTopic(m.env, nodeID, model.CommandHealth), mqtt.Qos0, []byte("{}"))
}

func (m *MqttTunnel) QueryAllContainerStatusData(_ context.Context, nodeID string) error {
	return m.mqttClient.Pub(utils.FormatArkletCommandTopic(m.env, nodeID, model.CommandQueryAllBiz), mqtt.Qos0, []byte("{}"))
}

func (m *MqttTunnel) StartContainer(ctx context.Context, nodeID, _ string, container *corev1.Container) error {
	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	logger := log.G(ctx).WithField("bizName", bizModel.BizName).WithField("bizVersion", bizModel.BizVersion)
	logger.Info("InstallModule")
	installBizRequestBytes, _ := json.Marshal(bizModel)
	return m.mqttClient.Pub(utils.FormatArkletCommandTopic(m.env, nodeID, model.CommandInstallBiz), mqtt.Qos0, installBizRequestBytes)
}

func (m *MqttTunnel) ShutdownContainer(ctx context.Context, nodeID, _ string, container *corev1.Container) error {
	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	unInstallBizRequestBytes, _ := json.Marshal(bizModel)
	logger := log.G(ctx).WithField("bizName", bizModel.BizName).WithField("bizVersion", bizModel.BizVersion)
	logger.Info("UninstallModule")
	return m.mqttClient.Pub(utils.FormatArkletCommandTopic(m.env, nodeID, model.CommandUnInstallBiz), mqtt.Qos0, unInstallBizRequestBytes)
}
