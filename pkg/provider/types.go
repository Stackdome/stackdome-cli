package provider

import (
	"context"
)

type Provider interface {
	CreateStorageResourceSSHTunnel(storageResourceAddress string) ProviderStorageSSHhandler
}

type ProviderStorageSSHhandler interface {
	SetupSSHTunnel(context.Context) error
}
