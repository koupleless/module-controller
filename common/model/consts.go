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
	// CommandBatchInstallBiz batch install biz, since koupleless-runtime 1.4.1
	CommandBatchInstallBiz = "batchInstallBiz"
)

// MQTT topic patterns for base communication
const (
	// BaseHeartBeatTopic for heartbeat messages, broadcast mode
	BaseHeartBeatTopic = "koupleless_%s/+/base/heart"
	// BaseQueryBaselineTopic for baseline queries, broadcast mode
	BaseQueryBaselineTopic = "koupleless_%s/+/base/queryBaseline"
	// BaseHealthTopic for health status, p2p mode
	BaseHealthTopic = "koupleless_%s/%s/base/health"
	// BaseSimpleBizTopic for simple business operations, p2p mode
	BaseSimpleBizTopic = "koupleless_%s/%s/base/simpleBiz"
	// BaseAllBizTopic for all business operations, p2p mode
	BaseAllBizTopic = "koupleless_%s/%s/base/biz"
	// BaseBizOperationResponseTopic for business operation responses, p2p mode
	BaseBizOperationResponseTopic = "koupleless_%s/%s/base/bizOperation"
	// BaseBatchInstallBizResponseTopic for response of batch install biz, p2p mode, since koupleless-runtime 1.4.1
	BaseBatchInstallBizResponseTopic = "koupleless_%s/%s/base/batchInstallBizResponse"
	// BaseBaselineResponseTopic for baseline responses, p2p mode
	BaseBaselineResponseTopic = "koupleless_%s/%s/base/baseline"
)

// Base labels
const (
	// LabelKeyOfTunnelPort specifies the tunnel port
	LabelKeyOfTunnelPort = "base.koupleless.io/tunnel-port"
)
