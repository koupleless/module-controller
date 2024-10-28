package utils

import (
	"context"
	"fmt"
	"github.com/koupleless/arkctl/common/fileutil"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/virtual-kubelet/common/log"
	"github.com/koupleless/virtual-kubelet/common/utils"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
	"time"
)

func GetBaseIDFromTopic(topic string) string {
	fileds := strings.Split(topic, "/")
	if len(fileds) < 2 {
		return ""
	}
	return fileds[1]
}

func Expired(publishTimestamp int64, maxLiveMilliSec int64) bool {
	return publishTimestamp+maxLiveMilliSec <= time.Now().UnixMilli()
}

func FormatArkletCommandTopic(env, nodeID, command string) string {
	return fmt.Sprintf("koupleless_%s/%s/%s", env, nodeID, command)
}

func FormatBaselineResponseTopic(env, nodeID string) string {
	return fmt.Sprintf(model.BaseBaselineResponseTopic, env, nodeID)
}

func GetBizVersionFromContainer(container *corev1.Container) string {
	bizVersion := ""
	for _, env := range container.Env {
		if env.Name == "BIZ_VERSION" {
			bizVersion = env.Value
			break
		}
	}
	return bizVersion
}

func TranslateCoreV1ContainerToBizModel(container *corev1.Container) ark.BizModel {
	return ark.BizModel{
		BizName:    container.Name,
		BizVersion: GetBizVersionFromContainer(container),
		BizUrl:     fileutil.FileUrl(container.Image),
	}
}

func GetBizIdentity(bizName, bizVersion string) string {
	return bizName + ":" + bizVersion
}

func TranslateHeartBeatDataToNodeInfo(data model.HeartBeatData) vkModel.NodeInfo {
	state := vkModel.NodeStatusDeactivated
	if data.State == "ACTIVATED" {
		state = vkModel.NodeStatusActivated
	}
	labels := map[string]string{}
	if data.NetworkInfo.ArkletPort != 0 {
		labels[model.LabelKeyOfArkletPort] = strconv.Itoa(data.NetworkInfo.ArkletPort)
	}
	return vkModel.NodeInfo{
		Metadata: vkModel.NodeMetadata{
			Name:    data.MasterBizInfo.Name,
			Version: data.MasterBizInfo.Version,
			Status:  state,
		},
		NetworkInfo: vkModel.NetworkInfo{
			NodeIP:   data.NetworkInfo.LocalIP,
			HostName: data.NetworkInfo.LocalHostName,
		},
		CustomLabels: labels,
	}
}

func TranslateHealthDataToNodeStatus(data ark.HealthData) vkModel.NodeStatusData {
	resourceMap := make(map[corev1.ResourceName]vkModel.NodeResource)
	memory := vkModel.NodeResource{}
	if data.Jvm.JavaMaxMetaspace != -1 {
		memory.Capacity = utils.ConvertByteNumToResourceQuantity(data.Jvm.JavaMaxMetaspace)
	}
	if data.Jvm.JavaMaxMetaspace != -1 && data.Jvm.JavaCommittedMetaspace != -1 {
		memory.Allocatable = utils.ConvertByteNumToResourceQuantity(data.Jvm.JavaMaxMetaspace - data.Jvm.JavaCommittedMetaspace)
	}
	resourceMap[corev1.ResourceMemory] = memory
	return vkModel.NodeStatusData{
		Resources: resourceMap,
		CustomLabels: map[string]string{
			model.LabelKeyOfTechStack: "java",
		},
	}
}

func TranslateHeartBeatDataToBaselineQuery(data model.Metadata) model.QueryBaselineRequest {
	return model.QueryBaselineRequest{
		Name:    data.Name,
		Version: data.Version,
		CustomLabels: map[string]string{
			model.LabelKeyOfTechStack: "java",
		},
	}
}

func TranslateBizInfosToContainerStatuses(data []ark.ArkBizInfo, changeTimestamp int64) []vkModel.ContainerStatusData {
	ret := make([]vkModel.ContainerStatusData, 0)
	for _, bizInfo := range data {
		updatedTime, reason, message := GetLatestState(bizInfo.BizState, bizInfo.BizStateRecords)
		statusData := vkModel.ContainerStatusData{
			Key:        GetBizIdentity(bizInfo.BizName, bizInfo.BizVersion),
			Name:       bizInfo.BizName,
			PodKey:     vkModel.PodKeyAll,
			State:      GetContainerStateFromBizState(bizInfo.BizState),
			ChangeTime: time.UnixMilli(changeTimestamp),
		}
		if updatedTime.UnixMilli() != 0 {
			statusData.ChangeTime = updatedTime
			statusData.Reason = reason
			statusData.Message = message
		}
		ret = append(ret, statusData)
	}
	return ret
}

func TranslateSimpleBizDataToBizInfos(data model.ArkSimpleAllBizInfoData) []ark.ArkBizInfo {
	ret := make([]ark.ArkBizInfo, 0)
	for _, simpleBizInfo := range data {
		bizInfo := TranslateSimpleBizDataToArkBizInfo(simpleBizInfo)
		if bizInfo == nil {
			continue
		}
		ret = append(ret, *bizInfo)
	}
	return ret
}

func TranslateSimpleBizDataToArkBizInfo(data model.ArkSimpleBizInfoData) *ark.ArkBizInfo {
	if len(data) < 3 {
		return nil
	}
	bizName := data[0]
	bizVersion := data[1]
	bizStateIndex := data[2]
	return &ark.ArkBizInfo{
		BizName:    bizName,
		BizVersion: bizVersion,
		BizState:   GetArkBizStateFromSimpleBizState(bizStateIndex),
	}
}

func GetContainerStateFromBizState(bizStateIndex string) vkModel.ContainerState {
	switch strings.ToLower(bizStateIndex) {
	case "resolved":
		return vkModel.ContainerStateResolved
	case "activated":
		return vkModel.ContainerStateActivated
	case "deactivated":
		return vkModel.ContainerStateDeactivated
	case "broken":
		return vkModel.ContainerStateDeactivated
	}
	return vkModel.ContainerStateWaiting
}

func GetArkBizStateFromSimpleBizState(bizStateIndex string) string {
	switch bizStateIndex {
	case "2":
		return "resolved"
	case "3":
		return "activated"
	case "4":
		return "deactivated"
	case "5":
		return "broken"
	}
	return ""
}

func GetLatestState(state string, records []ark.ArkBizStateRecord) (time.Time, string, string) {
	latestStateTime := time.UnixMilli(0)
	reason := ""
	message := ""
	for _, record := range records {
		if record.State != state {
			continue
		}
		if len(record.ChangeTime) < 3 {
			continue
		}
		changeTime, err := time.Parse("2006-01-02 15:04:05", record.ChangeTime[:len(record.ChangeTime)-4])
		if err != nil {
			logrus.Errorf("failed to parse change time %s", record.ChangeTime)
			continue
		}
		if changeTime.UnixMilli() > latestStateTime.UnixMilli() {
			latestStateTime = changeTime
			reason = record.Reason
			message = record.Message
		}
	}
	return latestStateTime, reason, message
}

func OnBaseUnreachable(ctx context.Context, info vkModel.UnreachableNodeInfo, env string, k8sClient client.Client) {
	// base not ready, delete from api server
	node := corev1.Node{}
	nodeName := utils.FormatNodeName(info.NodeID, env)
	err := k8sClient.Get(ctx, client.ObjectKey{Name: nodeName}, &node)
	logger := log.G(ctx).WithField("nodeID", info.NodeID).WithField("func", "OnNodeNotReady")
	if err == nil {
		// delete node from api server
		logger.Info("DeleteBaseNode")
		k8sClient.Delete(ctx, &node)
	} else if apiErrors.IsNotFound(err) {
		logger.Info("Node not found, skipping delete operation")
	} else {
		logger.WithError(err).Error("Failed to get node, cannot delete")
	}
}

func ExtractNetworkInfoFromNodeInfoData(initData vkModel.NodeInfo) model.NetworkInfo {
	portStr := initData.CustomLabels[model.LabelKeyOfArkletPort]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		logrus.Errorf("failed to parse port %s from node info", portStr)
		port = 1238
	}

	return model.NetworkInfo{
		LocalIP:       initData.NetworkInfo.NodeIP,
		LocalHostName: initData.NetworkInfo.HostName,
		ArkletPort:    port,
	}
}
