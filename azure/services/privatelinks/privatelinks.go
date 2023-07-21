package privatelinks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "privatelinks"

type PrivateLinkScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	PrivateLinkSpecs() []azure.ResourceSpecGetter
}

type Service struct {
	Scope PrivateLinkScope
	async.Reconciler
}

func New(scope PrivateLinkScope) (*Service, error) {
	client, err := newClient(scope)
	if err != nil {
		return nil, err
	}
	return &Service{
		Scope:      scope,
		Reconciler: async.New[armnetwork.PrivateLinkServicesClientCreateOrUpdateResponse, armnetwork.PrivateLinkServicesClientDeleteResponse](scope, client, client),
	}, nil
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	specs := s.Scope.PrivateLinkSpecs()
	if len(specs) == 0 {
		return nil
	}

	var resultingErr error
	for _, privateLinkSpec := range specs {
		_, err := s.CreateOrUpdateResource(ctx, privateLinkSpec, serviceName)
		if err != nil {
			if !azure.IsOperationNotDoneError(err) || resultingErr == nil {
				resultingErr = err
			}
		}
	}

	s.Scope.UpdatePutStatus(infrav1.PrivateLinksReadyCondition, serviceName, resultingErr)
	return resultingErr
}

func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	specs := s.Scope.PrivateLinkSpecs()
	if len(specs) == 0 {
		return nil
	}

	// We go through the list of PrivateLinkSpecs to delete each one, independently of the resultingErr of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (ie. error creating) -> operationNotDoneError (ie. creating in progress) -> no error (ie. created)
	var resultingErr error
	for _, privateLinkSpec := range specs {
		if err := s.DeleteResource(ctx, privateLinkSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || resultingErr == nil {
				resultingErr = err
			}
		}
	}
	s.Scope.UpdateDeleteStatus(infrav1.PrivateLinksReadyCondition, serviceName, resultingErr)
	return resultingErr
}

func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}
