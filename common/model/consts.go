package model

import "github.com/koupleless/virtual-kubelet/model"

// TrackEvent constants
const (
	// TrackEventVPodPeerDeploymentReplicaModify tracks when a peer deployment's replicas are modified
	TrackEventVPodPeerDeploymentReplicaModify = "PeerDeploymentReplicaModify"
)

// Label keys for module controller
const (
	// LabelKeyOfSkipReplicasControl indicates whether to skip replicas control
	LabelKeyOfSkipReplicasControl = "virtual-kubelet.koupleless.io/replicas-control"
	// LabelKeyOfVPodDeploymentStrategy specifies the deployment strategy
	LabelKeyOfVPodDeploymentStrategy = "virtual-kubelet.koupleless.io/strategy"
)

// Component types
const (
	// ComponentModule represents a module component
	ComponentModule = "module"
	// ComponentModuleDeployment represents a module deployment component
	ComponentModuleDeployment = "module-deployment"
)

// VPodDeploymentStrategy defines deployment strategies for VPods
type VPodDeploymentStrategy string

// Available VPod deployment strategies
const (
	// VPodDeploymentStrategyPeer indicates peer deployment strategy
	VPodDeploymentStrategyPeer VPodDeploymentStrategy = "peer"
)

// Error codes
const (
	// CodeKubernetesOperationFailed indicates a Kubernetes operation failure
	CodeKubernetesOperationFailed model.ErrorCode = "00003"
)

// Command types for module operations
const (
	// CommandHealth checks module health
	CommandHealth = "health"
	// CommandQueryAllBiz queries all business modules
	CommandQueryAllBiz = "queryAllBiz"
	// CommandInstallBiz installs a business module
	CommandInstallBiz = "installBiz"
	// CommandUnInstallBiz uninstalls a business module
	CommandUnInstallBiz = "uninstallBiz"
)

// MQTT topic patterns for base communication
const (
	// BaseHeartBeatTopic for heartbeat messages
	BaseHeartBeatTopic = "koupleless_%s/+/base/heart"
	// BaseQueryBaselineTopic for baseline queries
	BaseQueryBaselineTopic = "koupleless_%s/+/base/queryBaseline"
	// BaseHealthTopic for health status
	BaseHealthTopic = "koupleless_%s/%s/base/health"
	// BaseSimpleBizTopic for simple business operations
	BaseSimpleBizTopic = "koupleless_%s/%s/base/simpleBiz"
	// BaseAllBizTopic for all business operations
	BaseAllBizTopic = "koupleless_%s/%s/base/biz"
	// BaseBizOperationResponseTopic for business operation responses
	BaseBizOperationResponseTopic = "koupleless_%s/%s/base/bizOperation"
	// BaseBaselineResponseTopic for baseline responses
	BaseBaselineResponseTopic = "koupleless_%s/%s/base/baseline"
)

// Base labels
const (
	// LabelKeyOfTechStack specifies the technology stack
	LabelKeyOfTechStack = "base.koupleless.io/stack"
	// LabelKeyOfArkletPort specifies the arklet port
	LabelKeyOfArkletPort = "base.koupleless.io/arklet-port"
)
