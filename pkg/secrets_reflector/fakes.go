package secretsreflector

import (
	"context"
	"net/http"

	"github.com/dcos/client-go/dcos"
)

// fakeDCOSSecretsClient implements DCOSSecretsClient and is used for
// testing
type fakeDCOSSecretsClient struct {
	Err          error
	Resp         *http.Response
	ValidatePath func(pathToSecret string) error
}

func newFakeDCOSSecretsClient() DCOSSecretsClient {
	return &fakeDCOSSecretsClient{}
}

func (f *fakeDCOSSecretsClient) CreateSecret(ctx context.Context, store string, pathToSecret string, secretsV1Secret dcos.SecretsV1Secret) (*http.Response, error) {
	if f.ValidatePath != nil {
		err := f.ValidatePath(pathToSecret)
		if err != nil {
			return nil, err
		}
	}
	return f.Resp, f.Err
}

func (f *fakeDCOSSecretsClient) DeleteSecret(ctx context.Context, store string, pathToSecret string) (*http.Response, error) {
	if f.ValidatePath != nil {
		err := f.ValidatePath(pathToSecret)
		if err != nil {
			return nil, err
		}
	}
	return f.Resp, f.Err
}
