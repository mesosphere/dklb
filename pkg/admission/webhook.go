package admission

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	// admissionPAth is the path where the admission endpoint is served.
	admissionPath = "/admissionrequests"
	// bindAddress is the address ("host:port") which to bind to.
	bindAddress = "0.0.0.0:8443"
	// healthzPath is the path where the "health" endpoint is served.
	healthzPath = "/healthz"
)

var (
	// ingressGvk is the "GroupVersionKind" that corresponds to "Ingress" resources.
	ingressGvk = &schema.GroupVersionKind{Group: "extensions", Version: "v1beta1", Kind: "Ingress"}
	// ingressGvr is the "GroupVersionResource" that corresponds to "Ingress" resources.
	ingressGvr = metav1.GroupVersionResource{Group: "extensions", Version: "v1beta1", Resource: "ingresses"}
	// patchType is the type of patch sent in admission responses.
	patchType = admissionv1beta1.PatchTypeJSONPatch
	// serviceGvk is the "GroupVersionKind" that corresponds to "Service" resources.
	serviceGvk = &schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}
	// serviceGvr is the "GroupVersionResource" that corresponds to "Service" resources.
	serviceGvr = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
)

// Webhook represents an admission webhook.
type Webhook struct {
	// codecs is the codec factory to use to serialize/deserialize Kubernetes resources.
	codecs serializer.CodecFactory
	// clusterName is the name of the Mesos framework that corresponds to the current Kubernetes cluster.
	clusterName string
	// tlsCertificate is the TLS certificate to use for the server.
	tlsCertificate tls.Certificate
}

// NewWebhook creates a new instance of the admission webhook.
func NewWebhook(clusterName string, tlsCertificate tls.Certificate) *Webhook {
	// Create a new scheme and register the "Ingress" and "Service" types so we can serialize/deserialize them.
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(extsv1beta1.SchemeGroupVersion, &extsv1beta1.Ingress{})
	scheme.AddKnownTypes(corev1.SchemeGroupVersion, &corev1.Service{})
	return &Webhook{
		codecs:         serializer.NewCodecFactory(scheme),
		clusterName:    clusterName,
		tlsCertificate: tlsCertificate,
	}
}

// Run starts the HTTP server that backs the admission webhook.
func (w *Webhook) Run(stopCh chan struct{}) error {
	// Create an HTTP server and register handler functions to back the admission webhook.
	mux := http.NewServeMux()
	mux.HandleFunc(admissionPath, w.handleAdmission)
	mux.HandleFunc(healthzPath, handleHealthz)
	srv := http.Server{
		Addr:    bindAddress,
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{w.tlsCertificate},
		},
	}

	// Shutdown the server when stopCh is closed.
	go func() {
		<-stopCh
		ctx, fn := context.WithTimeout(context.Background(), 5*time.Second)
		defer fn()
		if err := srv.Shutdown(ctx); err != nil {
			log.Errorf("failed to shutdown the admission webhook: %v", err)
		} else {
			log.Debug("the admission webhook has been shutdown")
		}
	}()

	// Start listening and serving requests.
	log.Debug("starting the admission webhook")
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// handleAdmission handles the HTTP part of admission.
func (w *Webhook) handleAdmission(res http.ResponseWriter, req *http.Request) {
	// Read the request's body.
	var body []byte
	if req.Body != nil {
		if data, err := ioutil.ReadAll(req.Body); err == nil {
			body = data
		}
	}

	// Fail if the request's content type is not "application/json".
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		res.WriteHeader(http.StatusUnsupportedMediaType)
		return
	}

	// aReq is the AdmissionReview that was sent to the admission webhook.
	aReq := admissionv1beta1.AdmissionReview{}
	// rRes is the AdmissionReview that will be returned.
	aRes := admissionv1beta1.AdmissionReview{}

	// Deserialize the requested AdmissionReview and, if successful, pass it to the provided admission function.
	deserializer := w.codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(body, nil, &aReq); err != nil {
		aRes.Response = admissionResponseFromError(err)
	} else {
		aRes.Response = w.validateAndMutateResource(aReq)
	}
	// Set the request's UID in the response object.
	aRes.Response.UID = aReq.Request.UID

	// Serialize the response AdmissionReview.
	resp, err := json.Marshal(aRes)
	if err != nil {
		log.Errorf("failed to write admissionreview: %v", err)
		return
	}
	if _, err := res.Write(resp); err != nil {
		log.Errorf("failed to write admissionreview: %v", err)
		return
	}
}

func (w *Webhook) validateAndMutateResource(rev admissionv1beta1.AdmissionReview) *admissionv1beta1.AdmissionResponse {
	var (
		// currentObj will contain the resource in its current form.
		// It MUST NOT be modified, as it is used as the basis for the patch to apply as a result of the current request.
		currentObj runtime.Object
		// currentGVK will contain the GVK (Group/Version/Kind) of the current resource.
		// It is used to identify the kind of resource (Ingress/Service) we are dealing with in the current request.
		currentGVK *schema.GroupVersionKind
		// mutatedObj will contain a clone of "currentObj".
		// It will be modified as required in order to explicitly set the values of all annotations.
		mutatedObj runtime.Object
		// previousObj will contain the previous version of the current resource.
		// It will only be set and considered in case the current operation's type is "UPDATE".
		previousObj runtime.Object
		// err will contain any error we may encounter during the validation/mutation process.
		err error
	)

	// Set "currentGVK" based on the provided GVR (group/version/resource).
	switch rev.Request.Resource {
	case ingressGvr:
		// We're dealing with an "Ingress" resource.
		currentGVK = ingressGvk
	case serviceGvr:
		// We're dealing with a "Service" resource.
		currentGVK = serviceGvk
	default:
		// We're dealing with an unsupported resource, so we must fail.
		return admissionResponseFromError(fmt.Errorf("failed to validate resource with unsupported gvr %s", rev.Request.Resource.String()))
	}

	// Deserialize the current object.
	currentObj, _, err = w.codecs.UniversalDeserializer().Decode(rev.Request.Object.Raw, currentGVK, nil)
	if err != nil {
		return admissionResponseFromError(fmt.Errorf("failed to deserialize the current object: %v", err))
	}

	// If the current request corresponds to an update operation, we also deserialize the previous version of the resource so we can validate the transition.
	if rev.Request.Operation == admissionv1beta1.Update {
		previousObj, _, err = w.codecs.UniversalDeserializer().Decode(rev.Request.OldObject.Raw, currentGVK, nil)
		if err != nil {
			return admissionResponseFromError(fmt.Errorf("failed to deserialize the previous object: %v", err))
		}
	}

	// Perform validation on the current resource according to its type.
	switch cObj := currentObj.(type) {
	case *extsv1beta1.Ingress:
		var (
			previousIng *extsv1beta1.Ingress
		)
		if previousObj != nil {
			previousIng = previousObj.(*extsv1beta1.Ingress)
		}
		mutatedObj, err = w.validateAndMutateIngress(cObj, previousIng)
	case *corev1.Service:
		var (
			previousSvc *corev1.Service
		)
		if previousObj != nil {
			previousSvc = previousObj.(*corev1.Service)
		}
		mutatedObj, err = w.validateAndMutateService(cObj, previousSvc)
	default:
		return admissionResponseFromError(fmt.Errorf("failed to validate resource of unsupported type %v", reflect.TypeOf(currentObj)))
	}

	// If an error was returned as a result of validation, we must fail.
	if err != nil {
		return admissionResponseFromError(err)
	}

	// Create a patch containing the changes to apply to the resource.
	patch, err := CreateRFC6902Patch(currentObj, mutatedObj)
	if err != nil {
		return admissionResponseFromError(fmt.Errorf("failed to create patch: %v", err))
	}

	// Return an admission response that admits the resource and contains the patch to be applied.
	return &admissionv1beta1.AdmissionResponse{
		Allowed:   true,
		Patch:     patch,
		PatchType: &patchType,
	}
}

// admissionResponseFromError creates an admission response based on the specified error.
func admissionResponseFromError(err error) *admissionv1beta1.AdmissionResponse {
	return &admissionv1beta1.AdmissionResponse{
		Result: &metav1.Status{
			Message: err.Error(),
		},
	}
}
