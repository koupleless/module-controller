package utils

import (
	"fmt"
	"github.com/koupleless/arkctl/common/fileutil"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/virtual-kubelet/common/utils"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
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

func TranslateQueryAllBizDataToContainerStatuses(data []ark.ArkBizInfo) []vkModel.ContainerStatusData {
	ret := make([]vkModel.ContainerStatusData, 0)
	for _, bizInfo := range data {
		changeTime, reason, message := GetLatestState(bizInfo.BizState, bizInfo.BizStateRecords)
		ret = append(ret, vkModel.ContainerStatusData{
			Key:        GetBizIdentity(bizInfo.BizName, bizInfo.BizVersion),
			Name:       bizInfo.BizName,
			PodKey:     vkModel.PodKeyAll,
			State:      GetContainerStateFromBizState(bizInfo.BizState),
			ChangeTime: changeTime,
			Reason:     reason,
			Message:    message,
		})
	}
	return ret
}

func GetContainerStateFromBizState(bizState string) vkModel.ContainerState {
	switch bizState {
	case "ACTIVATED":
		return vkModel.ContainerStateActivated
	case "DEACTIVATED":
		return vkModel.ContainerStateDeactivated
	}
	return vkModel.ContainerStateResolved
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
