package webhook

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/harvester/harvester/pkg/webhook/types"
	"github.com/rancher/dynamiclistener"
	"github.com/rancher/dynamiclistener/server"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

var (
	certName = "pcidevices-webhook-tls"
	caName   = "pcidevices-webhook-ca"
	port     = int32(8443)

	mutationPath        = "/v1/webhook/mutation"
	failPolicyIgnore    = v1.Ignore
	sideEffectClassNone = v1.SideEffectClassNone
	namespace           = "harvester-system"
	threadiness         = 5
	MutatorName         = "pcidevices-mutator"
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

	router := mux.NewRouter()
	router.Handle(mutationPath, mutationHandler)
	if err := s.listenAndServe(clients, router, mutationResources); err != nil {
		return err
	}

	if err := clients.Start(s.context); err != nil {
		return err
	}
	return nil
}

func (s *AdmissionWebhookServer) listenAndServe(clients *Clients, handler http.Handler, mutationResources []types.Resource) error {
	apply := clients.Apply.WithDynamicLookup()
	clients.Core.Secret().OnChange(s.context, "secrets", func(key string, secret *corev1.Secret) (*corev1.Secret, error) {
		if secret == nil || secret.Name != caName || secret.Namespace != namespace || len(secret.Data[corev1.TLSCertKey]) == 0 {
			return nil, nil
		}
		logrus.Info("Sleeping for 15 seconds then applying webhook config")
		// Sleep here to make sure server is listening and all caches are primed
		time.Sleep(15 * time.Second)

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

		return secret, apply.WithOwner(secret).ApplyObjects(mutatingWebhookConfiguration)
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
