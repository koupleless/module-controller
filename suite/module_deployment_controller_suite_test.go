package suite

import (
	"context"
	"github.com/koupleless/module_controller/common/model"
	vkModel "github.com/koupleless/virtual-kubelet/model"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"time"
)

var _ = Describe("Module Deployment Controller Test", func() {

	ctx := context.Background()

	mockBase := NewMockMqttBase("test-base", "1.0.0", "test-base", env)
	mockBase2 := NewMockMqttBase("test-base", "1.0.0", "test-base-2", env)

	deployment1 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-base-deployment",
			Namespace: "default",
			Labels: map[string]string{
				vkModel.LabelKeyOfComponent:            model.ComponentModuleDeployment,
				vkModel.LabelKeyOfEnv:                  env,
				model.LabelKeyOfVPodDeploymentStrategy: string(model.VPodDeploymentStrategyPeer),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					vkModel.LabelKeyOfComponent: model.ComponentModule,
					"app":                       "test-biz-deployment",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						vkModel.LabelKeyOfComponent: model.ComponentModule,
						"app":                       "test-biz-deployment",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "test-biz",
							Image: "test-biz",
							Env: []v1.EnvVar{
								{
									Name:  "BIZ_VERSION",
									Value: "1.0.0",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						model.LabelKeyOfTechStack:      "java",
						vkModel.LabelKeyOfVNodeName:    "test-base",
						vkModel.LabelKeyOfVNodeVersion: "1.0.0",
					},
					Tolerations: []v1.Toleration{
						{
							Key:      vkModel.TaintKeyOfVnode,
							Operator: v1.TolerationOpEqual,
							Value:    "True",
							Effect:   v1.TaintEffectNoExecute,
						},
						{
							Key:      vkModel.TaintKeyOfEnv,
							Operator: v1.TolerationOpEqual,
							Value:    env,
							Effect:   v1.TaintEffectNoExecute,
						},
					},
				},
			},
		},
	}

	deployment2 := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-base-deployment-2",
			Namespace: "default",
			Labels: map[string]string{
				vkModel.LabelKeyOfComponent:            model.ComponentModuleDeployment,
				vkModel.LabelKeyOfEnv:                  env,
				model.LabelKeyOfVPodDeploymentStrategy: string(model.VPodDeploymentStrategyPeer),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To[int32](1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					vkModel.LabelKeyOfComponent: model.ComponentModule,
					"app":                       "test-biz-deployment",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						vkModel.LabelKeyOfComponent: model.ComponentModule,
						"app":                       "test-biz-deployment",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "test-biz-2",
							Image: "test-biz-2",
							Env: []v1.EnvVar{
								{
									Name:  "BIZ_VERSION",
									Value: "1.0.0",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						model.LabelKeyOfTechStack:      "java",
						vkModel.LabelKeyOfVNodeName:    "test-base-2",
						vkModel.LabelKeyOfVNodeVersion: "1.0.0",
					},
					Tolerations: []v1.Toleration{
						{
							Key:      vkModel.TaintKeyOfVnode,
							Operator: v1.TolerationOpEqual,
							Value:    "True",
							Effect:   v1.TaintEffectNoExecute,
						},
						{
							Key:      vkModel.TaintKeyOfEnv,
							Operator: v1.TolerationOpEqual,
							Value:    env,
							Effect:   v1.TaintEffectNoExecute,
						},
					},
				},
			},
		},
	}

	Context("deployment publish and query baseline", func() {
		It("publish deployment and replicas should be zero", func() {
			err := k8sClient.Create(ctx, &deployment1)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, &deployment1)
				return err == nil && *deployment1.Spec.Replicas == 0
			}, time.Second*10, time.Second).Should(BeTrue())
		})

		It("one node online and deployment replicas should be 1", func() {
			go mockBase.Start(ctx)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, &deployment1)
				return err == nil && *deployment1.Spec.Replicas == 1
			}, time.Second*10, time.Second).Should(BeTrue())
		})

		It("another node online and deployment replicas should be 2", func() {
			go mockBase2.Start(ctx)
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, &deployment1)
				return err == nil && *deployment1.Spec.Replicas == 2
			}, time.Second*10, time.Second).Should(BeTrue())
		})

		It("publish a new deployment and replicas should be 0", func() {
			err := k8sClient.Create(ctx, &deployment2)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment2.Name,
					Namespace: deployment2.Namespace,
				}, &deployment2)
				return err == nil && *deployment2.Spec.Replicas == 0
			}, time.Second*10, time.Second).Should(BeTrue())
		})

		It("mock base 2 query baseline should not fetch deployment2 containers", func() {
			mockBase2.QueryBaseline()
			Eventually(func() bool {
				return len(mockBase2.Baseline) == 1
			}, time.Second*10, time.Second).Should(BeTrue())
		})

		It("mock base 2 exit and replicas should be 1 finally", func() {
			mockBase2.Exit()
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, &deployment1)
				return err == nil && *deployment1.Spec.Replicas == 1
			}, time.Second*10, time.Second).Should(BeTrue())
		})

		It("mock base exit and replicas should be 0 finally", func() {
			mockBase.Exit()
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, &deployment1)
				return err == nil && *deployment1.Spec.Replicas == 0
			}, time.Second*10, time.Second).Should(BeTrue())
		})
	})
})
