package kubelet_proxy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	vkModel "github.com/koupleless/virtual-kubelet/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewPodHandler(t *testing.T) {
	clientSet := fake.NewClientset()
	handler, err := NewPodHandler(clientSet)
	require.NoError(t, err)
	require.NotNil(t, handler)
	assert.Equal(t, clientSet, handler.clientSet)

	_, err = NewPodHandler(nil)
	require.Error(t, err)
}

func TestPodHandler_AttachPodRoutes(t *testing.T) {
	vPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vpod",
			Namespace: "vpod-namespace",
		},
		Spec: corev1.PodSpec{
			NodeName: "vnode",
		},
	}
	vNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vnode",
			Labels: map[string]string{
				vkModel.LabelKeyOfBaseHostName: "base-pod",
			},
		},
	}
	basePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "base-pod",
			Namespace: "vpod-namespace",
		},
	}

	handler, err := NewPodHandler(fake.NewClientset(vPod, vNode, basePod))
	require.NoError(t, err)

	mux := http.NewServeMux()
	handler.AttachPodRoutes(mux)

	t.Run("normal case", func(t *testing.T) {
		t.Run("with default params", func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/containerLogs/vpod-namespace/vpod/ignored", nil)

			mux.ServeHTTP(recorder, req)
			assert.EqualValues(t, http.StatusOK, recorder.Code)
			assert.EqualValues(t, "fake logs", recorder.Body.String())
		})
		t.Run("with custom params", func(t *testing.T) {
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/containerLogs/vpod-namespace/vpod/ignored", nil)
			queryParams := make(url.Values)
			queryParams.Add("tailLines", "1")
			queryParams.Add("follow", "true")
			queryParams.Add("timestamps", "true")
			queryParams.Add("previous", "false")
			queryParams.Add("limitBytes", "1000")
			queryParams.Add("sinceTime", "2023-10-01T00:00:00Z")
			req.URL.RawQuery = queryParams.Encode()

			mux.ServeHTTP(recorder, req)
			assert.EqualValues(t, http.StatusOK, recorder.Code)
			assert.EqualValues(t, "fake logs", recorder.Body.String())

			queryParams.Del("sinceTime")
			queryParams.Add("sinceSeconds", "60")
			req.URL.RawQuery = queryParams.Encode()

			recorder = httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)
			assert.EqualValues(t, http.StatusOK, recorder.Code)
			assert.EqualValues(t, "fake logs", recorder.Body.String())
		})
	})

	t.Run("n: vpod not found", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(
			recorder,
			httptest.NewRequest("GET", "/containerLogs/vpod-namespace/nonexistent/ignored", nil),
		)
		assert.EqualValues(t, http.StatusNotFound, recorder.Code)
	})

	t.Run("n: vnode not found", func(t *testing.T) {
		vPodCopy := vPod.DeepCopy()
		vPodCopy.Spec.NodeName = "nonexistent-vnode"
		clientSet := fake.NewClientset(vPodCopy, vNode)

		handler, err := NewPodHandler(clientSet)
		require.NoError(t, err)
		mux := http.NewServeMux()
		handler.AttachPodRoutes(mux)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest("GET", "/containerLogs/vpod-namespace/vpod/ignored", nil))

		assert.EqualValues(t, http.StatusNotFound, recorder.Code)

	})

	t.Run("n: base pod not found", func(t *testing.T) {
		clientSet := fake.NewClientset(vPod, vNode)

		handler, err := NewPodHandler(clientSet)
		require.NoError(t, err)
		mux := http.NewServeMux()
		handler.AttachPodRoutes(mux)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest("GET", "/containerLogs/vpod-namespace/vpod/ignored", nil))

		assert.EqualValues(t, http.StatusNotFound, recorder.Code)
	})
}
