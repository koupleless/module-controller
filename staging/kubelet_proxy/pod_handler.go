package kubelet_proxy

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/koupleless/virtual-kubelet/common/utils"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/virtual-kubelet/virtual-kubelet/errdefs"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node/api"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

func NewPodHandler(clientSet kubernetes.Interface) (*PodHandler, error) {
	if clientSet == nil {
		return nil, fmt.Errorf("clientSet can't be nil")
	}
	return &PodHandler{clientSet: clientSet}, nil
}

type PodHandler struct {
	clientSet kubernetes.Interface // Kubernetes clientSet to interact with the cluster
}

func (f *PodHandler) AttachPodRoutes(mux *http.ServeMux) {
	mux.Handle("/", api.PodHandler(api.PodHandlerConfig{
		GetContainerLogs: f.getContainerLogs,
	}, true))
}

func (f *PodHandler) getContainerLogs(
	ctx context.Context,
	namespace string,
	podName string,
	containerName string,
	opts api.ContainerLogOpts,
) (io.ReadCloser, error) {
	log.L.Debugf("Forwarding log request for pod %s in namespace %s, container %s", podName, namespace, containerName)
	basePod, err := f.lookupBasePod(ctx, namespace, podName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, errdefs.NotFound(err.Error())
		}
		return nil, err
	}

	podLogOptions := &corev1.PodLogOptions{
		Container:  utils.OrElse(basePod.Labels[vkModel.LabelKeyOfBaseContainerName], "base"),
		Follow:     opts.Follow,
		Previous:   opts.Previous,
		Timestamps: opts.Timestamps,
		TailLines: func() *int64 {
			if opts.Tail > 0 {
				return ptr.To[int64](int64(opts.Tail))
			}
			return nil
		}(),
		LimitBytes: func() *int64 {
			if opts.LimitBytes > 0 {
				return ptr.To[int64](int64(opts.LimitBytes))
			}
			return nil
		}(),
		SinceTime: func() *metav1.Time {
			if !opts.SinceTime.IsZero() {
				return &metav1.Time{Time: opts.SinceTime}
			}
			return nil
		}(),
		SinceSeconds: func() *int64 {
			if opts.SinceSeconds > 0 {
				return ptr.To[int64](int64(opts.SinceSeconds))
			}
			return nil
		}(),
	}
	rc, err := f.clientSet.CoreV1().Pods(basePod.Namespace).GetLogs(basePod.Name, podLogOptions).Stream(ctx)
	if err != nil {
		return nil, err
	}
	return rc, nil
}

// lookupBasePod retrieves the base pod for a given virtual pod name in the specified namespace.
func (f *PodHandler) lookupBasePod(ctx context.Context, namespace, vPodName string) (*corev1.Pod, error) {
	vPod, err := f.clientSet.CoreV1().Pods(namespace).Get(ctx, vPodName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewNotFound(corev1.Resource("pods"), vPodName)
		}
		log.L.Errorf("Fail to pod %s in namespace %s: %v", vPodName, namespace, err)
		return nil, err
	}

	vNodeName := vPod.Spec.NodeName

	vNode, err := f.clientSet.CoreV1().Nodes().Get(ctx, vNodeName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, apierrors.NewNotFound(corev1.Resource("nodes"), vNodeName)
		}
		log.L.Errorf("Error getting node %s: %v", vNodeName, err)
		return nil, err
	}
	// compatible with old style label
	basePodName := utils.OrElse(
		vNode.Labels[vkModel.LabelKeyOfBaseHostName],
		vNode.Labels[corev1.LabelHostname],
	)
	if basePodName == "" {
		return nil, apierrors.NewNotFound(corev1.Resource("pods"), vPodName)
	}
	return f.clientSet.CoreV1().Pods(namespace).Get(ctx, basePodName, metav1.GetOptions{})
}
