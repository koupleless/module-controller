package model

import "github.com/koupleless/virtual-kubelet/model"

const (
	TrackEventVPodPeerDeploymentReplicaModify = "PeerDeploymentReplicaModify"
)

const (
	LabelKeyOfSkipReplicasControl    = "module-controller.koupleless.io/replicas-control"
	LabelKeyOfVPodDeploymentStrategy = "module-controller.koupleless.io/strategy"
)

const (
	ComponentModule           = "module"
	ComponentModuleDeployment = "module-deployment"
)

type VPodDeploymentStrategy string

const (
	VPodDeploymentStrategyPeer VPodDeploymentStrategy = "peer"
)

const (
	CodeKubernetesOperationFailed model.ErrorCode = "00003"
)

const (
	CommandHealth       = "health"
	CommandQueryAllBiz  = "queryAllBiz"
	CommandInstallBiz   = "installBiz"
	CommandUnInstallBiz = "uninstallBiz"
)

const (
	BaseHeartBeatTopic            = "koupleless_%s/+/base/heart"
	BaseQueryBaselineTopic        = "koupleless_%s/+/base/queryBaseline"
	BaseHealthTopic               = "koupleless_%s/%s/base/health"
	BaseSimpleBizTopic            = "koupleless_%s/%s/base/simpleBiz"
	BaseAllBizTopic               = "koupleless_%s/%s/base/biz"
	BaseBizOperationResponseTopic = "koupleless_%s/%s/base/bizOperation"
	BaseBaselineResponseTopic     = "koupleless_%s/%s/base/baseline"
)

const (
	LabelKeyOfTechStack = "base.koupleless.io/stack"
)
