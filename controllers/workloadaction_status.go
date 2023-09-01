package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rabbitstalkerv1alpha1 "prosimcorp.com/rabbit-stalker/api/v1alpha1"
)

const (

	//
	// ConditionTypeWorkloadActionReady indicates that the WorkloadAction is ready to act or not
	ConditionTypeWorkloadActionReady = "WorkloadActionReady"

	// Workload not found
	ConditionReasonWorkloadNotFound        = "WorkloadNotFound"
	ConditionReasonWorkloadNotFoundMessage = "Workload resource was not found"

	// Credentials not found
	ConditionReasonCredentialsNotFound        = "CredentialsNotFound"
	ConditionReasonCredentialsNotFoundMessage = "Credentials secret or key not found"

	// Required field not found
	ConditionReasonRequiredFieldNotFound        = "RequiredFieldNotFound"
	ConditionReasonRequiredFieldNotFoundMessage = "Required field not found. Is vhost set on literal queue name?"

	// HTTPRequest failed
	ConditionReasonUrlParsingFailed        = "UrlParsingFailed"
	ConditionReasonUrlParsingFailedMessage = "Url parsing failed. Fix its syntax and try again"

	// HTTPResponse not successful
	ConditionReasonHttpResponseNotSuccessful        = "HttpRequestNotSuccessful"
	ConditionReasonHttpResponseNotSuccessfulMessage = "Http request returned status code: %d"

	// Queue not found
	ConditionReasonQueueNotFound        = "QueueNotFound"
	ConditionReasonQueueNotFoundMessage = "No queues were found with defined name"

	// Condition value parsing failed
	ConditionReasonConditionValueParsingFailed        = "ConditionValueParsingFailed"
	ConditionReasonConditionValueParsingFailedMessage = "Condition value parsing process failed"

	// HTTPResponse not valid
	ConditionReasonHttpResponseNotValid        = "HttpResponseNotValid"
	ConditionReasonHttpResponseNotValidMessage = "Response can not be parsed"

	// Action not valid
	ConditionReasonInvalidAction        = "InvalidAction"
	ConditionReasonInvalidActionMessage = "Action is invalid"

	// Action execution failed
	ConditionReasonActionExecutionFailed        = "ActionExecutionFailed"
	ConditionReasonActionExecutionFailedMessage = "Action failed during execution"

	// Success
	ConditionReasonReady        = "Ready"
	ConditionReasonReadyMessage = "WorkloadAction is stalking the rabbit"

	// TODO: Work on a specific condition for GJSON
	// Ref: https://gjson.dev/
)

// NewWorkloadActionCondition a set of default options for creating a Condition.
func (r *WorkloadActionReconciler) NewWorkloadActionCondition(condType string, status metav1.ConditionStatus, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetWorkloadActionCondition returns the condition with the provided type.
func (r *WorkloadActionReconciler) GetWorkloadActionCondition(workloadAction *rabbitstalkerv1alpha1.WorkloadAction, condType string) *metav1.Condition {

	for i, v := range workloadAction.Status.Conditions {
		if v.Type == condType {
			return &workloadAction.Status.Conditions[i]
		}
	}
	return nil
}

// UpdateWorkloadActionCondition update or create a new condition inside the status of the CR
func (r *WorkloadActionReconciler) UpdateWorkloadActionCondition(workloadAction *rabbitstalkerv1alpha1.WorkloadAction, condition *metav1.Condition) {

	// Get the condition
	currentCondition := r.GetWorkloadActionCondition(workloadAction, condition.Type)

	if currentCondition == nil {
		// Create the condition when not existent
		workloadAction.Status.Conditions = append(workloadAction.Status.Conditions, *condition)
	} else {
		// Update the condition when existent.
		currentCondition.Status = condition.Status
		currentCondition.Reason = condition.Reason
		currentCondition.Message = condition.Message
		currentCondition.LastTransitionTime = metav1.Now()
	}
}
