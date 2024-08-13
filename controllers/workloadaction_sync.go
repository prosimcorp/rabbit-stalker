package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	rabbitstalkerv1alpha1 "prosimcorp.com/rabbit-stalker/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tidwall/gjson"
)

const (
	// RabbitAdminApiQueuesEndpoint TODO
	RabbitAdminApiQueuesEndpoint = "/api/queues"

	// Parameters related to RabbitMQ Admin API paginated endpoints
	StartingPageDefaultValue       = 1
	PageSizeDefaultValue           = 5
	UseRegexDefaultValue           = true
	HttpSchemeDefaultValue         = "https"
	HttpHeaderAcceptDefaultValue   = "application/json"
	HttpRequestTimeoutDefaultValue = 5 * time.Second

	// TODO
	AnnotationRestartedAt = "rabbit-stalker.prosimcorp.com/restartedAt"

	// parseSyncTimeError error message for invalid value on 'synchronization' parameter
	parseSyncTimeError = "Can not parse the synchronization time from workloadAction: %s"

	WorkloadRestartNotSupportedErrorMessage   = "restarting is not supported"
	PausedWorkloadRestartErrorMessage         = "can't restart paused deployment (run rollout resume first)"
	WorkloadActionAnnotationPatchErrorMessage = "impossible to patch the annotations for the workload template: %s"
	UnsuccessfulRequestErrorMessage           = "unsuccessful request: %d"
	InvalidJsonErrorMessage                   = "invalid json from the HTTP response"
	InvalidActionErrorMessage                 = "invalid action. supported: restart, delete"
	DeleteActionNotImplementedErrorMessage    = "deletion action is not implemented yet"
	VhostNotDefinedErrorMessage               = "queues with literal names require vhost field defined"
	QueuesNotFoundErrorMessage                = "no queues were found with defined name"

	// TODO
	ResourceKindDeployment  = "Deployment"
	ResourceKindStatefulSet = "StatefulSet"
	ResourceKindDaemonSet   = "DaemonSet"
	ResourceKindArgoRollout = "Rollout"

	// TODO
	ActionDelete  = "delete"
	ActionRestart = "restart"
)

// PatchConstructorPatchT represents a 'patch' and (they way it is applied)
// returned by a PatchConstructorFuncPointerT
// TODO Move wherever it's better than here
type PatchConstructorPatchT struct {
	// PatchType represents the type of patch to apply
	PatchType types.PatchType

	// Patch represents the patch desired to be applied
	Patch []byte
}

// PatchConstructorFuncPointerT represents a pointer to a function for crafting a patch
// TODO Move wherever it's better than here
type PatchConstructorFuncPointerT func(obj *unstructured.Unstructured) (PatchConstructorPatchT, error)

// HttpRequestAuth represents authentication params provided to a request
// TODO Move wherever it's better than here
type HttpRequestAuth struct {
	Username string
	Password string
	Token    string
}

// RabbitPaginatedResponse represents a response from an endpoint with pagination params
// TODO Move wherever it's better than here
type RabbitPaginatedResponse struct {
	FilteredCount int           `json:"filtered_count"`
	ItemCount     int           `json:"item_count"`
	Items         []interface{} `json:"items"`
	Page          int           `json:"page"`
	PageCount     int           `json:"page_count"`
	PageSize      int           `json:"page_size"`
	TotalCount    int           `json:"total_count"`
}

// GetSynchronizationTime return the spec.synchronization.time as duration, or default time on failures
func (r *WorkloadActionReconciler) GetSynchronizationTime(workloadActionManifest *rabbitstalkerv1alpha1.WorkloadAction) (synchronizationTime time.Duration, err error) {
	synchronizationTime, err = time.ParseDuration(workloadActionManifest.Spec.Synchronization.Time)
	if err != nil {
		err = NewErrorf(parseSyncTimeError, workloadActionManifest.Name)
		return synchronizationTime, err
	}

	return synchronizationTime, err
}

// GetSecretResource call Kubernetes API to return an arbitrary Secret object
func (r *WorkloadActionReconciler) GetSecretResource(ctx context.Context, namespace string, name string) (secret *corev1.Secret, err error) {

	secretResource := corev1.Secret{}

	err = r.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &secretResource)

	secret = &secretResource
	return secret, err
}

// GetCredentialsResources call Kubernetes API to return the Secret objects for the HTTP request authentication
func (r *WorkloadActionReconciler) GetCredentialsResources(ctx context.Context, workloadActionManifest *rabbitstalkerv1alpha1.WorkloadAction) (resources []*corev1.Secret, err error) {

	usernameSecret := &corev1.Secret{}
	passwordSecret := &corev1.Secret{}

	// Get the Secret with the username field inside
	secretNamespace := workloadActionManifest.Namespace
	if !reflect.ValueOf(workloadActionManifest.Spec.RabbitConnection.Credentials.Username.SecretRef.Namespace).IsZero() {
		secretNamespace = workloadActionManifest.Spec.RabbitConnection.Credentials.Username.SecretRef.Namespace
	}

	secretName := workloadActionManifest.Spec.RabbitConnection.Credentials.Username.SecretRef.Name
	usernameSecret, usernameErr := r.GetSecretResource(ctx, secretNamespace, secretName)

	// Get the Secret with the password field inside
	secretNamespace = workloadActionManifest.Namespace
	if !reflect.ValueOf(workloadActionManifest.Spec.RabbitConnection.Credentials.Password.SecretRef.Namespace).IsZero() {
		secretNamespace = workloadActionManifest.Spec.RabbitConnection.Credentials.Password.SecretRef.Namespace
	}

	secretName = workloadActionManifest.Spec.RabbitConnection.Credentials.Password.SecretRef.Name
	passwordSecret, passwordErr := r.GetSecretResource(ctx, secretNamespace, secretName)
	if usernameErr != nil || passwordErr != nil {
		return resources, err
	}
	resources = append(resources, usernameSecret)
	resources = append(resources, passwordSecret)

	return resources, err
}

// GetWorkloadResource call Kubernetes API to return the WorkloadResource object
func (r *WorkloadActionReconciler) GetWorkloadResource(ctx context.Context, workloadActionManifest *rabbitstalkerv1alpha1.WorkloadAction) (resource *unstructured.Unstructured, err error) {

	// Get the target manifest
	target := unstructured.Unstructured{}
	target.SetGroupVersionKind(workloadActionManifest.Spec.WorkloadRef.GroupVersionKind())

	err = r.Get(ctx, client.ObjectKey{
		Namespace: workloadActionManifest.Spec.WorkloadRef.Namespace,
		Name:      workloadActionManifest.Spec.WorkloadRef.Name,
	}, &target)

	resource = &target
	return resource, err
}

// addWorkloadResource add WorkloadRef referenced resource to the sources list
func (r *WorkloadActionReconciler) addWorkloadResource(ctx context.Context, workloadActionManifest *rabbitstalkerv1alpha1.WorkloadAction, resources *[]string) (err error) {

	// Get the target manifest
	target, err := r.GetWorkloadResource(ctx, workloadActionManifest)
	if err != nil {
		return err
	}

	targetObjectJson, err := json.Marshal(target.Object)
	if err != nil {
		return err
	}

	*resources = append(*resources, string(targetObjectJson))
	return err
}

// addAdditionalSources add additionalSources objects into the sources list
func (r *WorkloadActionReconciler) addAdditionalSources(ctx context.Context, workloadActionManifest *rabbitstalkerv1alpha1.WorkloadAction, resources *[]string) (err error) {

	// Fill the sources content, one by one
	sourceObject := &unstructured.Unstructured{}

	for _, sourceReference := range workloadActionManifest.Spec.AdditionalSources {
		sourceObject.SetGroupVersionKind(sourceReference.GroupVersionKind())

		err = r.Get(ctx, client.ObjectKey{
			Namespace: sourceReference.Namespace,
			Name:      sourceReference.Name,
		}, sourceObject)

		if err != nil {
			return err
		}

		sourceObjectJson, err := json.Marshal(sourceObject.Object)
		if err != nil {
			return err
		}

		*resources = append(*resources, string(sourceObjectJson))
	}

	return err
}

// GetSourcesList return a JSON compatible list of objects with the target as first item, and the additionalSources after it
func (r *WorkloadActionReconciler) GetSourcesList(ctx context.Context, workloadManifest *rabbitstalkerv1alpha1.WorkloadAction) (resources []string, err error) {

	// Fill the resources list with the target
	err = r.addWorkloadResource(ctx, workloadManifest, &resources)
	if err != nil {
		return resources, err
	}

	// Fill the resources list with the sources
	err = r.addAdditionalSources(ctx, workloadManifest, &resources)
	if err != nil {
		return resources, err
	}

	return resources, err
}

// GetParsedConditionValue return condition's value field with all substitutions already done
func (r *WorkloadActionReconciler) GetParsedConditionValue(ctx context.Context, workloadManifest *rabbitstalkerv1alpha1.WorkloadAction) (conditionValue string, err error) {
	referenceOpeningPattern := `\[\d+\]`
	openingPattern := `{{`
	closingPattern := `}}`

	var sourcesJson []string
	sourcesJson, err = r.GetSourcesList(ctx, workloadManifest)
	if err != nil {
		return conditionValue, err
	}

	// Ignore looping on empty sources
	if len(sourcesJson) == 0 {
		return conditionValue, err
	}

	regexPattern := fmt.Sprintf(`%s%s([\s\S]*?)%s`, referenceOpeningPattern, regexp.QuoteMeta(openingPattern), regexp.QuoteMeta(closingPattern))
	regex := regexp.MustCompile(regexPattern)

	// Look for potential replacements on condition.value
	conditionValue = workloadManifest.Spec.Condition.Value
	matches := regex.FindAllStringSubmatch(conditionValue, -1)

	for _, match := range matches {
		// Look for the desired source in each replacement
		srcIndexString := match[0][strings.IndexByte(match[0], '[')+1 : strings.IndexByte(match[0], ']')]
		srcIndex, err := strconv.Atoi(srcIndexString)
		if err != nil {
			err = errors.New("invalid source index on condition value")
			return conditionValue, err
		}

		// Check index is lower than sources length
		if srcIndex >= len(sourcesJson) {
			err = errors.New("invalid index to the sources list")
			return conditionValue, err
		}

		match[1] = strings.TrimSpace(match[1])

		parsedValue := gjson.Get(sourcesJson[srcIndex], match[1])

		conditionValue = strings.Replace(conditionValue, match[0], parsedValue.String(), 1)
	}

	return conditionValue, err
}

// getPodTemplateAnnotations get podTemplate annotations from Deployment, Statefulset and Daemonset resources
func getPodTemplateAnnotations(obj *unstructured.Unstructured) (annotations []byte, err error) {
	var templateAnnotations map[string]interface{}

	// 1. Modify template annotations (spec.template.metadata.annotations) to include AnnotationRestartedAt
	templateAnnotations, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "metadata", "annotations")
	if err != nil {
		return annotations, err
	}
	// Take care about annotations not being present
	if !found || templateAnnotations == nil {
		templateAnnotations = map[string]interface{}{}
	}
	templateAnnotations[AnnotationRestartedAt] = time.Now().Format(time.RFC3339)

	// 2. Actually update the workload object against Kubernetes API
	annotations, err = json.Marshal(templateAnnotations)

	return annotations, err
}

// defaultPatchConstructor return a patch valid for core workload resources (deployments, statefulsets, daemonsets)
// adding previously existing annotations from podTemplate
func defaultPatchConstructor(obj *unstructured.Unstructured) (patch PatchConstructorPatchT, err error) {
	annotations, err := getPodTemplateAnnotations(obj)
	if err != nil {
		return patch, err
	}

	patch.Patch = []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":%s}}}}`, annotations))
	patch.PatchType = types.StrategicMergePatchType

	return patch, err
}

// deploymentPatchConstructor return a patch for deployment resources to be used in SetWorkloadRestartPatch
func deploymentPatchConstructor(obj *unstructured.Unstructured) (patch PatchConstructorPatchT, err error) {
	pausedValue, found, err := unstructured.NestedBool(obj.Object, "spec", "paused")
	if err != nil {
		return patch, err
	}

	if found && pausedValue {
		err = errors.New(PausedWorkloadRestartErrorMessage)
		return patch, err
	}

	patch, err = defaultPatchConstructor(obj)

	return patch, err
}

// argoRolloutPatchConstructor return a patch for Argo Rollout resources to be used in SetWorkloadRestartPatch
// Ref: https://argo-rollouts.readthedocs.io/en/stable/features/restart/
func argoRolloutPatchConstructor(obj *unstructured.Unstructured) (patch PatchConstructorPatchT, err error) {

	patchTimestamp := time.Now().Format(time.RFC3339)

	patch.Patch = []byte(fmt.Sprintf(`{"spec":{"restartAt":%s}}`, patchTimestamp))
	patch.PatchType = types.StrategicMergePatchType

	return patch, nil
}

// SetWorkloadRestartPatch restart a workload by changing an annotation.
// This will trigger an automatic reconciliation on the workload in the same way done by kubectl
func (r *WorkloadActionReconciler) SetWorkloadRestartPatch(ctx context.Context, obj *unstructured.Unstructured) (err error) {

	var patchConstructorMap map[string]PatchConstructorFuncPointerT = map[string]PatchConstructorFuncPointerT{
		ResourceKindDeployment:  deploymentPatchConstructor,
		ResourceKindStatefulSet: defaultPatchConstructor,
		ResourceKindDaemonSet:   defaultPatchConstructor,
		ResourceKindArgoRollout: argoRolloutPatchConstructor,
	}

	resourceType := obj.GetObjectKind().GroupVersionKind()
	// TODO: Check the group version just in case future changes on Kubernetes APIs
	//groupVersion := resourceType.GroupVersion()

	// 1. Check allowed workload types
	kind := resourceType.Kind
	if _, ok := patchConstructorMap[kind]; !ok {
		err = errors.New(WorkloadRestartNotSupportedErrorMessage)
		return err
	}

	// 2. Construct the patch with related function
	returnedPatch, err := patchConstructorMap[kind](obj)
	if err != nil {
		return err
	}

	// 3. Execute the patch
	err = r.Patch(ctx, obj, client.RawPatch(returnedPatch.PatchType, returnedPatch.Patch))
	if err != nil {
		err = fmt.Errorf(WorkloadActionAnnotationPatchErrorMessage, err)
	}

	return err
}

// getHttpContent execute a request against a server and return the response in bytes
func (r *WorkloadActionReconciler) getHttpContent(url string, auth HttpRequestAuth) (statusCode int, responseBytes []byte, err error) {

	httpClient := http.Client{Timeout: HttpRequestTimeoutDefaultValue}

	request, err := http.NewRequest(http.MethodGet, url, http.NoBody)
	if err != nil {
		return statusCode, responseBytes, err
	}

	request.Header.Add("Accept", HttpHeaderAcceptDefaultValue)

	if len(auth.Username) != 0 && len(auth.Password) != 0 {
		request.SetBasicAuth(auth.Username, auth.Password)
	}

	response, err := httpClient.Do(request)
	if err != nil {
		return statusCode, responseBytes, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		err = errors.New(fmt.Sprintf(UnsuccessfulRequestErrorMessage, response.StatusCode))
		return response.StatusCode, responseBytes, err
	}

	responseBytes, err = io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, responseBytes, err
	}

	return http.StatusOK, responseBytes, err
}

// getQueueFromSimpleResponse return the status for a specific queue on a non-paginated request
func (r *WorkloadActionReconciler) getQueueFromSimpleResponse(parsedUrl *url.URL, requestAuth HttpRequestAuth) (statusCode int, queuePool [][]byte, err error) {

	var responseBody []byte

	// Get the content of one page
	statusCode, responseBody, err = r.getHttpContent(parsedUrl.String(), requestAuth)
	if err != nil {
		return statusCode, queuePool, err
	}

	// Add item to queuePool
	queuePool = append(queuePool, responseBody)

	return statusCode, queuePool, err
}

// getQueuesFromPaginatedResponse return all the queues' status across paginated results, together
func (r *WorkloadActionReconciler) getQueuesFromPaginatedResponse(parsedUrl *url.URL, requestAuth HttpRequestAuth) (statusCode int, queuePool [][]byte, err error) {

	var responseBody []byte
	CurrentPage := StartingPageDefaultValue

	//
	regexParsedUrlQuery := parsedUrl.Query()
	regexParsedUrlQuery.Set("page", strconv.Itoa(StartingPageDefaultValue))
	parsedUrl.RawQuery = regexParsedUrlQuery.Encode()

	// Get the queues information from all the pages
	for {
		// Get the content of one page
		statusCode, responseBody, err = r.getHttpContent(parsedUrl.String(), requestAuth)
		if err != nil {
			return statusCode, queuePool, err
		}

		// Transform response into Go well-known struct
		data := RabbitPaginatedResponse{}
		err := json.Unmarshal(responseBody, &data)
		if err != nil {
			return statusCode, queuePool, err
		}

		// No items, don't waste compute resources processing
		if data.ItemCount == 0 {
			break
		}

		// Add page's items to queuePool
		for _, item := range data.Items {
			itemBytes, err := json.Marshal(item)
			if err != nil {
				// TODO: Should we completely return or just ignore current page?
				return statusCode, queuePool, err
			}
			queuePool = append(queuePool, itemBytes)
		}

		// Last page or no items. Exit
		if CurrentPage >= data.PageCount || data.ItemCount == 0 {
			break
		}

		// Items pending. Request them
		CurrentPage++
		temporaryParsedUrl := parsedUrl.Query()
		temporaryParsedUrl.Set("page", strconv.Itoa(CurrentPage))
		parsedUrl.RawQuery = temporaryParsedUrl.Encode()
	}

	return statusCode, queuePool, err
}

// reconcileWorkloadAction call Kubernetes API to actually workloadAction the resource
func (r *WorkloadActionReconciler) reconcileWorkloadAction(ctx context.Context, workloadActionManifest *rabbitstalkerv1alpha1.WorkloadAction) (err error) {

	// 1. Get the workload object
	targetObject, err := r.GetWorkloadResource(ctx, workloadActionManifest)
	if err != nil {
		r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
			metav1.ConditionFalse,
			ConditionReasonWorkloadNotFound,
			ConditionReasonWorkloadNotFoundMessage,
		))
		return err
	}

	// 2. Look for the Secret resources only when credentials refs are set in WorkloadAction
	var credentialsObjects []*corev1.Secret

	if !reflect.ValueOf(workloadActionManifest.Spec.RabbitConnection.Credentials).IsZero() {
		credentialsObjects, err = r.GetCredentialsResources(ctx, workloadActionManifest)
		if err != nil {
			r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
				metav1.ConditionFalse,
				ConditionReasonCredentialsNotFound,
				ConditionReasonCredentialsNotFoundMessage,
			))
			return err
		}
	}

	// 3. Fill the credentials only when the Secret resources where extracted
	var username, password []byte
	var usernameFound, passwordFound bool

	if len(credentialsObjects) == 2 {
		username, usernameFound = credentialsObjects[0].Data[workloadActionManifest.Spec.RabbitConnection.Credentials.Username.SecretRef.Key]
		password, passwordFound = credentialsObjects[1].Data[workloadActionManifest.Spec.RabbitConnection.Credentials.Password.SecretRef.Key]
		if !usernameFound || !passwordFound {
			r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
				metav1.ConditionFalse,
				ConditionReasonCredentialsNotFound,
				ConditionReasonCredentialsNotFoundMessage,
			))
			return err
		}
	}

	requestAuth := HttpRequestAuth{
		Username: string(username),
		Password: string(password),
	}

	// 4. Configure the URL and params for the HTTP request

	// Queues with literal name MUST be searched inside a vhost
	// TODO: Improve the status condition
	if !workloadActionManifest.Spec.RabbitConnection.UseRegex &&
		workloadActionManifest.Spec.RabbitConnection.Vhost == "" {
		err = errors.New(VhostNotDefinedErrorMessage)
		r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
			metav1.ConditionFalse,
			ConditionReasonRequiredFieldNotFound,
			ConditionReasonRequiredFieldNotFoundMessage,
		))
		return err
	}

	// Set request URL depending on literal or regex
	urlString := workloadActionManifest.Spec.RabbitConnection.Url

	parsedUrl, err := url.Parse(urlString)
	if err != nil {
		r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
			metav1.ConditionFalse,
			ConditionReasonUrlParsingFailed,
			ConditionReasonUrlParsingFailedMessage,
		))
		return err
	}

	// Set HTTPS when no scheme defined (secure by default)
	if parsedUrl.Scheme == "" {
		parsedUrl.Scheme = HttpSchemeDefaultValue
	}

	// Point to queues' monitoring endpoint
	parsedUrl = parsedUrl.JoinPath(RabbitAdminApiQueuesEndpoint)

	// Set the Vhost always when it's set on manifest
	if workloadActionManifest.Spec.RabbitConnection.Vhost != "" {
		parsedUrl = parsedUrl.JoinPath(workloadActionManifest.Spec.RabbitConnection.Vhost)
	}

	// Set queue name on URL when using a literal queueName
	if !workloadActionManifest.Spec.RabbitConnection.UseRegex {
		parsedUrl = parsedUrl.JoinPath(workloadActionManifest.Spec.RabbitConnection.Queue)
	}

	// Set initial query params for paginated requests when needed
	if workloadActionManifest.Spec.RabbitConnection.UseRegex {
		regexParsedUrlQuery := parsedUrl.Query()
		regexParsedUrlQuery.Set("page", strconv.Itoa(StartingPageDefaultValue))
		regexParsedUrlQuery.Set("page_size", strconv.Itoa(PageSizeDefaultValue))
		regexParsedUrlQuery.Set("name", workloadActionManifest.Spec.RabbitConnection.Queue)
		regexParsedUrlQuery.Set("use_regex", strconv.FormatBool(UseRegexDefaultValue))
		parsedUrl.RawQuery = regexParsedUrlQuery.Encode()
	}

	// 5. Make the HTTP request to the RabbitMQ admin API
	var statusCode int
	var queuePool [][]byte

	switch workloadActionManifest.Spec.RabbitConnection.UseRegex {
	case true:
		statusCode, queuePool, err = r.getQueuesFromPaginatedResponse(parsedUrl, requestAuth)
	default:
		statusCode, queuePool, err = r.getQueueFromSimpleResponse(parsedUrl, requestAuth)
	}

	if err != nil {
		r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
			metav1.ConditionFalse,
			ConditionReasonHttpResponseNotSuccessful,
			fmt.Sprintf(ConditionReasonHttpResponseNotSuccessfulMessage, statusCode),
		))
		return err
	}

	// Check whether some queue was found or not
	if len(queuePool) == 0 {
		err = errors.New(QueuesNotFoundErrorMessage)
		r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
			metav1.ConditionFalse,
			ConditionReasonQueueNotFound,
			ConditionReasonQueueNotFoundMessage,
		))
		return err
	}

	// 6. Evaluate the condition to execute an action.
	// The condition.value is computed first as it's computed only once.
	parsedValue, err := r.GetParsedConditionValue(ctx, workloadActionManifest)
	if err != nil {
		r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
			metav1.ConditionFalse,
			ConditionReasonConditionValueParsingFailed,
			ConditionReasonConditionValueParsingFailedMessage,
		))
		return err
	}

	// We allow regex for the queue.name. That means there could be different results on condition.key for each queue.
	// because of that, we need to compare them all against condition.value, one by one
	for queueItemIndex, queueItem := range queuePool {

		// Ref: https://gjson.dev/
		if !gjson.Valid(string(queueItem)) {
			err = errors.New(InvalidJsonErrorMessage)
			r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
				metav1.ConditionFalse,
				ConditionReasonHttpResponseNotValid,
				ConditionReasonHttpResponseNotValidMessage,
			))
			return err
		}

		// Parse the condition for current item
		// Condition is met for some item? go to action's execution
		parsedKey := gjson.GetBytes(queueItem, workloadActionManifest.Spec.Condition.Key)
		if parsedKey.String() == parsedValue {
			break
		}

		// Last loop. Condition was not met. Return without execution
		if len(queuePool) == queueItemIndex+1 {
			return err
		}
	}

	// 7. Condition is met. Execute the action
	switch workloadActionManifest.Spec.Action {
	case ActionRestart:
		err = r.SetWorkloadRestartPatch(ctx, targetObject)
	case ActionDelete:
		err = errors.New(DeleteActionNotImplementedErrorMessage)
	default:
		err = errors.New(InvalidActionErrorMessage)
		r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
			metav1.ConditionFalse,
			ConditionReasonInvalidAction,
			ConditionReasonInvalidActionMessage,
		))
		return err
	}

	if err != nil {
		r.UpdateWorkloadActionCondition(workloadActionManifest, r.NewWorkloadActionCondition(ConditionTypeWorkloadActionReady,
			metav1.ConditionFalse,
			ConditionReasonActionExecutionFailed,
			ConditionReasonActionExecutionFailedMessage,
		))
	}

	return err
}
