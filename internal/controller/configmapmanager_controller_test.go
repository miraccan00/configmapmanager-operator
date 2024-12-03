package controller

import (
	"context"
	"testing"

	blacksyriusv1 "github.com/miraccan00/configmapmanager/api/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcileCreateConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = blacksyriusv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &ConfigMapManagerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}

	cmManager := &blacksyriusv1.ConfigMapManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmapmanager",
			Namespace: "default",
		},
		Spec: blacksyriusv1.ConfigMapManagerSpec{
			ConfigMaps: []blacksyriusv1.ConfigMapSpec{
				{
					Name: "test-configmap",
					Updates: []blacksyriusv1.ConfigMapUpdate{
						{Key: "example-key", NewValue: "example-value"},
					},
				},
			},
		},
	}

	err := fakeClient.Create(context.Background(), cmManager)
	assert.NoError(t, err)

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-configmapmanager",
			Namespace: "default",
		},
	}

	_, err = reconciler.Reconcile(context.Background(), request)
	assert.NoError(t, err)

	configMap := &corev1.ConfigMap{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-configmap", Namespace: "default"}, configMap)
	assert.NoError(t, err)
	assert.Equal(t, "example-value", configMap.Data["example-key"])
}

func TestReconcileUpdateConfigMap(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = blacksyriusv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	initialConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "default",
		},
		Data: map[string]string{
			"example-key": "old-value",
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initialConfigMap).Build()
	reconciler := &ConfigMapManagerReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("test"),
	}

	cmManager := &blacksyriusv1.ConfigMapManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmapmanager",
			Namespace: "default",
		},
		Spec: blacksyriusv1.ConfigMapManagerSpec{
			ConfigMaps: []blacksyriusv1.ConfigMapSpec{
				{
					Name: "test-configmap",
					Updates: []blacksyriusv1.ConfigMapUpdate{
						{Key: "example-key", NewValue: "new-value"},
					},
				},
			},
		},
	}

	err := fakeClient.Create(context.Background(), cmManager)
	assert.NoError(t, err)

	request := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "test-configmapmanager",
			Namespace: "default",
		},
	}

	_, err = reconciler.Reconcile(context.Background(), request)
	assert.NoError(t, err)

	configMap := &corev1.ConfigMap{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-configmap", Namespace: "default"}, configMap)
	assert.NoError(t, err)
	assert.Equal(t, "new-value", configMap.Data["example-key"])
}

func TestRestartDeployments(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx",
							EnvFrom: []corev1.EnvFromSource{
								{
									ConfigMapRef: &corev1.ConfigMapEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "test-configmap",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deployment).Build()
	reconciler := &ConfigMapManagerReconciler{
		Client: fakeClient,
		Log:    ctrl.Log.WithName("test"),
	}

	err := reconciler.restartDeployments(context.Background(), "default", "test-configmap", ctrl.Log.WithName("test"))
	assert.NoError(t, err)

	updatedDeployment := &appsv1.Deployment{}
	err = fakeClient.Get(context.Background(), types.NamespacedName{Name: "test-deployment", Namespace: "default"}, updatedDeployment)
	assert.NoError(t, err)
	_, exists := updatedDeployment.Spec.Template.Annotations["configmapmanager/restartedAt"]
	assert.True(t, exists)
}
