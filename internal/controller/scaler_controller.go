package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	apiv1alpha1 "github.com/rezacloner1372/schedule-pod-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

// ScalerReconciler reconciles a Scaler object
type ScalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *ScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling Scaler")
	scaler := &apiv1alpha1.Scaler{}

	// Get the instancee of the Scaler
	err := r.Get(ctx, req.NamespacedName, scaler)
	if err != nil {
		log.Error(err, "Failed to get Scaler")
		return ctrl.Result{}, err
	}
	startTime := scaler.Spec.Start
	endTime := scaler.Spec.End
	replicas := scaler.Spec.Replicas

	currentHour := time.Now().UTC().Hour()

	log.Info(fmt.Sprintf("Current hour: %d", currentHour))
	if currentHour >= startTime && currentHour <= endTime {
		if err := scaleDeployment(scaler, r, ctx, replicas, log); err != nil {
			log.Error(err, "Failed to scale Deployment")
			return ctrl.Result{}, err
		}
	} else {
		scaler.Status.Status = apiv1alpha1.SUCCESS // If no scaling needed, still update to SUCCESS
		err = r.Status().Update(ctx, scaler)
		if err != nil {
			log.Error(err, "Failed to update Scaler status")
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: time.Duration(30 * time.Second)}, nil
}

func scaleDeployment(scaler *apiv1alpha1.Scaler, r *ScalerReconciler, ctx context.Context, replicas int32, log logr.Logger) error {
	for _, deploy := range scaler.Spec.Deployments {
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{
			Namespace: deploy.Namespace,
			Name:      deploy.Name,
		},
			deployment,
		)
		if err != nil {
			log.Info("scaler.Status.Status: ", scaler.Status.Status)
			log.Error(err, "Failed to get Deployment")
			scaler.Status.Status = apiv1alpha1.FAILURE
			_ = r.Status().Update(ctx, scaler)
			return err
		}

		if deployment.Spec.Replicas != &replicas {
			deployment.Spec.Replicas = &replicas
			err := r.Update(ctx, deployment)
			if err != nil {
				scaler.Status.Status = apiv1alpha1.FAILURE
				log.Info("scaler.Status.Status: ", scaler.Status.Status)
				log.Error(err, "Failed to update Deployment")
				_ = r.Status().Update(ctx, scaler)
				return err
			}
			scaler.Status.Status = apiv1alpha1.SUCCESS
		} else {
			scaler.Status.Status = apiv1alpha1.SUCCESS
		}
		err = r.Status().Update(ctx, scaler)
		if err != nil {
			log.Error(err, "Failed to update Scaler status")
			return err
		}
	}
	return nil
}

func (r *ScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.Scaler{}).
		Complete(r)
}
