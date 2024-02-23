package webhook

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/server"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/harvester/harvester/pkg/webhook/types"
)

var (
	certName = "pcidevices-webhook-tls"
	caName   = "pcidevices-webhook-ca"
	port     = int32(8443)

	mutationPath        = "/v1/webhook/mutation"
	validationPath      = "/v1/webhook/validation"
	failPolicyIgnore    = v1.Ignore
	sideEffectClassNone = v1.SideEffectClassNone
	namespace           = "harvester-system"
	threadiness         = 5
	MutatorName         = "pcidevices-mutator"
	ValidatorName       = "pcidevices-validator"
	whiteListedCiphers  = []uint16{tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	}
)

// AdmissionWebhookServer serves the mutating webhook for pcidevices
type AdmissionWebhookServer struct {
	context    context.Context
	restConfig *rest.Config
}

// New helps initialise a new AdmissionWebhookServer
func New(ctx context.Context, restConfig *rest.Config) *AdmissionWebhookServer {
	return &AdmissionWebhookServer{
		context:    ctx,
		restConfig: restConfig,
	}
}

// ListenAndServe starts the http listener and handlers
func (s *AdmissionWebhookServer) ListenAndServe() error {
	clients, err := NewClient(s.context, s.restConfig, threadiness)
	if err != nil {
		return err
	}

	RegisterIndexers(clients)

	mutationHandler, mutationResources, err := Mutation(clients)
	if err != nil {
		return err
	}

	validationHandler, validationResources, err := Validation(clients)
	if err != nil {
		return err
	}

	router := mux.NewRouter()
	router.Handle(mutationPath, mutationHandler)
	router.Handle(validationPath, validationHandler)
	if err := s.listenAndServe(clients, router, mutationResources, validationResources); err != nil {
		return err
	}

	if err := clients.Start(s.context); err != nil {
		return err
	}
	return nil
}

func (s *AdmissionWebhookServer) listenAndServe(clients *Clients, handler http.Handler, mutationResources []types.Resource, validationResources []types.Resource) error {
	apply := clients.Apply.WithDynamicLookup()
	clients.Core.Secret().OnChange(s.context, "secrets", func(key string, secret *corev1.Secret) (*corev1.Secret, error) {
		if secret == nil || secret.Name != caName || secret.Namespace != namespace || len(secret.Data[corev1.TLSCertKey]) == 0 {
			return nil, nil
		}
		logrus.Info("Sleeping for 15 seconds then applying webhook config")
		// Sleep here to make sure server is listening and all caches are primed
		time.Sleep(15 * time.Second)

		logrus.Debugf("Building validation rules...")
		validationRules := s.buildRules(validationResources)

		logrus.Debugf("Building mutation rules...")
		mutationRules := s.buildRules(mutationResources)

		mutatingWebhookConfiguration := &v1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: MutatorName,
			},
			Webhooks: []v1.MutatingWebhook{
				{
					Name: "pcidevices.harvesterhci.io",
					ClientConfig: v1.WebhookClientConfig{
						Service: &v1.ServiceReference{
							Namespace: namespace,
							Name:      "pcidevices-webhook",
							Path:      &mutationPath,
							Port:      &port,
						},
						CABundle: secret.Data[corev1.TLSCertKey],
					},
					Rules:                   mutationRules,
					FailurePolicy:           &failPolicyIgnore,
					SideEffects:             &sideEffectClassNone,
					AdmissionReviewVersions: []string{"v1", "v1beta1"},
				},
			},
		}

		validatingWebhookConfiguration := &v1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: ValidatorName,
			},
			Webhooks: []v1.ValidatingWebhook{
				{
					Name: "pcidevices.harvesterhci.io",
					ClientConfig: v1.WebhookClientConfig{
						Service: &v1.ServiceReference{
							Namespace: namespace,
							Name:      "pcidevices-webhook",
							Path:      &validationPath,
							Port:      &port,
						},
						CABundle: secret.Data[corev1.TLSCertKey],
					},
					Rules:                   validationRules,
					FailurePolicy:           &failPolicyIgnore,
					SideEffects:             &sideEffectClassNone,
					AdmissionReviewVersions: []string{"v1", "v1beta1"},
				},
			},
		}
		return secret, apply.WithOwner(secret).ApplyObjects(mutatingWebhookConfiguration, validatingWebhookConfiguration)
	})

	tlsName := fmt.Sprintf("pcidevices-webhook.%s.svc", namespace)

	return server.ListenAndServe(s.context, int(port), 0, handler, &server.ListenOpts{
		Secrets:       clients.Core.Secret(),
		CertNamespace: namespace,
		CertName:      certName,
		CAName:        caName,
		TLSListenerConfig: dynamiclistener.Config{
			SANs: []string{
				tlsName,
			},
			FilterCN: dynamiclistener.OnlyAllow(tlsName),
			TLSConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				CipherSuites: whiteListedCiphers,
			},
		},
	})
}

func (s *AdmissionWebhookServer) buildRules(resources []types.Resource) []v1.RuleWithOperations {
	rules := []v1.RuleWithOperations{}
	for _, rsc := range resources {
		logrus.Debugf("Add rule for %+v", rsc)
		scope := rsc.Scope
		rules = append(rules, v1.RuleWithOperations{
			Operations: rsc.OperationTypes,
			Rule: v1.Rule{
				APIGroups:   []string{rsc.APIGroup},
				APIVersions: []string{rsc.APIVersion},
				Resources:   rsc.Names,
				Scope:       &scope,
			},
		})
	}

	return rules
}
