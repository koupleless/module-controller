package koupleless_mqtt_tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/module_controller/common/utils"
	"github.com/koupleless/module_controller/common/zaplogger"
	"github.com/koupleless/module_controller/controller/module_deployment_controller"
	"github.com/koupleless/module_controller/module_tunnels/koupleless_mqtt_tunnel/mqtt"
	utils2 "github.com/koupleless/virtual-kubelet/common/utils"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/koupleless/virtual-kubelet/tunnel"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

var _ tunnel.Tunnel = &MqttTunnel{}

type MqttTunnel struct {
	ctx context.Context
	sync.Mutex

	mqttClient *mqtt.Client
	kubeClient client.Client
	env        string

	ready bool

	onBaseDiscovered      tunnel.OnBaseDiscovered
	onHealthDataArrived   tunnel.OnBaseStatusArrived
	onAllBizStatusArrived tunnel.OnAllBizStatusArrived
	onOneBizDataArrived   tunnel.OnSingleBizStatusArrived

	moduleDeploymentController *module_deployment_controller.ModuleDeploymentController
}

func NewMqttTunnel(ctx context.Context, env string, kubeClient client.Client, moduleDeploymentController *module_deployment_controller.ModuleDeploymentController) MqttTunnel {
	return MqttTunnel{
		ctx:                        ctx,
		env:                        env,
		kubeClient:                 kubeClient,
		moduleDeploymentController: moduleDeploymentController,
	}
}

func (mqttTunnel *MqttTunnel) Ready() bool {
	return mqttTunnel.ready
}

func (mqttTunnel *MqttTunnel) GetBizUniqueKey(container *corev1.Container) string {
	return utils.GetBizIdentity(container.Name, utils.GetBizVersionFromContainer(container))
}

func (mqttTunnel *MqttTunnel) RegisterNode(nodeInfo vkModel.NodeInfo) {
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeInfo.Metadata.Name)
	mqttTunnel.mqttClient.Sub(fmt.Sprintf(model.BaseHealthTopic, mqttTunnel.env, nodeID), mqtt.Qos1, mqttTunnel.healthMsgCallback)

	mqttTunnel.mqttClient.Sub(fmt.Sprintf(model.BaseSimpleBizTopic, mqttTunnel.env, nodeID), mqtt.Qos1, mqttTunnel.bizMsgCallback)

	mqttTunnel.mqttClient.Sub(fmt.Sprintf(model.BaseAllBizTopic, mqttTunnel.env, nodeID), mqtt.Qos1, mqttTunnel.allBizMsgCallback)

	mqttTunnel.mqttClient.Sub(fmt.Sprintf(model.BaseBizOperationResponseTopic, mqttTunnel.env, nodeID), mqtt.Qos1, mqttTunnel.bizOperationResponseCallback)
	mqttTunnel.Lock()
	defer mqttTunnel.Unlock()
}

func (mqttTunnel *MqttTunnel) UnRegisterNode(nodeName string) {
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	mqttTunnel.mqttClient.UnSub(fmt.Sprintf(model.BaseHealthTopic, mqttTunnel.env, nodeID))

	mqttTunnel.mqttClient.UnSub(fmt.Sprintf(model.BaseSimpleBizTopic, mqttTunnel.env, nodeID))

	mqttTunnel.mqttClient.UnSub(fmt.Sprintf(model.BaseAllBizTopic, mqttTunnel.env, nodeID))

	mqttTunnel.mqttClient.UnSub(fmt.Sprintf(model.BaseBizOperationResponseTopic, mqttTunnel.env, nodeID))

	mqttTunnel.Lock()
	defer mqttTunnel.Unlock()
}

func (mqttTunnel *MqttTunnel) OnNodeNotReady(nodeName string) {
	utils.OnBaseUnreachable(mqttTunnel.ctx, nodeName, mqttTunnel.kubeClient)
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

func (mqttTunnel *MqttTunnel) Start(clientID, env string) (err error) {
	zlogger := zaplogger.FromContext(mqttTunnel.ctx)
	c := &MqttConfig{}
	c.init()
	clientID = fmt.Sprintf("%s@@@%s", c.MqttClientPrefix, clientID)
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
			zlogger.Info(fmt.Sprintf("MQTT client connected: %s", clientID))
			client.Subscribe(fmt.Sprintf(model.BaseHeartBeatTopic, mqttTunnel.env), mqtt.Qos1, mqttTunnel.heartBeatMsgCallback)
			client.Subscribe(fmt.Sprintf(model.BaseQueryBaselineTopic, mqttTunnel.env), mqtt.Qos1, mqttTunnel.queryBaselineMsgCallback)
		},
	})

	err = mqttTunnel.mqttClient.Connect()
	if err != nil {
		zlogger.Error(err, "mqtt connect error")
		return err
	}

	go func() {
		<-mqttTunnel.ctx.Done()
		mqttTunnel.mqttClient.Disconnect()
	}()
	mqttTunnel.ready = true
	return
}

func (mqttTunnel *MqttTunnel) heartBeatMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	zlogger := zaplogger.FromContext(mqttTunnel.ctx)

	zlogger.Info(fmt.Sprintf("query health beat callback for %s: %s", msg.Topic(), msg.Payload()))

	var data model.ArkMqttMsg[model.BaseStatus]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		zlogger.Error(err, "Error unmarshalling heart beat data")
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}
	if mqttTunnel.onBaseDiscovered != nil {
		mqttTunnel.onBaseDiscovered(utils.ConvertBaseStatusToNodeInfo(data.Data, mqttTunnel.env))
	}
}

func (mqttTunnel *MqttTunnel) queryBaselineMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	zlogger := zaplogger.FromContext(mqttTunnel.ctx)

	zlogger.Info(fmt.Sprintf("query baseline callback for %s: %s", msg.Topic(), msg.Payload()))

	nodeID := utils.GetBaseIdentityFromTopic(msg.Topic())
	var data model.ArkMqttMsg[model.BaseMetadata]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		zlogger.Error(err, "Error unmarshalling queryBaseline data")
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}

	baselineContainers := mqttTunnel.moduleDeploymentController.QueryContainerBaseline(mqttTunnel.ctx, utils.ConvertBaseMetadataToBaselineQuery(data.Data))
	go func() {
		baselineBizs := make([]ark.BizModel, 0)
		for _, container := range baselineContainers {
			baselineBizs = append(baselineBizs, utils.TranslateCoreV1ContainerToBizModel(&container))
		}
		baselineBytes, _ := json.Marshal(baselineBizs)
		err = mqttTunnel.mqttClient.Pub(utils.FormatBaselineResponseTopic(mqttTunnel.env, nodeID), mqtt.Qos1, baselineBytes)
		if err != nil {
			zlogger.Error(err, "Error publishing baselineContainers response data")
		}
	}()
}

func (mqttTunnel *MqttTunnel) healthMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	zlogger := zaplogger.FromContext(mqttTunnel.ctx)

	zlogger.Info(fmt.Sprintf("query base health status callback for %s: %s", msg.Topic(), msg.Payload()))

	nodeID := utils.GetBaseIdentityFromTopic(msg.Topic())
	var data model.ArkMqttMsg[ark.HealthResponse]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		zlogger.Error(err, "Error unmarshalling health response")
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}
	if data.Data.Code != "SUCCESS" {
		return
	}
	if mqttTunnel.onHealthDataArrived != nil {
		nodeName := utils2.FormatNodeName(nodeID, mqttTunnel.env)
		mqttTunnel.onHealthDataArrived(nodeName, utils.ConvertHealthDataToNodeStatus(data.Data.Data.HealthData))
	}
}

func (mqttTunnel *MqttTunnel) bizMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	zlogger := zaplogger.FromContext(mqttTunnel.ctx)

	zlogger.Info(fmt.Sprintf("query all simple biz status callback for %s: %s", msg.Topic(), msg.Payload()))

	nodeID := utils.GetBaseIdentityFromTopic(msg.Topic())
	var data model.ArkMqttMsg[model.ArkSimpleAllBizInfoData]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		zlogger.Error(err, "Error unmarshalling biz response")
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}

	if mqttTunnel.onAllBizStatusArrived != nil {
		bizInfos := utils.TranslateSimpleBizDataToBizInfos(data.Data)
		// 更新 vNode 上 stats
		nodeName := utils2.FormatNodeName(nodeID, mqttTunnel.env)
		mqttTunnel.onAllBizStatusArrived(nodeName, utils.TranslateBizInfosToContainerStatuses(bizInfos, data.PublishTimestamp))
	}
}

func (mqttTunnel *MqttTunnel) allBizMsgCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	zlogger := zaplogger.FromContext(mqttTunnel.ctx)

	zlogger.Info(fmt.Sprintf("query all biz status callback for %s: %s", msg.Topic(), msg.Payload()))

	nodeID := utils.GetBaseIdentityFromTopic(msg.Topic())
	var data model.ArkMqttMsg[ark.QueryAllArkBizResponse]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		zlogger.Error(err, "Error unmarshalling biz response")
		return
	}
	if utils.Expired(data.PublishTimestamp, 1000*10) {
		return
	}

	if mqttTunnel.onAllBizStatusArrived != nil {
		nodeName := utils2.FormatNodeName(nodeID, mqttTunnel.env)
		mqttTunnel.onAllBizStatusArrived(nodeName, utils.TranslateBizInfosToContainerStatuses(data.Data.GenericArkResponseBase.Data, data.PublishTimestamp))
	}
}

func (mqttTunnel *MqttTunnel) bizOperationResponseCallback(_ paho.Client, msg paho.Message) {
	defer msg.Ack()
	zlogger := zaplogger.FromContext(mqttTunnel.ctx)

	zlogger.Info(fmt.Sprintf("query biz operation status callback for %s: %s", msg.Topic(), msg.Payload()))

	nodeID := utils.GetBaseIdentityFromTopic(msg.Topic())
	var data model.ArkMqttMsg[model.BizOperationResponse]
	err := json.Unmarshal(msg.Payload(), &data)
	if err != nil {
		zlogger.Error(err, "Error unmarshalling biz response")
		return
	}

	nodeName := utils2.FormatNodeName(nodeID, mqttTunnel.env)
	if data.Data.Command == model.CommandInstallBiz {
		if data.Data.Response.Code == "SUCCESS" {
			mqttTunnel.onOneBizDataArrived(nodeName, vkModel.BizStatusData{
				Key:        utils.GetBizIdentity(data.Data.BizName, data.Data.BizVersion),
				Name:       data.Data.BizName,
				State:      string(vkModel.BizStateActivated),
				ChangeTime: time.UnixMilli(data.PublishTimestamp),
				Reason:     fmt.Sprintf("%s:%s %s succeed", data.Data.BizName, data.Data.BizVersion, data.Data.Command),
				Message:    data.Data.Response.Data.Message,
			})
		} else {
			mqttTunnel.onOneBizDataArrived(nodeName, vkModel.BizStatusData{
				Key:        utils.GetBizIdentity(data.Data.BizName, data.Data.BizVersion),
				Name:       data.Data.BizName,
				State:      string(vkModel.BizStateBroken),
				ChangeTime: time.UnixMilli(data.PublishTimestamp),
				Reason:     fmt.Sprintf("%s:%s %s failed", data.Data.BizName, data.Data.BizVersion, data.Data.Command),
				Message:    data.Data.Response.Data.Message,
			})
		}
	} else if data.Data.Command == model.CommandUnInstallBiz {
		if data.Data.Response.Code == "SUCCESS" {
			mqttTunnel.onOneBizDataArrived(nodeName, vkModel.BizStatusData{
				Key:        utils.GetBizIdentity(data.Data.BizName, data.Data.BizVersion),
				Name:       data.Data.BizName,
				State:      string(vkModel.BizStateStopped),
				ChangeTime: time.UnixMilli(data.PublishTimestamp),
				Reason:     fmt.Sprintf("%s:%s %s succeed", data.Data.BizName, data.Data.BizVersion, data.Data.Command),
				Message:    data.Data.Response.Data.Message,
			})
		} else {
			mqttTunnel.onOneBizDataArrived(nodeName, vkModel.BizStatusData{
				Key:        utils.GetBizIdentity(data.Data.BizName, data.Data.BizVersion),
				Name:       data.Data.BizName,
				State:      string(vkModel.BizStateBroken),
				ChangeTime: time.UnixMilli(data.PublishTimestamp),
				Reason:     fmt.Sprintf("%s:%s %s failed", data.Data.BizName, data.Data.BizVersion, data.Data.Command),
				Message:    data.Data.Response.Data.Message,
			})
		}
	} else {
		unsupported := fmt.Sprintf("unsupport command: %s", data.Data.Command)
		zlogger.Error(errors.New(unsupported), unsupported)
	}
}

func (mqttTunnel *MqttTunnel) FetchHealthData(nodeName string) error {
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	return mqttTunnel.mqttClient.Pub(utils.FormatArkletCommandTopic(mqttTunnel.env, nodeID, model.CommandHealth), mqtt.Qos0, []byte("{}"))
}

func (mqttTunnel *MqttTunnel) QueryAllBizStatusData(nodeName string) error {
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	return mqttTunnel.mqttClient.Pub(utils.FormatArkletCommandTopic(mqttTunnel.env, nodeID, model.CommandQueryAllBiz), mqtt.Qos0, []byte("{}"))
}

func (mqttTunnel *MqttTunnel) StartBiz(nodeName, _ string, container *corev1.Container) error {
	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	zlogger := zaplogger.FromContext(mqttTunnel.ctx).WithValues("bizName", bizModel.BizName, "bizVersion", bizModel.BizVersion)
	zlogger.Info("InstallModule")
	installBizRequestBytes, _ := json.Marshal(bizModel)
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	return mqttTunnel.mqttClient.Pub(utils.FormatArkletCommandTopic(mqttTunnel.env, nodeID, model.CommandInstallBiz), mqtt.Qos0, installBizRequestBytes)
}

func (mqttTunnel *MqttTunnel) StopBiz(nodeName, _ string, container *corev1.Container) error {
	bizModel := utils.TranslateCoreV1ContainerToBizModel(container)
	unInstallBizRequestBytes, _ := json.Marshal(bizModel)
	zlogger := zaplogger.FromContext(mqttTunnel.ctx).WithValues("bizName", bizModel.BizName, "bizVersion", bizModel.BizVersion)
	zlogger.Info("UninstallModule")
	nodeID := utils2.ExtractNodeIDFromNodeName(nodeName)
	return mqttTunnel.mqttClient.Pub(utils.FormatArkletCommandTopic(mqttTunnel.env, nodeID, model.CommandUnInstallBiz), mqtt.Qos0, unInstallBizRequestBytes)
}
