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

	nodeID := "test-base"
	mockBase := NewMockMqttBase("test-base", "1.0.0", nodeID, env)

	nodeID2 := "test-base-2"
	mockBase2 := NewMockMqttBase("test-base-2", "1.0.0", nodeID2, env)

	Context("base online and deactive finally", func() {
		It("base should become a ready vnode eventually", func() {
			go mockBase.Start(ctx)
			vnode := &v1.Node{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(nodeID, env),
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
			mockBase.Exit()
			Eventually(func() bool {
				vnode := &v1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(nodeID, env),
				}, vnode)
				return errors.IsNotFound(err)
			}, time.Second*50, time.Second).Should(BeTrue())
		})
	})

	Context("base online and unreachable finally", func() {
		It("base should become a ready vnode eventually", func() {
			go mockBase2.Start(ctx)
			vnode := &v1.Node{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(nodeID2, env),
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
			mockBase2.Unreachable()
			Eventually(func() bool {
				vnode := &v1.Node{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: utils.FormatNodeName(nodeID2, env),
				}, vnode)
				return errors.IsNotFound(err)
			}, time.Second*50, time.Second).Should(BeTrue())
		})
	})

})
