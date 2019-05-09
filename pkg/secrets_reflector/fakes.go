package secretsreflector

import (
	"context"
	"net/http"

	"github.com/dcos/client-go/dcos"
)

// fakeDCOSSecretsClient implements DCOSSecretsClient and is used for testing.
type fakeDCOSSecretsClient struct {
	OnCreate func(pathToSecret string) error
	OnUpdate func(pathToSecret string) error
}

func newFakeDCOSSecretsClient() DCOSSecretsClient {
	return &fakeDCOSSecretsClient{
		OnCreate: func(string) error { return nil },
		OnUpdate: func(string) error { return nil },
	}
}

func (f *fakeDCOSSecretsClient) CreateSecret(ctx context.Context, store string, pathToSecret string, secretsV1Secret dcos.SecretsV1Secret) (*http.Response, error) {
	return nil, f.OnCreate(pathToSecret)
}

func (f *fakeDCOSSecretsClient) UpdateSecret(ctx context.Context, store string, pathToSecret string, secretsV1Secret dcos.SecretsV1Secret) (*http.Response, error) {
	return nil, f.OnUpdate(pathToSecret)
}
