package privatelinks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type azureClient struct {
	privateLinks *armnetwork.PrivateLinkServicesClient
}

// newClient creates a new route tables client from an authorizer.
func newClient(auth azure.Authorizer) (*azureClient, error) {
	opts, err := azure.ARMClientOptions(auth.CloudEnvironment())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create routetables client options")
	}
	factory, err := armnetwork.NewClientFactory(auth.SubscriptionID(), auth.Token(), opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create armnetwork client factory")
	}
	return &azureClient{factory.NewPrivateLinkServicesClient()}, nil
}

// Get returns the specified private link.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.azureClient.Get")
	defer done()
	resp, err := ac.privateLinks.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), nil)
	if err != nil {
		return nil, err
	}
	return resp.PrivateLinkService, nil
}

func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string, parameters interface{}) (result interface{}, poller *runtime.Poller[armnetwork.PrivateLinkServicesClientCreateOrUpdateResponse], err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.azureClient.CreateOrUpdateAsync")
	defer done()

	privateLink, ok := parameters.(armnetwork.PrivateLinkService)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a network.PrivateLinkService", parameters)
	}

	opts := &armnetwork.PrivateLinkServicesClientBeginCreateOrUpdateOptions{ResumeToken: resumeToken}
	poller, err = ac.privateLinks.BeginCreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), privateLink, opts)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	pollOpts := &runtime.PollUntilDoneOptions{Frequency: async.DefaultPollerFrequency}
	resp, err := poller.PollUntilDone(ctx, pollOpts)
	if err != nil {
		// If an error occurs, return the poller.
		// This means the long-running operation didn't finish in the specified timeout.
		return nil, poller, err
	}

	// if the operation completed, return a nil poller
	return resp.PrivateLinkService, nil, err
}

func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string) (poller *runtime.Poller[armnetwork.PrivateLinkServicesClientDeleteResponse], err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.azureClient.DeleteAsync")
	defer done()

	opts := &armnetwork.PrivateLinkServicesClientBeginDeleteOptions{ResumeToken: resumeToken}
	poller, err = ac.privateLinks.BeginDelete(ctx, spec.ResourceGroupName(), spec.ResourceName(), opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	pollOpts := &runtime.PollUntilDoneOptions{Frequency: async.DefaultPollerFrequency}
	_, err = poller.PollUntilDone(ctx, pollOpts)
	if err != nil {
		// if an error occurs, return the poller.
		// this means the long-running operation didn't finish in the specified timeout.
		return poller, err
	}

	// if the operation completed, return a nil poller.
	return nil, err
}
