package utils

import (
	"context"
	"fmt"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/virtual-kubelet/common/utils"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

// Test cases for GetBaseIdentityFromTopic function
func TestGetBaseIDFromTopic(t *testing.T) {
	tests := []struct {
		topic    string
		expected string
	}{
		{topic: "devices/12345/data", expected: "12345"}, // Normal case
		{topic: "a/b", expected: "b"},                    // Just two fields
		{topic: "a", expected: ""},                       // Not enough fields
		{topic: "", expected: ""},                        // Empty string
		{topic: "abc", expected: ""},                     // No delimiter
		{topic: "one/two/three", expected: "two"},        // Multiple fields
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			actual := GetBaseIdentityFromTopic(tt.topic)
			if actual != tt.expected {
				t.Errorf("GetBaseIdentityFromTopic(%q) = %q; expected %q", tt.topic, actual, tt.expected)
			}
		})
	}
}

// Test cases for Expired function
func TestExpired(t *testing.T) {
	// Mocking the current time to have consistent test results
	currentTime := time.Now().UnixMilli()

	tests := []struct {
		publishTimestamp int64
		maxLiveMilliSec  int64
		expected         bool
	}{
		{publishTimestamp: currentTime - 10000, maxLiveMilliSec: 5000, expected: true},  // Expired
		{publishTimestamp: currentTime - 4000, maxLiveMilliSec: 5000, expected: false},  // Not expired yet
		{publishTimestamp: currentTime, maxLiveMilliSec: 5000, expected: false},         // Just published
		{publishTimestamp: currentTime - 5000, maxLiveMilliSec: 5000, expected: true},   // Exactly expired
		{publishTimestamp: currentTime - 6000, maxLiveMilliSec: 10000, expected: false}, // Far from expiring
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			actual := Expired(tt.publishTimestamp, tt.maxLiveMilliSec)
			if actual != tt.expected {
				t.Errorf("Expired(%d, %d) = %v; expected %v", tt.publishTimestamp, tt.maxLiveMilliSec, actual, tt.expected)
			}
		})
	}
}

// Test cases for FormatArkletCommandTopic function
func TestFormatArkletCommandTopic(t *testing.T) {
	tests := []struct {
		env      string
		nodeID   string
		command  string
		expected string
	}{
		{env: "production", nodeID: "node123", command: "start", expected: "koupleless_production/node123/start"}, // Normal case
		{env: "test", nodeID: "node456", command: "stop", expected: "koupleless_test/node456/stop"},               // Different environment
		{env: "dev", nodeID: "node789", command: "restart", expected: "koupleless_dev/node789/restart"},           // Different command
		{env: "", nodeID: "node000", command: "update", expected: "koupleless_/node000/update"},                   // Empty environment
		{env: "stage", nodeID: "", command: "status", expected: "koupleless_stage//status"},                       // Empty nodeID
		{env: "qa", nodeID: "node999", command: "", expected: "koupleless_qa/node999/"},                           // Empty command
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("env=%s,nodeID=%s,command=%s", tt.env, tt.nodeID, tt.command), func(t *testing.T) {
			actual := FormatArkletCommandTopic(tt.env, tt.nodeID, tt.command)
			if actual != tt.expected {
				t.Errorf("FormatArkletCommandTopic(%q, %q, %q) = %q; expected %q", tt.env, tt.nodeID, tt.command, actual, tt.expected)
			}
		})
	}
}

// Test cases for FormatBaselineResponseTopic function
func TestFormatBaselineResponseTopic(t *testing.T) {
	tests := []struct {
		env      string
		nodeID   string
		expected string
	}{
		{env: "production", nodeID: "node123", expected: "koupleless_production/node123/base/baseline"},
		{env: "test", nodeID: "node456", expected: "koupleless_test/node456/base/baseline"},
		{env: "dev", nodeID: "node789", expected: "koupleless_dev/node789/base/baseline"},
		{env: "", nodeID: "node000", expected: "koupleless_/node000/base/baseline"},
		{env: "stage", nodeID: "", expected: "koupleless_stage//base/baseline"},
		{env: "qa", nodeID: "node999", expected: "koupleless_qa/node999/base/baseline"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("env=%s,nodeID=%s", tt.env, tt.nodeID), func(t *testing.T) {
			actual := FormatBaselineResponseTopic(tt.env, tt.nodeID)
			if actual != tt.expected {
				t.Errorf("FormatBaselineResponseTopic(%q, %q) = %q; expected %q", tt.env, tt.nodeID, actual, tt.expected)
			}
		})
	}
}

// Test cases for GetBizVersionFromContainer function
func TestGetBizVersionFromContainer(t *testing.T) {
	tests := []struct {
		container *corev1.Container
		expected  string
	}{
		{
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{Name: "BIZ_VERSION", Value: "1.0.0"},
				},
			},
			expected: "1.0.0",
		},
		{
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{Name: "OTHER_ENV", Value: "foo"},
					{Name: "BIZ_VERSION", Value: "2.0.0"},
				},
			},
			expected: "2.0.0",
		},
		{
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{Name: "OTHER_ENV", Value: "bar"},
				},
			},
			expected: "",
		},
		{
			container: &corev1.Container{
				Env: []corev1.EnvVar{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			actual := GetBizVersionFromContainer(tt.container)
			if actual != tt.expected {
				t.Errorf("GetBizVersionFromContainer() = %q; expected %q", actual, tt.expected)
			}
		})
	}
}

// Test cases for TranslateCoreV1ContainerToBizModel function
func TestTranslateCoreV1ContainerToBizModel(t *testing.T) {
	tests := []struct {
		container *corev1.Container
		expected  ark.BizModel
	}{
		{
			container: &corev1.Container{
				Name:  "test-container",
				Image: "test-image.jar",
				Env: []corev1.EnvVar{
					{Name: "BIZ_VERSION", Value: "1.0.0"},
				},
			},
			expected: ark.BizModel{
				BizName:    "test-container",
				BizVersion: "1.0.0",
				BizUrl:     "test-image.jar",
			},
		},
		{
			container: &corev1.Container{
				Name:  "another-container",
				Image: "another-image.jar",
				Env: []corev1.EnvVar{
					{Name: "OTHER_ENV", Value: "foo"},
					{Name: "BIZ_VERSION", Value: "2.0.0"},
				},
			},
			expected: ark.BizModel{
				BizName:    "another-container",
				BizVersion: "2.0.0",
				BizUrl:     "another-image.jar",
			},
		},
		{
			container: &corev1.Container{
				Name:  "empty-version-container",
				Image: "empty-version-image.jar",
				Env: []corev1.EnvVar{
					{Name: "OTHER_ENV", Value: "bar"},
				},
			},
			expected: ark.BizModel{
				BizName:    "empty-version-container",
				BizVersion: "",
				BizUrl:     "empty-version-image.jar",
			},
		},
		{
			container: &corev1.Container{
				Name:  "no-env-container",
				Image: "no-env-image.jar",
				Env:   []corev1.EnvVar{},
			},
			expected: ark.BizModel{
				BizName:    "no-env-container",
				BizVersion: "",
				BizUrl:     "no-env-image.jar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.container.Name, func(t *testing.T) {
			actual := TranslateCoreV1ContainerToBizModel(tt.container)
			if actual != tt.expected {
				t.Errorf("TranslateCoreV1ContainerToBizModel() = %+v; expected %+v", actual, tt.expected)
			}
		})
	}
}

// Test cases for GetBizIdentity function
func TestGetBizIdentity(t *testing.T) {
	tests := []struct {
		bizName    string
		bizVersion string
		expected   string
	}{
		{bizName: "mybiz", bizVersion: "1.0.0", expected: "mybiz:1.0.0"},
		{bizName: "anotherbiz", bizVersion: "2.1.5", expected: "anotherbiz:2.1.5"},
		{bizName: "testbiz", bizVersion: "", expected: "testbiz:"},
		{bizName: "", bizVersion: "3.0.0", expected: ":3.0.0"},
		{bizName: "", bizVersion: "", expected: ":"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.bizName, tt.bizVersion), func(t *testing.T) {
			actual := GetBizIdentity(tt.bizName, tt.bizVersion)
			if actual != tt.expected {
				t.Errorf("GetBizIdentity(%q, %q) = %q; expected %q", tt.bizName, tt.bizVersion, actual, tt.expected)
			}
		})
	}
}

// Test cases for ConvertBaseStatusToNodeInfo function
func TestTranslateHeartBeatDataToNodeInfo(t *testing.T) {
	env := "test"
	tests := []struct {
		data     model.BaseStatus
		expected vkModel.NodeInfo
	}{
		{
			data: model.BaseStatus{
				BaseMetadata: model.BaseMetadata{
					Identity:    "base1",
					ClusterName: "base",
					Version:     "1.0.0",
				},
				LocalIP:       "192.168.1.1",
				LocalHostName: "host1",
				Port:          1238,
				State:         "ACTIVATED",
			},
			expected: vkModel.NodeInfo{
				Metadata: vkModel.NodeMetadata{
					Name:        utils.FormatNodeName("base1", env),
					Version:     "1.0.0",
					ClusterName: "base",
				},
				NetworkInfo: vkModel.NetworkInfo{
					NodeIP:   "192.168.1.1",
					HostName: "host1",
				},
				CustomLabels: map[string]string{
					model.LabelKeyOfTunnelPort: "1238",
				},
				State: vkModel.NodeStateActivated,
			},
		},
		{
			data: model.BaseStatus{
				BaseMetadata: model.BaseMetadata{
					Identity:    "base2",
					ClusterName: "base",
					Version:     "2.0.0",
				},
				LocalIP:       "192.168.1.2",
				LocalHostName: "host2",
				State:         "DEACTIVATED",
			},
			expected: vkModel.NodeInfo{
				Metadata: vkModel.NodeMetadata{
					Name:        utils.FormatNodeName("base2", env),
					Version:     "2.0.0",
					ClusterName: "base",
				},
				NetworkInfo: vkModel.NetworkInfo{
					NodeIP:   "192.168.1.2",
					HostName: "host2",
				},
				CustomLabels: map[string]string{},
				State:        vkModel.NodeStateDeactivated,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.data.BaseMetadata.Identity, func(t *testing.T) {
			actual := ConvertBaseStatusToNodeInfo(tt.data, env)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("ConvertBaseStatusToNodeInfo() = %+v; expected %+v", actual, tt.expected)
			}
		})
	}
}

// Test cases for TranslateSimpleBizDataToBizInfos function
func TestTranslateQueryAllBizDataToContainerStatuses(t *testing.T) {
	tests := []struct {
		data     []ark.ArkBizInfo
		expected []vkModel.BizStatusData
	}{
		{
			data: []ark.ArkBizInfo{
				{
					BizName:    "biz1",
					BizVersion: "1.0.0",
					BizState:   "ACTIVATED",
					BizStateRecords: []ark.ArkBizStateRecord{
						{
							State:      "ACTIVATED",
							ChangeTime: 1234,
							Reason:     "started",
							Message:    "Biz started",
						},
					},
				},
			},
			expected: []vkModel.BizStatusData{
				{
					Key:  "biz1:1.0.0",
					Name: "biz1",
					//PodKey:     vkModel.PodKeyAll,
					State:      string(vkModel.BizStateActivated),
					ChangeTime: time.UnixMilli(1234),
					Reason:     "started",
					Message:    "Biz started",
				},
			},
		},
		{
			data: []ark.ArkBizInfo{
				{
					BizName:    "biz2",
					BizVersion: "2.0.0",
					BizState:   "DEACTIVATED",
					BizStateRecords: []ark.ArkBizStateRecord{
						{
							State:      "DEACTIVATED",
							ChangeTime: 1234,
							Reason:     "stopped",
							Message:    "Biz stopped",
						},
					},
				},
			},
			expected: []vkModel.BizStatusData{
				{
					Key:  "biz2:2.0.0",
					Name: "biz2",
					//PodKey:     vkModel.PodKeyAll,
					State:      string(vkModel.BizStateDeactivated),
					ChangeTime: time.UnixMilli(1234),
					Reason:     "stopped",
					Message:    "Biz stopped",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.data[0].BizName, func(t *testing.T) {
			actual := TranslateBizInfosToContainerStatuses(tt.data, 0)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("TranslateSimpleBizDataToBizInfos() = %+v; expected %+v", actual, tt.expected)
			}
		})
	}
}

func TestGetLatestState(t *testing.T) {
	// Mock data for testing
	records := []ark.ArkBizStateRecord{
		{
			State:      "state1",
			ChangeTime: 1234,
			Reason:     "Reason1",
			Message:    "Message1",
		},
		{
			State:      "state2",
			ChangeTime: 12345,
			Reason:     "Reason2",
			Message:    "Message2",
		},
		{
			State:      "state1",
			ChangeTime: 123456,
			Reason:     "Reason3",
			Message:    "Message3",
		},
	}

	expectedTime := time.UnixMilli(123456)
	expectedReason := "Reason3"
	expectedMessage := "Message3"

	latestTime, reason, message := GetLatestState(records)

	if !latestTime.Equal(expectedTime) {
		t.Errorf("Expected latest time %v, got %v", expectedTime, latestTime)
	}
	if reason != expectedReason {
		t.Errorf("Expected reason %s, got %s", expectedReason, reason)
	}
	if message != expectedMessage {
		t.Errorf("Expected message %s, got %s", expectedMessage, message)
	}
}

func TestGetLatestStateNoMatchingRecords(t *testing.T) {
	// Mock data for testing
	records := []ark.ArkBizStateRecord{
		{
			State:      "state2",
			ChangeTime: -1234,
			Reason:     "Reason1",
			Message:    "Message1",
		},
	}

	expectedTime := time.UnixMilli(0)
	expectedReason := ""
	expectedMessage := ""

	latestTime, reason, message := GetLatestState(records)

	if !latestTime.Equal(expectedTime) {
		t.Errorf("Expected latest time %v, got %v", expectedTime, latestTime)
	}
	if reason != expectedReason {
		t.Errorf("Expected reason %s, got %s", expectedReason, reason)
	}
	if message != expectedMessage {
		t.Errorf("Expected message %s, got %s", expectedMessage, message)
	}
}

func TestGetLatestStateInvalidChangeTime(t *testing.T) {
	// Mock data for testing
	records := []ark.ArkBizStateRecord{
		{
			State:      "state1",
			ChangeTime: 0,
			Reason:     "Reason1",
			Message:    "Message1",
		},
		{
			State:      "state1",
			ChangeTime: 1234,
			Reason:     "Reason2",
			Message:    "Message2",
		},
	}

	expectedTime := time.UnixMilli(1234)
	expectedReason := "Reason2"
	expectedMessage := "Message2"

	latestTime, reason, message := GetLatestState(records)

	if !latestTime.Equal(expectedTime) {
		t.Errorf("Expected latest time %v, got %v", expectedTime, latestTime)
	}
	if reason != expectedReason {
		t.Errorf("Expected reason %s, got %s", expectedReason, reason)
	}
	if message != expectedMessage {
		t.Errorf("Expected message %s, got %s", expectedMessage, message)
	}
}

func TestTranslateHealthDataToNodeStatus(t *testing.T) {
	testCases := []struct {
		input    ark.HealthData
		expected vkModel.NodeStatusData
	}{
		{
			input: ark.HealthData{
				Jvm: ark.JvmInfo{
					JavaMaxMetaspace:       1024,
					JavaCommittedMetaspace: 0,
				},
			},
			expected: vkModel.NodeStatusData{
				Resources: map[corev1.ResourceName]vkModel.NodeResource{
					corev1.ResourceMemory: {
						Capacity:    resource.MustParse("1Ki"),
						Allocatable: resource.MustParse("1Ki"),
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		result := ConvertHealthDataToNodeStatus(tc.input)
		if result.Resources[corev1.ResourceMemory].Capacity != tc.expected.Resources[corev1.ResourceMemory].Capacity {
			t.Errorf("ConvertHealthDataToNodeStatus(%v) = %v; want %v", tc.input, result, tc.expected)
		}
		if result.Resources[corev1.ResourceMemory].Allocatable != tc.expected.Resources[corev1.ResourceMemory].Allocatable {
			t.Errorf("ConvertHealthDataToNodeStatus(%v) = %v; want %v", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateHeartBeatDataToBaselineQuery(t *testing.T) {
	testCases := []struct {
		input    model.BaseMetadata
		expected model.QueryBaselineRequest
	}{
		{
			input: model.BaseMetadata{
				Identity:    "base1",
				ClusterName: "test",
				Version:     "1.0.0",
			},
			expected: model.QueryBaselineRequest{
				Identity:    "base1",
				ClusterName: "test",
				Version:     "1.0.0",
			},
		},
	}

	for _, tc := range testCases {
		result := ConvertBaseMetadataToBaselineQuery(tc.input)
		if result.ClusterName != tc.expected.ClusterName || result.Version != tc.expected.Version {
			t.Errorf("ConvertBaseMetadataToBaselineQuery(%s) = %v; want %v", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateSimpleBizDataToArkBizInfos(t *testing.T) {
	testCases := []struct {
		input    model.ArkSimpleAllBizInfoData
		expected []ark.ArkBizInfo
	}{
		{
			input: model.ArkSimpleAllBizInfoData{
				model.ArkSimpleBizInfoData{
					Name: "biz1", Version: "0.0.1", State: string(vkModel.BizStateActivated),
				},
				model.ArkSimpleBizInfoData{},
			},
			expected: []ark.ArkBizInfo{
				{
					BizName:    "biz1",
					BizState:   string(vkModel.BizStateActivated),
					BizVersion: "0.0.1",
				}, {},
			},
		},
	}

	for _, tc := range testCases {
		result := TranslateSimpleBizDataToBizInfos(tc.input)
		assert.Equal(t, len(result), len(tc.expected), fmt.Errorf("ConvertBaseMetadataToBaselineQuery(%v) = %v; want %v", tc.input, result, tc.expected))
	}
}

func TestTranslateSimpleBizDataToArkBizInfo(t *testing.T) {
	info := TranslateSimpleBizDataToArkBizInfo(model.ArkSimpleBizInfoData{})
	assert.NotNil(t, info)
	info = TranslateSimpleBizDataToArkBizInfo(model.ArkSimpleBizInfoData{
		Name: "biz1", Version: "0.0.1", State: "activated",
	})
	assert.NotNil(t, info)
}

func TestGetLatestState_ChangeTimeLenLt3(t *testing.T) {
	updatedTime, reason, message := GetLatestState([]ark.ArkBizStateRecord{
		{
			State:      "ACTIVATED",
			ChangeTime: 0,
		},
	})
	assert.Zero(t, updatedTime.UnixMilli())
	assert.Empty(t, reason)
	assert.Empty(t, message)
}

func TestExtractNetworkInfoFromNodeInfoData(t *testing.T) {
	data := ConvertBaseStatusFromNodeInfo(vkModel.NodeInfo{
		CustomLabels: map[string]string{
			model.LabelKeyOfTunnelPort: ";",
		},
	})
	assert.Equal(t, data.Port, 1238)
}

func TestOnBaseUnreachable(t *testing.T) {
	OnBaseUnreachable(context.Background(), "test-node-name", fake.NewFakeClient())
}
