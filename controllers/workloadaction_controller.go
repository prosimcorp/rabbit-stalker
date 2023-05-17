/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	rabbitstalkerv1alpha1 "docplanner.com/rabbit-stalker/api/v1alpha1"
)

const (
	defaultSyncTimeForExitWithError = 10 * time.Second
	workloadActionFinalizer         = "rabbit-stalker.docplanner.com/finalizer"

	scheduleSynchronization = "Schedule synchronization in: %s"

	workloadActionNotFoundError          = "WorkloadAction resource not found. Ignoring since object must be deleted."
	workloadActionRetrievalError         = "Error getting the WorkloadAction from the cluster"
	workloadActionFinalizersUpdateError  = "Failed to update finalizer of WorkloadAction: %s"
	workloadActionConditionUpdateError   = "Failed to update the condition on WorkloadAction: %s"
	workloadActionSyncTimeRetrievalError = "Can not get synchronization time from the WorkloadAction: %s"
	workloadActionReconcileError         = "Can not reconcile WorkloadAction: %s"
)

// WorkloadActionReconciler reconciles a WorkloadAction object
type WorkloadActionReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// TODO: Include zapp logger here
}

//+kubebuilder:rbac:groups=rabbit-stalker.docplanner.com,resources=workloadactions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rabbit-stalker.docplanner.com,resources=workloadactions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rabbit-stalker.docplanner.com,resources=workloadactions/finalizers,verbs=update
//+kubebuilder:rbac:groups="*",resources=secrets;deployments;daemonsets;statefulsets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WorkloadActionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	var err error
	var result ctrl.Result

	//1. Get the content of the WorkloadAction
	workloadActionManifest := &rabbitstalkerv1alpha1.WorkloadAction{}

	err = r.Get(ctx, req.NamespacedName, workloadActionManifest)

	// 2. Check existence on the cluster
	if err != nil {

		// 2.1 It does NOT exist: manage removal
		if err = client.IgnoreNotFound(err); err == nil {
			LogInfof(ctx, workloadActionNotFoundError)
			return result, err
		}

		// 2.2 Failed to get the resource, requeue the request
		LogInfof(ctx, workloadActionRetrievalError)
		return result, err
	}

	// 3. Check if the WorkloadAction instance is marked to be deleted: indicated by the deletion timestamp being set
	if !workloadActionManifest.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(workloadActionManifest, workloadActionFinalizer) {
			// Remove the finalizers on WorkloadAction CR
			controllerutil.RemoveFinalizer(workloadActionManifest, workloadActionFinalizer)
			err = r.Update(ctx, workloadActionManifest)
			if err != nil {
				LogInfof(ctx, workloadActionFinalizersUpdateError, req.Name)
			}
		}
		result = ctrl.Result{}
		err = nil
		return result, err
	}

	// 4. Add finalizer to the WorkloadAction CR
	if !controllerutil.ContainsFinalizer(workloadActionManifest, workloadActionFinalizer) {
		controllerutil.AddFinalizer(workloadActionManifest, workloadActionFinalizer)
		err = r.Update(ctx, workloadActionManifest)
		if err != nil {
			return result, err
		}
	}

	// 5. Update the status before the requeue
	defer func() {
		err = r.Status().Update(ctx, workloadActionManifest)
		if err != nil {
			LogInfof(ctx, workloadActionConditionUpdateError, err)
		}
		err = nil
	}()

	// 6. Schedule periodical request
	RequeueTime, err := r.GetSynchronizationTime(workloadActionManifest)
	if err != nil {
		LogInfof(ctx, workloadActionSyncTimeRetrievalError, workloadActionManifest.Name)
		return result, err
	}
	result = ctrl.Result{
		//Requeue:      true,
		RequeueAfter: RequeueTime,
	}

	// 7. The WorkloadAction CR already exist: manage the update
	err = r.reconcileWorkloadAction(ctx, workloadActionManifest)
	if err != nil {
		LogInfof(ctx, workloadActionReconcileError, err)

		err = nil
		return result, err
	}

	// 8. Success, update the status
	r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
		metav1.ConditionTrue,
		ConditionReasonReady,
		ConditionReasonReadyMessage,
	))

	LogInfof(ctx, scheduleSynchronization, result.RequeueAfter.String())
	return result, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkloadActionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&rabbitstalkerv1alpha1.WorkloadAction{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}
