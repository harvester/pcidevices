package helpers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rancher/wrangler/pkg/gvk"
	"github.com/rancher/wrangler/pkg/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubernetes/pkg/apis/admission"
	"k8s.io/kubernetes/pkg/apis/authentication"
)

// GenerateMutationRequest is a helper method to perform an admission review request
// against a webhook and return the response to call
func GenerateMutationRequest(source runtime.Object, endpoint string, cfg *rest.Config) (*admission.AdmissionResponse, error) {

	admissionReq, err := generateAdmissionRequest(source, cfg)
	if err != nil {
		return nil, err
	}

	reqBody, err := json.Marshal(admissionReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling admission req: %v", err)
	}

	respByte, err := makeWebhookCall(endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	respObj := make(map[string]interface{})
	err = json.Unmarshal(respByte, &respObj)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %v", err)
	}

	respInterface, ok := respObj["response"]
	if !ok {
		return nil, fmt.Errorf("found no response in the webhook response")
	}

	admissionRespJSON, err := json.Marshal(respInterface)
	if err != nil {
		return nil, fmt.Errorf("error marshalling response interface into json")
	}

	admissionResponse := &admission.AdmissionResponse{}

	err = json.Unmarshal(admissionRespJSON, admissionResponse)
	return admissionResponse, err
}

func makeWebhookCall(endpoint string, message []byte) ([]byte, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		TLSHandshakeTimeout: 5 * time.Second,
	}
	c := http.Client{
		Transport: tr,
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(message))
	if err != nil {
		return nil, fmt.Errorf("error generating new request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Host", "pcidevices-webhook.harvester-system.svc")
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error performing webhook request: %v", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("expected status 200 but got %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	// return response contents
	return io.ReadAll(resp.Body)
}

func generateAdmissionRequest(obj runtime.Object, cfg *rest.Config) (*admission.AdmissionReview, error) {
	objGVK, err := gvk.Get(obj)
	if err != nil {
		return nil, fmt.Errorf("error finding identifying gvk :%v", err)
	}

	resource, err := getResourceVersion(objGVK, cfg)
	if err != nil {
		return nil, fmt.Errorf("error getting resource version :%v", err)
	}

	unstructObj, err := unstructured.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("error converting to unstructured obj: %v", err)
	}

	req := &admission.AdmissionRequest{
		UID: types.UID(uuid.New().String()),
		Kind: metav1.GroupVersionKind{
			Kind:    objGVK.Kind,
			Group:   objGVK.Group,
			Version: objGVK.Version,
		},
		Resource: metav1.GroupVersionResource{
			Group:    objGVK.Group,
			Version:  objGVK.Version,
			Resource: resource,
		},
		RequestKind: &metav1.GroupVersionKind{
			Kind:    objGVK.Kind,
			Group:   objGVK.Group,
			Version: objGVK.Version,
		},
		RequestResource: &metav1.GroupVersionResource{
			Group:    objGVK.Group,
			Version:  objGVK.Version,
			Resource: resource,
		},
		Name:      unstructObj.GetName(),
		Namespace: unstructObj.GetNamespace(),
		Operation: admission.Create,
		UserInfo:  authentication.UserInfo{},
		Object:    obj,
	}

	return &admission.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: req}, nil
}

func getResourceVersion(gvk schema.GroupVersionKind, cfg *rest.Config) (string, error) {
	c := discovery.NewDiscoveryClientForConfigOrDie(cfg)

	groupResources, err := restmapper.GetAPIGroupResources(c)
	if err != nil {
		return "", fmt.Errorf("error getting api group resources: %v", err)
	}

	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	mapping, err := mapper.RESTMapping(gvk.GroupKind())
	return mapping.Resource.Resource, err
}
