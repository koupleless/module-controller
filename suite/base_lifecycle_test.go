package suite

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
	mockMqttBase := NewMockMqttBase("test-mqtt-base", "1.0.0", mqttNodeID, env)

	httpNodeID := "test-http-base"
	mockHttpBase := NewMockHttpBase("test-http-base", "1.0.0", httpNodeID, env, 1238)

	Context("mqtt base online and deactive finally", func() {
		It("base should become a ready vnode eventually", func() {
			go mockMqttBase.Start(ctx)
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
			go mockMqttBase.Start(ctx)
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

	Context("http base online and deactive finally", func() {
		It("base should become a ready vnode eventually", func() {
			go mockHttpBase.Start(ctx)
			vnode := &v1.Node{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(httpNodeID, env),
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
			mockHttpBase.Exit()
			Eventually(func() bool {
				vnode := &v1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(httpNodeID, env),
				}, vnode)
				return errors.IsNotFound(err)
			}, time.Second*50, time.Second).Should(BeTrue())
		})
	})

	Context("http base online and unreachable finally", func() {
		It("base should become a ready vnode eventually", func() {
			go mockHttpBase.Start(ctx)
			vnode := &v1.Node{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(httpNodeID, env),
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
			mockHttpBase.Unreachable()
			Eventually(func() bool {
				vnode := &v1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(httpNodeID, env),
				}, vnode)
				return errors.IsNotFound(err)
			}, time.Second*50, time.Second).Should(BeTrue())

			mockHttpBase.Exit()
		})
	})

	Context("http base online and change base id finally", func() {
		It("base should become a ready vnode eventually", func() {
			go mockHttpBase.Start(ctx)
			vnode := &v1.Node{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(httpNodeID, env),
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

		It("base id changed finally exit", func() {
			mockHttpBase.ID = "changed-base-id"
			Eventually(func() bool {
				vnode := &v1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(httpNodeID, env),
				}, vnode)
				return errors.IsNotFound(err)
			}, time.Second*50, time.Second).Should(BeTrue())

			mockHttpBase.Exit()
		})
	})

})
