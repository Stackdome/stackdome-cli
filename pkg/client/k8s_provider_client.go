package client

import (
	"encoding/base64"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

type providerConfig interface {
	ProviderCACert() string
	ProviderServerURL() string
	ProviderToken() string
	Valid() bool
	SSHUser() string
}

type ProviderClient struct {
	config     providerConfig
	scheme     *runtime.Scheme
	RestConfig *rest.Config
	client.Client
}

func NewProviderClient(cfg providerConfig) (*ProviderClient, error) {
	if !cfg.Valid() {
		return nil, fmt.Errorf("config not valid")
	}

	// Validate CA cert.
	caCertBytes, err := base64.StdEncoding.DecodeString(cfg.ProviderCACert())
	if err != nil {
		return nil, fmt.Errorf("failed to base64 decode CA cert string: %w", err)
	}
	_, err = certutil.NewPoolFromBytes(caCertBytes)
	if err != nil {
		return nil, err
	}

	restConfig := &rest.Config{
		Host:        cfg.ProviderServerURL(),
		BearerToken: cfg.ProviderToken(),
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caCertBytes,
		},
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := workspacev1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	// Uncached k8s client.
	clientset, err := client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		return nil, err
	}

	return &ProviderClient{
		config:     cfg,
		scheme:     scheme,
		Client:     clientset,
		RestConfig: restConfig,
	}, nil
}

func (c *ProviderClient) SSHUser() string {
	return c.config.SSHUser()
}
