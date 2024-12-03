package controller

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	blacksyriusv1 "github.com/miraccan00/configmapmanager/api/v1"
)

// ConfigMapManagerReconciler reconciles a ConfigMapManager object
type ConfigMapManagerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=blacksyrius.ci.com,resources=configmapmanagers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=blacksyrius.ci.com,resources=configmapmanagers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=blacksyrius.ci.com,resources=configmapmanagers/finalizers,verbs=update

// ConfigMap ve Deployment kaynaklarına erişim izinleri
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;update;patch

func (r *ConfigMapManagerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("configmapmanager", req.NamespacedName)

	// ConfigMapManager nesnesini alın
	var cmManager blacksyriusv1.ConfigMapManager
	if err := r.Get(ctx, req.NamespacedName, &cmManager); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "ConfigMapManager alınamadı")
			return ctrl.Result{}, err
		}
		// Nesne bulunamadıysa, işleme gerek yok
		return ctrl.Result{}, nil
	}

	// Spec içindeki ConfigMaps listesini dolaşın
	for _, cmSpec := range cmManager.Spec.ConfigMaps {
		var configMap corev1.ConfigMap
		cmName := types.NamespacedName{Name: cmSpec.Name, Namespace: req.Namespace}

		// ConfigMap'i alın
		err := r.Get(ctx, cmName, &configMap)
		if err != nil && errors.IsNotFound(err) {
			// ConfigMap mevcut değilse oluşturun
			configMap = corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cmSpec.Name,
					Namespace: req.Namespace,
				},
				Data: make(map[string]string),
			}

			// İstenen değerleri ekleyin
			for _, update := range cmSpec.Updates {
				configMap.Data[update.Key] = update.NewValue
			}

			// ConfigMap'i oluşturun
			if err := r.Create(ctx, &configMap); err != nil {
				logger.Error(err, "ConfigMap oluşturulamadı", "ConfigMap", cmName)
				return ctrl.Result{}, err
			}
			logger.Info("ConfigMap oluşturuldu", "ConfigMap", cmName)

			// İlgili Deployment'ları yeniden başlatın
			if err := r.restartDeployments(ctx, req.Namespace, cmSpec.Name, logger); err != nil {
				logger.Error(err, "Deployment'lar yeniden başlatılamadı")
				return ctrl.Result{}, err
			}
		} else if err != nil {
			logger.Error(err, "ConfigMap alınamadı", "ConfigMap", cmName)
			return ctrl.Result{}, err
		} else {
			// ConfigMap mevcut, karşılaştırma yapın ve güncelleyin
			updated := false
			if configMap.Data == nil {
				configMap.Data = make(map[string]string)
			}
			for _, update := range cmSpec.Updates {
				if currentValue, exists := configMap.Data[update.Key]; !exists || currentValue != update.NewValue {
					configMap.Data[update.Key] = update.NewValue
					updated = true
				}
			}

			// Değişiklik varsa ConfigMap'i güncelleyin
			if updated {
				if err := r.Update(ctx, &configMap); err != nil {
					logger.Error(err, "ConfigMap güncellenemedi", "ConfigMap", cmName)
					return ctrl.Result{}, err
				}
				logger.Info("ConfigMap güncellendi", "ConfigMap", cmName)

				// İlgili Deployment'ları yeniden başlatın
				if err := r.restartDeployments(ctx, req.Namespace, cmSpec.Name, logger); err != nil {
					logger.Error(err, "Deployment'lar yeniden başlatılamadı")
					return ctrl.Result{}, err
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *ConfigMapManagerReconciler) restartDeployments(ctx context.Context, namespace, configMapName string, logger logr.Logger) error {
	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments, client.InNamespace(namespace)); err != nil {
		logger.Error(err, "Deployment'lar listelenemedi")
		return err
	}

	for _, deployment := range deployments.Items {
		usesConfigMap := false

		// Volume'larda ConfigMap kullanımını kontrol edin
		for _, volume := range deployment.Spec.Template.Spec.Volumes {
			if volume.ConfigMap != nil && volume.ConfigMap.Name == configMapName {
				usesConfigMap = true
				break
			}
		}

		// EnvFrom'da ConfigMap kullanımını kontrol edin
		if !usesConfigMap {
			for _, container := range deployment.Spec.Template.Spec.Containers {
				for _, envFrom := range container.EnvFrom {
					if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == configMapName {
						usesConfigMap = true
						break
					}
				}
				if usesConfigMap {
					break
				}
			}
		}

		if usesConfigMap {
			// Anotasyon ekleyerek Deployment'ı yeniden başlatın
			if deployment.Spec.Template.Annotations == nil {
				deployment.Spec.Template.Annotations = make(map[string]string)
			}
			deployment.Spec.Template.Annotations["configmapmanager/restartedAt"] = time.Now().Format(time.RFC3339)

			if err := r.Update(ctx, &deployment); err != nil {
				logger.Error(err, "Deployment güncellenemedi", "Deployment", deployment.Name)
				return err
			}
			logger.Info("Deployment yeniden başlatıldı", "Deployment", deployment.Name)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ConfigMapManagerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log = ctrl.Log.WithName("controllers").WithName("ConfigMapManager")
	return ctrl.NewControllerManagedBy(mgr).
		For(&blacksyriusv1.ConfigMapManager{}).
		Complete(r)
}
