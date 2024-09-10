package utils

import (
	"fmt"
	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/koupleless/module_controller/common/model"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	corev1 "k8s.io/api/core/v1"
	"reflect"
	"testing"
	"time"
)

// Test cases for GetBaseIDFromTopic function
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
			actual := GetBaseIDFromTopic(tt.topic)
			if actual != tt.expected {
				t.Errorf("GetBaseIDFromTopic(%q) = %q; expected %q", tt.topic, actual, tt.expected)
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
				Image: "test-image",
				Env: []corev1.EnvVar{
					{Name: "BIZ_VERSION", Value: "1.0.0"},
				},
			},
			expected: ark.BizModel{
				BizName:    "test-container",
				BizVersion: "1.0.0",
				BizUrl:     "test-image",
			},
		},
		{
			container: &corev1.Container{
				Name:  "another-container",
				Image: "another-image",
				Env: []corev1.EnvVar{
					{Name: "OTHER_ENV", Value: "foo"},
					{Name: "BIZ_VERSION", Value: "2.0.0"},
				},
			},
			expected: ark.BizModel{
				BizName:    "another-container",
				BizVersion: "2.0.0",
				BizUrl:     "another-image",
			},
		},
		{
			container: &corev1.Container{
				Name:  "empty-version-container",
				Image: "empty-version-image",
				Env: []corev1.EnvVar{
					{Name: "OTHER_ENV", Value: "bar"},
				},
			},
			expected: ark.BizModel{
				BizName:    "empty-version-container",
				BizVersion: "",
				BizUrl:     "empty-version-image",
			},
		},
		{
			container: &corev1.Container{
				Name:  "no-env-container",
				Image: "no-env-image",
				Env:   []corev1.EnvVar{},
			},
			expected: ark.BizModel{
				BizName:    "no-env-container",
				BizVersion: "",
				BizUrl:     "no-env-image",
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

// Test cases for TranslateHeartBeatDataToNodeInfo function
func TestTranslateHeartBeatDataToNodeInfo(t *testing.T) {
	tests := []struct {
		data     model.HeartBeatData
		expected vkModel.NodeInfo
	}{
		{
			data: model.HeartBeatData{
				State: "ACTIVATED",
				MasterBizInfo: model.Metadata{
					Name:    "biz1",
					Version: "1.0.0",
				},
				NetworkInfo: model.NetworkInfo{
					LocalIP:       "192.168.1.1",
					LocalHostName: "host1",
				},
			},
			expected: vkModel.NodeInfo{
				Metadata: vkModel.NodeMetadata{
					Name:    "biz1",
					Version: "1.0.0",
					Status:  vkModel.NodeStatusActivated,
				},
				NetworkInfo: vkModel.NetworkInfo{
					NodeIP:   "192.168.1.1",
					HostName: "host1",
				},
			},
		},
		{
			data: model.HeartBeatData{
				State: "DEACTIVATED",
				MasterBizInfo: model.Metadata{
					Name:    "biz2",
					Version: "2.0.0",
				},
				NetworkInfo: model.NetworkInfo{
					LocalIP:       "192.168.1.2",
					LocalHostName: "host2",
				},
			},
			expected: vkModel.NodeInfo{
				Metadata: vkModel.NodeMetadata{
					Name:    "biz2",
					Version: "2.0.0",
					Status:  vkModel.NodeStatusDeactivated,
				},
				NetworkInfo: vkModel.NetworkInfo{
					NodeIP:   "192.168.1.2",
					HostName: "host2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.data.MasterBizInfo.Name, func(t *testing.T) {
			actual := TranslateHeartBeatDataToNodeInfo(tt.data)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("TranslateHeartBeatDataToNodeInfo() = %+v; expected %+v", actual, tt.expected)
			}
		})
	}
}

// Test cases for TranslateQueryAllBizDataToContainerStatuses function
func TestTranslateQueryAllBizDataToContainerStatuses(t *testing.T) {
	tests := []struct {
		data     []ark.ArkBizInfo
		expected []vkModel.ContainerStatusData
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
							ChangeTime: "2022-01-01 00:00:00.000",
							Reason:     "started",
							Message:    "Biz started",
						},
					},
				},
			},
			expected: []vkModel.ContainerStatusData{
				{
					Key:        "biz1:1.0.0",
					Name:       "biz1",
					PodKey:     vkModel.PodKeyAll,
					State:      vkModel.ContainerStateActivated,
					ChangeTime: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
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
							ChangeTime: "2022-02-01 00:00:00.000",
							Reason:     "stopped",
							Message:    "Biz stopped",
						},
					},
				},
			},
			expected: []vkModel.ContainerStatusData{
				{
					Key:        "biz2:2.0.0",
					Name:       "biz2",
					PodKey:     vkModel.PodKeyAll,
					State:      vkModel.ContainerStateDeactivated,
					ChangeTime: time.Date(2022, 2, 1, 0, 0, 0, 0, time.UTC),
					Reason:     "stopped",
					Message:    "Biz stopped",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.data[0].BizName, func(t *testing.T) {
			actual := TranslateQueryAllBizDataToContainerStatuses(tt.data)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("TranslateQueryAllBizDataToContainerStatuses() = %+v; expected %+v", actual, tt.expected)
			}
		})
	}
}

func TestGetContainerStateFromBizState(t *testing.T) {
	testCases := []struct {
		input    string
		expected vkModel.ContainerState
	}{
		{"ACTIVATED", vkModel.ContainerStateActivated},
		{"DEACTIVATED", vkModel.ContainerStateDeactivated},
		{"UNKNOWN", vkModel.ContainerStateResolved},
	}

	for _, tc := range testCases {
		result := GetContainerStateFromBizState(tc.input)
		if result != tc.expected {
			t.Errorf("GetContainerStateFromBizState(%s) = %v; want %v", tc.input, result, tc.expected)
		}
	}
}

func TestGetLatestState(t *testing.T) {
	// Mock data for testing
	records := []ark.ArkBizStateRecord{
		{
			State:      "state1",
			ChangeTime: "2023-10-10 15:04:05.000",
			Reason:     "Reason1",
			Message:    "Message1",
		},
		{
			State:      "state2",
			ChangeTime: "2023-10-10 16:04:05.000",
			Reason:     "Reason2",
			Message:    "Message2",
		},
		{
			State:      "state1",
			ChangeTime: "2023-10-10 17:04:05.000",
			Reason:     "Reason3",
			Message:    "Message3",
		},
	}

	expectedTime := time.Date(2023, 10, 10, 17, 4, 5, 0, time.UTC)
	expectedReason := "Reason3"
	expectedMessage := "Message3"

	latestTime, reason, message := GetLatestState("state1", records)

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
			ChangeTime: "2023-10-10 15:04:05.000",
			Reason:     "Reason1",
			Message:    "Message1",
		},
	}

	expectedTime := time.UnixMilli(0)
	expectedReason := ""
	expectedMessage := ""

	latestTime, reason, message := GetLatestState("state1", records)

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
			ChangeTime: "invalid_time",
			Reason:     "Reason1",
			Message:    "Message1",
		},
		{
			State:      "state1",
			ChangeTime: "2023-10-10 15:04:05.000",
			Reason:     "Reason2",
			Message:    "Message2",
		},
	}

	expectedTime := time.Date(2023, 10, 10, 15, 4, 5, 0, time.UTC)
	expectedReason := "Reason2"
	expectedMessage := "Message2"

	latestTime, reason, message := GetLatestState("state1", records)

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
