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

	log.Info("***1*** hey I'm called")

	var name, releaseName string

	switch req.Kind.Kind {
	case "Deployment":
		log.Info("***2*** deployment")
		var deployment appsv1.Deployment
		if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
			log.Errorf("***err*** Could not unmarshal raw object: %v", err)
			return &admissionv1beta1.AdmissionResponse{
				Allowed: true,
				UID:     req.UID,
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		name = deployment.Name
		if deployment.Labels != nil {
			releaseName = deployment.Labels["release"]
		}
		log.Info("***3*** release name", releaseName)
	default:
		log.Info("***2/b*** valami mas")
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
			UID:     req.UID,
			Result: &metav1.Status{
				Message: fmt.Sprintf("resource type %s is not applicable for this webhook", req.Kind.Kind),
			},
		}
	}

	configMap, err := a.clientSet.CoreV1().ConfigMaps("pipeline-infra").Get("spot-deploy-config", metav1.GetOptions{})
	if err != nil {
		log.Info("***4*** cm err")
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
			UID:     req.UID,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info("***4*** cm")

	if configMap.Data == nil {
		log.Info("***5*** data nil")
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
			UID:     req.UID,
			Result:  &metav1.Status{Status: "Success", Message: "no entry found in configMap"},
		}
	}
	log.Info("***5/b*** data van, kulcs: %s", releaseName+"."+strings.ToLower(req.Kind.Kind)+"."+name)
	pct, ok := configMap.Data[releaseName+"."+strings.ToLower(req.Kind.Kind)+"."+name]
	if !ok {
		log.Info("***6/b*** data van de nem ok")
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
			UID:     req.UID,
			Result:  &metav1.Status{Status: "Success", Message: "no entry found in configMap"},
		}
	}

	log.Info("***6*** CM-ben megvan")

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

	log.Info("***7*** patched")

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		log.Info("***8*** patch hiba")
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
			UID:     req.UID,
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info("***8/b*** sending response with peccs")

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
