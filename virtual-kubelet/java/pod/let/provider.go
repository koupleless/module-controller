/**
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package let

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/koupleless/arkctl/v1/service/ark"
	"github.com/prometheus/client_model/go"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	"github.com/virtual-kubelet/virtual-kubelet/node/api/statsv1alpha1"
	"github.com/virtual-kubelet/virtual-kubelet/node/nodeutil"
	corev1 "k8s.io/api/core/v1"
)

var _ nodeutil.Provider = &BaseProvider{}

const LOOP_BACK_IP = "127.0.0.1"

type BaseProvider struct {
	namespace string

	arkService   ark.Service
	podCache     *LocalPodCache
	podConverter *Converter

	port int
}

func NewBaseProvider(namespace string) *BaseProvider {
	return &BaseProvider{
		namespace:  namespace,
		arkService: ark.BuildService(context.Background()),
		podCache: &LocalPodCache{
			RWMutex:           &sync.RWMutex{},
			cache:             make(map[string]*corev1.Pod),
			podNameToCacheKey: make(map[string]string),
		},
		podConverter: &Converter{},
		port:         1234,
	}
}

func (b *BaseProvider) queryAllBiz(ctx context.Context) ([]ark.ArkBizInfo, error) {
	resp, err := b.arkService.QueryAllBiz(ctx, ark.QueryAllArkBizRequest{
		HostName: LOOP_BACK_IP,
		Port:     b.port,
	})

	if err != nil {
		log.G(ctx).WithError(err).Error("QueryAllBizFailed")
		return nil, err
	}

	if resp.Code != "SUCCESS" {
		log.G(ctx).WithError(errors.New(resp.Message)).Error("QueryAllBizFailed")
		return nil, err
	}

	return resp.Data, nil
}

func (b *BaseProvider) queryBiz(ctx context.Context, bizName, bizVersion string) (*ark.ArkBizInfo, error) {
	bizInfos, err := b.queryAllBiz(ctx)
	if err != nil {
		return nil, err
	}

	for _, info := range bizInfos {
		if info.BizName == bizName && info.BizVersion == bizVersion {
			return &info, nil
		}
	}

	return nil, nil
}

// CreatePod directly install a biz bundle to base
func (b *BaseProvider) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	biz := b.podConverter.GetBizModel(pod)

	if err := b.podCache.Put(biz.BizName, biz.BizVersion, pod); err != nil {
		log.G(ctx).WithError(err).Error("CreatePodFailed")
		return err
	}

	if err := b.arkService.InstallBiz(ctx, ark.InstallBizRequest{
		BizModel: *biz,
		TargetContainer: ark.ArkContainerRuntimeInfo{
			RunType:    ark.ArkContainerRunTypeLocal,
			Coordinate: LOOP_BACK_IP,
			Port:       &b.port,
		},
	}); err != nil {
		b.podCache.Remove(biz.BizName, biz.BizVersion)
		log.G(ctx).WithError(err).Error("InstallBizFailed")
		return err
	}
	return nil
}

// UpdatePod uninstall then install
func (b *BaseProvider) UpdatePod(ctx context.Context, pod *corev1.Pod) error {
	biz := b.podConverter.GetBizModel(pod)
	bizInfo, err := b.queryBiz(ctx, biz.BizName, biz.BizVersion)
	if err != nil {
		log.G(ctx).WithError(err).Error("UpdatePodFailed")
		return err
	}

	if bizInfo != nil {
		if err := b.arkService.UnInstallBiz(ctx, ark.UnInstallBizRequest{
			BizModel: ark.BizModel{
				BizName:    bizInfo.BizName,
				BizVersion: bizInfo.BizVersion,
			},
			TargetContainer: ark.ArkContainerRuntimeInfo{
				RunType:    ark.ArkContainerRunTypeLocal,
				Coordinate: LOOP_BACK_IP,
				Port:       &b.port,
			},
		}); err != nil {
			log.G(ctx).WithError(err).Error("UnInstallBizFailed")
			return err
		}
		b.podCache.Remove(bizInfo.BizName, bizInfo.BizVersion)
	}

	if err := b.podCache.Put(bizInfo.BizName, bizInfo.BizVersion, pod); err != nil {
		log.G(ctx).WithError(err).Error("UpdateLocalPodFailed")
		return err
	}

	if err := b.arkService.InstallBiz(ctx, ark.InstallBizRequest{
		BizModel: *biz,
		TargetContainer: ark.ArkContainerRuntimeInfo{
			RunType:    ark.ArkContainerRunTypeLocal,
			Coordinate: LOOP_BACK_IP,
			Port:       &b.port,
		},
	}); err != nil {
		log.G(ctx).WithError(err).Error("InstallBizFailed")
		return err
	}
	return nil
}

// DeletePod install then uninstall
func (b *BaseProvider) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	biz := b.podConverter.GetBizModel(pod)
	bizInfo, err := b.queryBiz(ctx, biz.BizName, biz.BizVersion)
	if err != nil {
		return err
	}

	if bizInfo != nil {
		if err := b.arkService.UnInstallBiz(ctx, ark.UnInstallBizRequest{
			BizModel: ark.BizModel{
				BizName:    bizInfo.BizName,
				BizVersion: bizInfo.BizVersion,
			},
			TargetContainer: ark.ArkContainerRuntimeInfo{
				RunType:    ark.ArkContainerRunTypeLocal,
				Coordinate: LOOP_BACK_IP,
				Port:       &b.port,
			},
		}); err != nil {
			log.G(ctx).WithError(err).Error("UnInstallBizFailed")
			return err
		}
		b.podCache.Remove(bizInfo.BizName, bizInfo.BizVersion)
	}
	return nil
}

func (b *BaseProvider) GetPod(_ context.Context, _, name string) (*corev1.Pod, error) {
	return b.podCache.GetByPodName(name), nil
}

// GetPodStatus this will be called repeatedly by virtual kubelet framework to get the pod status
func (b *BaseProvider) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	pod, err := b.GetPod(ctx, namespace, name)
	if err != nil {
		return nil, err
	}

	if pod == nil {
		return nil, nil
	}
	return &pod.Status, nil
}

func (b *BaseProvider) GetPods(ctx context.Context) ([]*corev1.Pod, error) {
	bizs, err := b.queryAllBiz(ctx)
	if err != nil {
		return nil, err
	}

	pods := make([]*corev1.Pod, 0)
	for _, biz := range bizs {
		pod := b.podCache.Get(biz.BizName, biz.BizVersion)
		pods = append(pods, pod)
	}
	return pods, nil
}

func (b *BaseProvider) GetContainerLogs(ctx context.Context, namespace, podName, containerName string, opts api.ContainerLogOpts) (io.ReadCloser, error) {
	// todo: implement this by using the port to get the logs
	return nil, nil
}

func (b *BaseProvider) RunInContainer(ctx context.Context, namespace, podName, containerName string, cmd []string, attach api.AttachIO) error {
	panic("koupleless java virtual base does not support run")
}

func (b *BaseProvider) AttachToContainer(ctx context.Context, namespace, podName, containerName string, attach api.AttachIO) error {
	panic("koupleless java virtual base does not support attach")
}

func (b *BaseProvider) GetStatsSummary(ctx context.Context) (*statsv1alpha1.Summary, error) {
	return &statsv1alpha1.Summary{}, nil
}

func (b *BaseProvider) GetMetricsResource(ctx context.Context) ([]*io_prometheus_client.MetricFamily, error) {
	// todo: implement me
	return make([]*io_prometheus_client.MetricFamily, 0), nil
}

func (b *BaseProvider) PortForward(ctx context.Context, namespace, pod string, port int32, stream io.ReadWriteCloser) error {
	panic("koupleless java virtual base does not support port forward")
}
