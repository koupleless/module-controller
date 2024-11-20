package mqtt

import (
	"context"
	"github.com/koupleless/virtual-kubelet/common/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"time"
)

var _ = Describe("Base Lifecycle Test", func() {

	ctx := context.Background()

	mqttNodeID := "test-mqtt-base"
	clusterName := "test-cluster-name"
	mockMqttBase := NewMockMqttBase(mqttNodeID, clusterName, "1.0.0", env)

	Context("mqtt base online and deactive finally", func() {
		It("base should become a ready vnode eventually", func() {
			time.Sleep(time.Second)

			go mockMqttBase.Start(ctx, clientID)
			vnode := &v1.Node{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(mqttNodeID, env),
				}, vnode)
				vnodeReady := false
				for _, cond := range vnode.Status.Conditions {
					if cond.Type == v1.NodeReady {
						vnodeReady = cond.Status == v1.ConditionTrue
						break
					}
				}
				return err == nil && vnodeReady
			}, time.Second*50, time.Second).Should(BeTrue())
		})

		It("base offline with deactive message and finally exit", func() {
			mockMqttBase.Exit()
			Eventually(func() bool {
				vnode := &v1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(mqttNodeID, env),
				}, vnode)
				return errors.IsNotFound(err)
			}, time.Second*50, time.Second).Should(BeTrue())
		})
	})

	Context("mqtt base online and unreachable finally", func() {
		It("base should become a ready vnode eventually", func() {
			time.Sleep(time.Second)

			go mockMqttBase.Start(ctx, clientID)
			vnode := &v1.Node{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(mqttNodeID, env),
				}, vnode)
				vnodeReady := false
				for _, cond := range vnode.Status.Conditions {
					if cond.Type == v1.NodeReady {
						vnodeReady = cond.Status == v1.ConditionTrue
						break
					}
				}
				return err == nil && vnodeReady
			}, time.Second*50, time.Second).Should(BeTrue())
		})

		It("base unreachable finally exit", func() {
			mockMqttBase.Unreachable()
			Eventually(func() bool {
				vnode := &v1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(mqttNodeID, env),
				}, vnode)
				return errors.IsNotFound(err)
			}, time.Second*50, time.Second).Should(BeTrue())

			mockMqttBase.Exit()
		})
	})
})
