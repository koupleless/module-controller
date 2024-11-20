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

	clusterName := "test-cluster-name"

	mockBase := NewMockMqttBase("test-base", clusterName, "1.0.0", env)
	mockBase2 := NewMockMqttBase("test-base-2", clusterName, "1.0.0", env)

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
							Image: "test-biz.jar",
							Env: []v1.EnvVar{
								{
									Name:  "BIZ_VERSION",
									Value: "1.0.0",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						model.LabelKeyOfTechStack:          "java",
						vkModel.LabelKeyOfVNodeVersion:     "1.0.0",
						vkModel.LabelKeyOfVNodeClusterName: clusterName,
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
							Image: "test-biz-2.jar",
							Env: []v1.EnvVar{
								{
									Name:  "BIZ_VERSION",
									Value: "1.0.0",
								},
							},
						},
					},
					NodeSelector: map[string]string{
						model.LabelKeyOfTechStack:          "java",
						vkModel.LabelKeyOfVNodeVersion:     "1.0.0",
						vkModel.LabelKeyOfVNodeClusterName: clusterName + "-2",
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
				depFromKubernetes := &appsv1.Deployment{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, depFromKubernetes)
				return err == nil && *depFromKubernetes.Spec.Replicas == 0
			}, time.Second*20, time.Second).Should(BeTrue())
		})

		It("one node online and deployment replicas should be 1", func() {
			go mockBase.Start(ctx, clientID)
			Eventually(func() bool {
				depFromKubernetes := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, depFromKubernetes)
				return err == nil && *depFromKubernetes.Spec.Replicas == 1
			}, time.Minute*20, time.Second).Should(BeTrue())
		})

		It("another node online and deployment replicas should be 2", func() {
			go mockBase2.Start(ctx, clientID)
			Eventually(func() bool {
				depFromKubernetes := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, depFromKubernetes)
				return err == nil && *depFromKubernetes.Spec.Replicas == 2
			}, time.Second*20, time.Second).Should(BeTrue())
		})

		It("publish a new deployment and replicas should be 0", func() {
			err := k8sClient.Create(ctx, &deployment2)
			Expect(err).ToNot(HaveOccurred())
			Eventually(func() bool {
				depFromKubernetes := &appsv1.Deployment{}
				err = k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment2.Name,
					Namespace: deployment2.Namespace,
				}, depFromKubernetes)
				return err == nil && *depFromKubernetes.Spec.Replicas == 0
			}, time.Second*20, time.Second).Should(BeTrue())
		})

		It("mock base 2 query baseline should not fetch deployment2 containers", func() {
			mockBase2.QueryBaseline()
			Eventually(func() bool {
				return len(mockBase2.Baseline) == 1
			}, time.Second*20, time.Second).Should(BeTrue())
		})

		It("replicas should be 2", func() {
			mockBase2.Exit()
			Eventually(func() bool {
				depFromKubernetes := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, depFromKubernetes)
				return err == nil && *depFromKubernetes.Spec.Replicas == 2
			}, time.Second*20, time.Second).Should(BeTrue())
		})

		It("mock base 2 exit and replicas should be 1 finally", func() {
			mockBase2.Exit()
			Eventually(func() bool {
				depFromKubernetes := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, depFromKubernetes)
				return err == nil && *depFromKubernetes.Spec.Replicas == 1
			}, time.Second*20, time.Second).Should(BeTrue())
		})

		It("mock base exit and replicas should be 0 finally", func() {
			mockBase.Exit()
			Eventually(func() bool {
				depFromKubernetes := &appsv1.Deployment{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      deployment1.Name,
					Namespace: deployment1.Namespace,
				}, depFromKubernetes)
				return err == nil && *depFromKubernetes.Spec.Replicas == 0
			}, time.Second*30, time.Second).Should(BeTrue())
		})
	})
})
