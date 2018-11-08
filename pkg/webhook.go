package pkg

import (
	"encoding/json"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	log "github.com/sirupsen/logrus"
	"fmt"
	"strings"
	"k8s.io/apimachinery/pkg/types"
)

type AdmissionHook struct {
	clientSet   *kubernetes.Clientset
	initialized bool
}

func NewAdmissionHook(client *kubernetes.Clientset) *AdmissionHook {
	return &AdmissionHook{
		clientSet: client,
	}
}

func (a *AdmissionHook) MutatingResource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
		Group:    "admission.banzaicloud.com",
		Version:  "v1beta1",
		Resource: "spotschedulings",
	},
		"spotscheduling"
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (a *AdmissionHook) Admit(req *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {

	var resource string

	switch req.Kind.Kind {
	case "Deployment":
		log.Debug("found deployment in request")
		var deployment appsv1.Deployment
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			log.Warnf("could not unmarshal raw object, resource won't be mutated: %v", err)
			return a.successResponseNoPatch(req.UID, err.Error())
		}
		name := deployment.Name
		if deployment.Labels == nil || deployment.Labels["release"] == "" {
			log.Warnf("no release label found, resource won't be mutated")
			return a.successResponseNoPatch(req.UID, "no release label found")
		}
		releaseName := deployment.Labels["release"]
		resource = releaseName + "." + strings.ToLower(req.Kind.Kind) + "." + name
	default:
		return a.successResponseNoPatch(req.UID, fmt.Sprintf("resource type %s is not applicable for this webhook", req.Kind.Kind))
	}

	configMap, err := a.clientSet.CoreV1().ConfigMaps("pipeline-system").Get("spot-deploy-config", metav1.GetOptions{})
	if err != nil {
		log.WithField("resource", resource).Warnf("spot deployment ConfigMap couldn't be retrieved, resource won't be mutated: %v", err)
		return a.successResponseNoPatch(req.UID, err.Error())
	}

	if configMap.Data == nil {
		log.WithField("resource", resource).Warnf("there's no data in spot deploy ConfigMap, resource won't be mutated: %v", err)
		return a.successResponseNoPatch(req.UID, "no entry found in configMap")
	}

	pct, ok := configMap.Data[resource]
	if !ok {
		log.WithField("resource", resource).Warnf("resource not found in spot deploy ConfigMap, resource won't be mutated: %v", err)
		return a.successResponseNoPatch(req.UID, "no entry found in configMap")
	}

	log.WithField("resource", resource).Debug("creating patches")

	var patch []patchOperation
	patch = append(patch, patchOperation{
		Op:   "add",
		Path: "/spec/template/metadata/annotations",
		Value: map[string]string{
			"app.banzaicloud.io/odPercentage": pct,
		},
	})

	patch = append(patch, patchOperation{
		Op:    "add",
		Path:  "/spec/template/spec/schedulerName",
		Value: "spot-scheduler",
	})

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.WithField("resource", resource).Warnf("failed to marshal patch bytes, resource won't be mutated: %v", err)
		return a.successResponseNoPatch(req.UID, err.Error())
	}

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

func (a *AdmissionHook) successResponseNoPatch(uid types.UID, message string) *admissionv1beta1.AdmissionResponse {
	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
		UID:     uid,
		Result: &metav1.Status{
			Status:  "Success",
			Message: message,
		},
	}
}
