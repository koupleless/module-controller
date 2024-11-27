package utils

import (
	"context"
	"fmt"
	"github.com/koupleless/module_controller/common/zaplogger"
	"strconv"
	"strings"
	"time"

	"github.com/koupleless/arkctl/common/fileutil"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/virtual-kubelet/common/utils"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetBaseIdentityFromTopic extracts the base ID from a topic string by splitting on '/'
func GetBaseIdentityFromTopic(topic string) string {
	fileds := strings.Split(topic, "/")
	if len(fileds) < 2 {
		return ""
	}
	return fileds[1]
}

// Expired checks if a timestamp has expired based on a max lifetime
func Expired(publishTimestamp int64, maxLiveMilliSec int64) bool {
	return publishTimestamp+maxLiveMilliSec <= time.Now().UnixMilli()
}

// FormatArkletCommandTopic formats a topic string for arklet commands
func FormatArkletCommandTopic(env, nodeID, command string) string {
	return fmt.Sprintf("koupleless_%s/%s/%s", env, nodeID, command)
}

// FormatBaselineResponseTopic formats a topic string for baseline responses
func FormatBaselineResponseTopic(env, nodeID string) string {
	return fmt.Sprintf(model.BaseBaselineResponseTopic, env, nodeID)
}

// GetBizVersionFromContainer extracts the biz version from a container's env vars
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

// TranslateCoreV1ContainerToBizModel converts a k8s container to an ark biz model
func TranslateCoreV1ContainerToBizModel(container *corev1.Container) ark.BizModel {
	return ark.BizModel{
		BizName:    container.Name,
		BizVersion: GetBizVersionFromContainer(container),
		BizUrl:     fileutil.FileUrl(container.Image),
	}
}

// GetBizIdentity creates a unique identifier from biz name and version
func GetBizIdentity(bizName, bizVersion string) string {
	return bizName + ":" + bizVersion
}

// ConvertBaseStatusToNodeInfo converts heartbeat data to node info
func ConvertBaseStatusToNodeInfo(data model.BaseStatus, env string) vkModel.NodeInfo {
	state := vkModel.NodeStateDeactivated
	if strings.EqualFold(data.State, "ACTIVATED") {
		state = vkModel.NodeStateActivated
	}
	labels := map[string]string{}
	if data.Port != 0 {
		labels[model.LabelKeyOfTunnelPort] = strconv.Itoa(data.Port)
	}

	return vkModel.NodeInfo{
		Metadata: vkModel.NodeMetadata{
			Name:        utils.FormatNodeName(data.BaseMetadata.Identity, env),
			ClusterName: data.BaseMetadata.ClusterName,
			Version:     data.BaseMetadata.Version,
		},
		NetworkInfo: vkModel.NetworkInfo{
			NodeIP:   data.LocalIP,
			HostName: data.LocalHostName,
		},
		CustomLabels: labels,
		State:        state,
	}
}

// ConvertHealthDataToNodeStatus converts health data to node status
func ConvertHealthDataToNodeStatus(data ark.HealthData) vkModel.NodeStatusData {
	resourceMap := make(map[corev1.ResourceName]vkModel.NodeResource)
	memory := vkModel.NodeResource{}
	// Set memory capacity if JavaMaxMetaspace is valid (not -1)
	if data.Jvm.JavaMaxMetaspace != -1 {
		memory.Capacity = utils.ConvertByteNumToResourceQuantity(data.Jvm.JavaMaxMetaspace)
	}

	// Calculate allocatable memory as max metaspace minus committed metaspace
	// Only if both values are valid (not -1)
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

// ConvertBaseMetadataToBaselineQuery converts heartbeat metadata to a baseline query
func ConvertBaseMetadataToBaselineQuery(data model.BaseMetadata) model.QueryBaselineRequest {
	return model.QueryBaselineRequest{
		Identity:    data.Identity,
		ClusterName: data.ClusterName,
		Version:     data.Version,
	}
}

// TranslateBizInfosToContainerStatuses converts biz info to container statuses
func TranslateBizInfosToContainerStatuses(data []ark.ArkBizInfo, changeTimestamp int64) []vkModel.BizStatusData {
	ret := make([]vkModel.BizStatusData, 0)
	for _, bizInfo := range data {
		updatedTime, reason, message := GetLatestState(bizInfo.BizStateRecords)
		statusData := vkModel.BizStatusData{
			Key:  GetBizIdentity(bizInfo.BizName, bizInfo.BizVersion),
			Name: bizInfo.BizName,
			// fille PodKey when using
			//PodKey:     vkModel.PodKeyAll,
			State: bizInfo.BizState,
			// TODO: 需要使用实际 bizState 变化的时间，而非心跳时间
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

// TranslateSimpleBizDataToBizInfos converts simple biz data to biz info
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

// TranslateSimpleBizDataToArkBizInfo converts simple biz data to ark biz info
func TranslateSimpleBizDataToArkBizInfo(data model.ArkSimpleBizInfoData) *ark.ArkBizInfo {
	return &ark.ArkBizInfo{
		BizName:         data.Name,
		BizVersion:      data.Version,
		BizState:        data.State,
		BizStateRecords: []ark.ArkBizStateRecord{data.LatestStateRecord},
	}
}

// GetLatestState finds the most recent state record and returns its details
func GetLatestState(records []ark.ArkBizStateRecord) (time.Time, string, string) {
	latestStateTime := int64(0)
	reason := ""
	message := ""
	for _, record := range records {
		changeTime := record.ChangeTime
		if changeTime > latestStateTime {
			latestStateTime = changeTime
			reason = record.Reason
			message = record.Message
		}
	}
	return time.UnixMilli(latestStateTime), reason, message
}

// OnBaseUnreachable handles cleanup when a base becomes unreachable
func OnBaseUnreachable(ctx context.Context, nodeName string, k8sClient client.Client) {
	// base not ready, delete from api server
	node := corev1.Node{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: nodeName}, &node)
	logger := zaplogger.FromContext(ctx).WithValues("nodeName", nodeName, "func", "OnNodeNotReady")
	if err == nil {
		// delete node from api server
		logger.Info("DeleteBaseNode")
		deleteErr := k8sClient.Delete(ctx, &node)
		if deleteErr != nil && !apiErrors.IsNotFound(err) {
			logger.Error(deleteErr, "delete base node failed")
		}
	} else if apiErrors.IsNotFound(err) {
		logger.Info("Node not found, skipping delete operation")
	} else {
		logger.Error(err, "Failed to get node, cannot delete")
	}
}

// ConvertBaseStatusFromNodeInfo extracts network info from node info
func ConvertBaseStatusFromNodeInfo(initData vkModel.NodeInfo) model.BaseStatus {
	portStr := initData.CustomLabels[model.LabelKeyOfTunnelPort]

	port, err := strconv.Atoi(portStr)
	if err != nil {
		zaplogger.GetLogger().Error(nil, fmt.Sprintf("failed to parse port %s from node info", portStr))
		port = 1238
	}

	return model.BaseStatus{
		BaseMetadata: model.BaseMetadata{
			Identity:    utils.ExtractNodeIDFromNodeName(initData.Metadata.Name),
			Version:     initData.Metadata.Version,
			ClusterName: initData.Metadata.ClusterName,
		},

		LocalIP:       initData.NetworkInfo.NodeIP,
		LocalHostName: initData.NetworkInfo.HostName,
		Port:          port,
		State:         string(initData.State),
	}
}
