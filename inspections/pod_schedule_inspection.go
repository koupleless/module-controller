package inspection

import (
	"context"
	"github.com/koupleless/module_controller/common/model"
	"github.com/koupleless/virtual-kubelet/common/tracker"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

// PodScheduleInspection inspect the pod always in pending status
type PodScheduleInspection struct {
	sync.Mutex

	kubeClient client.Client
	ownerMap   map[string]int64
}

func (p *PodScheduleInspection) Register(kubeClient client.Client) {
	p.kubeClient = kubeClient
	p.ownerMap = make(map[string]int64)
}

func (p *PodScheduleInspection) GetInterval() time.Duration {
	return time.Second * 60
}

func (p *PodScheduleInspection) Inspect(ctx context.Context, env string) {
	requirement, _ := labels.NewRequirement(vkModel.LabelKeyOfComponent, selection.In, []string{model.ComponentModule})
	envRequirement, _ := labels.NewRequirement(vkModel.LabelKeyOfEnv, selection.In, []string{env})

	// get all module pods with pending phase
	modulePods := &v1.PodList{}
	err := p.kubeClient.List(ctx, modulePods, &client.ListOptions{
		LabelSelector: labels.NewSelector().Add(*requirement, *envRequirement),
		FieldSelector: fields.OneTermEqualSelector("status.phase", string(v1.PodPending)),
	})
	if err != nil {
		logrus.Errorf("failed to list pods: %v", err)
		return
	}

	for _, pod := range modulePods.Items {
		// select pods with scheduled but schedule failed
		if len(pod.Status.Conditions) != 0 && pod.Status.Conditions[0].Type == v1.PodScheduled && pod.Status.Conditions[0].Status == v1.ConditionFalse {
			// check owner has been reported
			if len(pod.OwnerReferences) != 0 && p.ownerReported(string(pod.OwnerReferences[0].UID), 10*60) {
				continue
			}
			tracker.G().ErrorReport(pod.Labels[vkModel.LabelKeyOfTraceID], vkModel.TrackSceneVPodDeploy, model.TrackEventModuleSchedule, pod.Status.Conditions[0].Message, pod.Labels, model.CodeVPodScheduleFailed)
		}
	}
}

func (p *PodScheduleInspection) ownerReported(uid string, expireSeconds int64) bool {
	p.Lock()
	defer p.Unlock()
	now := time.Now().Unix()
	if p.ownerMap[uid] >= now-expireSeconds {
		return true
	}
	p.ownerMap[uid] = now
	return false
}
