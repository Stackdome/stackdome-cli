package client

import (
	"fmt"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	certutil "k8s.io/client-go/util/cert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	workspacev1alpha1 "soradev.io/cluster-agent/api/v1alpha1"
)

type ProviderClient struct {
	config     *config.Config
	scheme     *runtime.Scheme
	RestConfig *rest.Config
	client.Client
}

func NewProviderClient(cfg *config.Config) (*ProviderClient, error) {
	if !cfg.Valid() {
		return nil, fmt.Errorf("config not valid")
	}

	// Validate CA cert.
	_, err := certutil.NewPoolFromBytes(cfg.ProviderConfig.CaCert)
	if err != nil {
		return nil, err
	}

	restConfig := &rest.Config{
		Host:        cfg.ProviderConfig.ServerUrl,
		BearerToken: cfg.ProviderConfig.Token,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: cfg.ProviderConfig.CaCert,
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
