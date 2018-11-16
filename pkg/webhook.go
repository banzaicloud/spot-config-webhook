package pkg

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	SpotAnnotationKey      = "spot_annotation_key"
	SpotApiResourceGroup   = "spot_api_resource_group"
	SpotApiResourceVersion = "spot_api_resource_version"
	SpotApiResourceName    = "spot_api_resource_name"
	SpotConfigMapNamespace = "spot_configmap_namespace"
	SpotConfigMapName      = "spot_configmap_name"
	SpotSchedulerName      = "spot_scheduler_name"
)

type AdmissionHook struct {
	clientSet         *kubernetes.Clientset
	resourceGroup     string
	resourceVersion   string
	resourceName      string
	spotAnnotationKey string
	configMapNs       string
	configMapName     string
	schedulerName     string
	initialized       bool
}

func NewAdmissionHook(client *kubernetes.Clientset, annotationKey, group, version, resourceName, cmNs, cmName, schedulerName string) *AdmissionHook {
	log.WithField(SpotAnnotationKey, annotationKey).
		WithField(SpotApiResourceGroup, group).
		WithField(SpotApiResourceVersion, version).
		WithField(SpotApiResourceName, resourceName).
		WithField(SpotConfigMapNamespace, cmNs).
		WithField(SpotConfigMapName, cmName).
		WithField(SpotSchedulerName, schedulerName).
		Infof("admission hook parameters")
	return &AdmissionHook{
		clientSet:         client,
		resourceGroup:     group,
		resourceVersion:   version,
		resourceName:      resourceName,
		configMapNs:       cmNs,
		configMapName:     cmName,
		schedulerName:     schedulerName,
		spotAnnotationKey: annotationKey,
	}
}

func (a *AdmissionHook) MutatingResource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
		Group:    a.resourceGroup,
		Version:  a.resourceVersion,
		Resource: fmt.Sprintf("%ss", a.resourceName),
	},
		a.resourceName
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (a *AdmissionHook) Admit(req *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {

	var name string
	var labels map[string]string
	var podAnnotations map[string]string

	switch req.Kind.Kind {
	case "Deployment":
		log.Debug("found Deployment in request")
		var deployment appsv1.Deployment
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			return a.successResponseNoPatch(req.UID, "", errors.Wrap(err, "could not unmarshal raw object"))
		}
		name = deployment.Name
		labels = deployment.Labels
		podAnnotations = deployment.Spec.Template.Annotations
	case "StatefulSet":
		log.Debug("found StatefulSet in request")
		var statefulSet appsv1.StatefulSet
		if err := json.Unmarshal(req.Object.Raw, &statefulSet); err != nil {
			return a.successResponseNoPatch(req.UID, "", errors.Wrap(err, "could not unmarshal raw object"))
		}
		name = statefulSet.Name
		labels = statefulSet.Labels
		podAnnotations = statefulSet.Spec.Template.Annotations
	case "ReplicaSet":
		log.Debug("found ReplicaSet in request")
		var replicaSet appsv1.ReplicaSet
		if err := json.Unmarshal(req.Object.Raw, &replicaSet); err != nil {
			return a.successResponseNoPatch(req.UID, "", errors.Wrap(err, "could not unmarshal raw object"))
		}
		name = replicaSet.Name
		labels = replicaSet.Labels
		podAnnotations = replicaSet.Spec.Template.Annotations
	default:
		return a.successResponseNoPatch(req.UID, "", errors.Errorf("resource type %s is not applicable for this webhook", req.Kind.Kind))
	}

	if labels == nil || labels["release"] == "" {
		return a.successResponseNoPatch(req.UID, "", errors.New("no release label found"))
	}
	resource := labels["release"] + "." + strings.ToLower(req.Kind.Kind) + "." + name

	configMap, err := a.clientSet.CoreV1().ConfigMaps(a.configMapNs).Get(a.configMapName, metav1.GetOptions{})
	if err != nil {
		return a.successResponseNoPatch(req.UID, resource, errors.Wrap(err, "spot deployment ConfigMap couldn't be retrieved"))
	}

	if configMap.Data == nil {
		return a.successResponseNoPatch(req.UID, resource, errors.New("there's no data in spot deploy ConfigMap"))
	}

	pct, ok := configMap.Data[resource]
	if !ok {
		return a.successResponseNoPatch(req.UID, resource, errors.New("resource not found in spot deploy ConfigMap"))
	}

	log.WithField("resource", resource).Debug("creating patches")

	var patch []patchOperation
	if v, ok := podAnnotations[a.spotAnnotationKey]; !ok {
		patch = append(patch, patchOperation{
			Op:   "add",
			Path: "/spec/template/metadata/annotations",
			Value: map[string]string{
				a.spotAnnotationKey: pct,
			},
		})
	} else {
		log.WithField("resource", resource).WithField(a.spotAnnotationKey, v).Debug("annotation is already present")
	}

	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/spec/template/spec/schedulerName",
		Value: a.schedulerName,
	})

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return a.successResponseNoPatch(req.UID, resource, errors.Wrap(err, "failed to marshal patch bytes"))
	}

	a.cleanupSpotConfigMap(resource)

	log.WithField("resource", resource).Debug("sending patched response")

	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
		UID:     req.UID,
		Result:  &metav1.Status{Status: "Success", Message: ""},
		Patch:   patchBytes,
		PatchType: func() *admissionv1beta1.PatchType {
			pt := admissionv1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

func (a *AdmissionHook) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	return nil
}

func (a *AdmissionHook) successResponseNoPatch(uid types.UID, resource string, err error) *admissionv1beta1.AdmissionResponse {
	log.WithField("resource", resource).WithError(err).Warn("resource won't be mutated")
	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
		UID:     uid,
		Result: &metav1.Status{
			Status:  "Success",
			Message: err.Error(),
		},
	}
}

func (a *AdmissionHook) cleanupSpotConfigMap(resource string) error {
	cm, err := a.clientSet.CoreV1().ConfigMaps(a.configMapNs).Get(a.configMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if cm.Data == nil {
		return nil
	}
	_, ok := cm.Data[resource]
	if ok {
		delete(cm.Data, resource)
	}
	_, err = a.clientSet.CoreV1().ConfigMaps(a.configMapNs).Update(cm)
	if err != nil {
		return err
	}
	log.WithField("resource", resource).Debug("deleted entry from ConfigMap")
	return nil
}
