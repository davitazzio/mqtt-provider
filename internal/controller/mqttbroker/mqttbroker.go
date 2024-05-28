/*
Copyright 2022 The Crossplane Authors.

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

package mqttbroker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-mqttprovider/apis/mqtt/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-mqttprovider/apis/v1alpha1"
	mqttservice "github.com/crossplane/provider-mqttprovider/internal/controller/mqttService"
	"github.com/crossplane/provider-mqttprovider/internal/features"
)

const (
	errNotMqttBroker = "managed resource is not a MqttBroker custom resource"
	errTrackPCUsage  = "cannot track ProviderConfig usage"
	errGetPC         = "cannot get ProviderConfig"
	errGetCreds      = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

var (
	newmqttservice = func(_ []byte, remoteHost string) (*mqttservice.Mqttservice, error) {
		return mqttservice.GetInstance(remoteHost), nil
	}
)

// Setup adds a controller that reconciles MqttBroker managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.MqttBrokerGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.MqttBrokerGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: newmqttservice,
			logger:       o.Logger}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.MqttBroker{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	logger       logging.Logger
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(creds []byte, MqttBrokerName string) (*mqttservice.Mqttservice, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.MqttBroker)
	if !ok {
		return nil, errors.New(errNotMqttBroker)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	svc, err := c.newServiceFn(data, cr.Spec.ForProvider.NodeAddress)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: svc, logger: c.logger}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API. In practice this
	// would be something like an AWS SDK client.
	logger  logging.Logger
	service *mqttservice.Mqttservice
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.MqttBroker)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotMqttBroker)
	}
	cr.Status.SetConditions(v1.ReconcileSuccess())
	cr.SetConditions(v1.Available())

	numProcessi, err := mqttservice.Observebroker(cr.Spec.ForProvider.NodeAddress, cr.Spec.ForProvider.RemoteUser, c.logger)

	if err != nil {
		c.logger.Debug("errore: il processo è da creare ")
		cr.Status.AtProvider.Active = false
	} else {
		cr.Status.AtProvider.QueueState = numProcessi
		cr.Status.AtProvider.Active = true
	}

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: cr.Status.AtProvider.Active,
		// c.service.GetActive(),

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: true,

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.MqttBroker)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotMqttBroker)
	}

	// c.logger.Debug(fmt.Sprintf("Creating resource %s with parameter 'node_address' %s.", cr.Name, cr.Spec.ForProvider.NodeAddress))

	_, err := mqttservice.StartBroker(cr.Spec.ForProvider.NodeAddress, cr.Spec.ForProvider.RemoteUser, c.logger)
	if err != nil {
		c.logger.Debug("errore nell'avvio del Mqtt-Broker remoto")
	}

	// // an example of resource creatino connecting to an HTTP server
	// sendHTTPReq(cr.Spec.ForProvider.NodeAddress, cr.Spec.ForProvider.NodePort, cr.Spec.ForProvider.Service, c.logger)

	// newCondition := true
	mg.SetConditions(v1.Available())

	meta.SetExternalCreateSucceeded(mg, time.Now())
	return managed.ExternalCreation{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.MqttBroker)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotMqttBroker)
	}

	c.logger.Debug(fmt.Sprintf("Updating: %+v", cr))

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.MqttBroker)
	if !ok {
		return errors.New(errNotMqttBroker)
	}

	err := mqttservice.TerminateBroker(cr.Spec.ForProvider.NodeAddress, cr.Spec.ForProvider.RemoteUser, c.logger)
	if err != nil {
		c.logger.Debug("errore nell'osservazione del MqttBrokero remoto")
	}

	cr.Status.AtProvider.Active = false

	mg.SetConditions(v1.Deleting())

	return nil
}

func sendHTTPReq(nodeAddress string, nodePort string, service *string, logger logging.Logger) {

	address := "http://" + nodeAddress + ":" + nodePort + "/" + *service

	logger.Debug(fmt.Sprintf("Sending HTTP request to node %s", address))

	resp, err := http.Get(address)
	if err != nil {
		logger.Debug("Error decoding request.")
		logger.Debug(err.Error())
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			logger.Debug(err.Error())
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debug("Error reading response.")
		logger.Debug(err.Error())
	}
	logger.Debug(string(body))
}
