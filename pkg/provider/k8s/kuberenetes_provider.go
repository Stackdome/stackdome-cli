package k8s

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/ashishmax31/voyager-cli/pkg/client"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/provider"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/util/podutils"
)

const (
	LOCAL_PORT_FOR_SSH_TUNNEL = 17892
)

type k8sProvider struct {
	cfg            *config.Config
	providerClient client.ProviderClient
}

type k8sProviderStorageBackend struct {
	k8sProvider *k8sProvider
	resourceSvc string
}

func NewK8sProvider(cfg *config.Config, client client.ProviderClient) provider.Provider {
	return &k8sProvider{
		cfg:            cfg,
		providerClient: client,
	}
}

func (k *k8sProvider) CreateStorageResourceSSHTunnel(storageResourceInternalAddress string) provider.ProviderStorageSSHhandler {
	return &k8sProviderStorageBackend{
		k8sProvider: k,
		resourceSvc: storageResourceInternalAddress,
	}
}

func (k *k8sProviderStorageBackend) SetupSSHTunnel(ctx context.Context) error {
	if err := k.setupSSHTunnel(ctx); err != nil {
		return err
	}
	return nil
}

func (k *k8sProviderStorageBackend) setupSSHTunnel(ctx context.Context) error {
	println("in setupSSHTunnel")
	storageSvc := &corev1.Service{}
	// Name := k.resourceSvc
	if err := k.k8sProvider.providerClient.Get(
		ctx, types.NamespacedName{
			Name:      k.resourceSvc,
			Namespace: k.k8sProvider.cfg.ProviderConfig.Namespace,
		},
		storageSvc,
	); err != nil {
		return err
	}
	clientSet, err := kubernetes.NewForConfig(k.k8sProvider.providerClient.RestConfig)
	if err != nil {
		return err
	}
	attachablePod, err := attachablePodForObject(clientSet, storageSvc, time.Second*10)
	if err != nil {
		return err
	}
	req := clientSet.CoreV1().RESTClient().Post().Resource("pods").Namespace(attachablePod.Namespace).Name(attachablePod.Name).SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(k.k8sProvider.providerClient.RestConfig)
	if err != nil {
		return err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	ports := []string{fmt.Sprintf("%d:%d", LOCAL_PORT_FOR_SSH_TUNNEL, 22)}
	pf, err := portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
	if err != nil {
		return err
	}

	// TODO: Retries.
	go func() {
		if err := pf.ForwardPorts(); err != nil {
			fmt.Printf("portforward session errored: %s \n", err.Error())
			return
		}
		fmt.Println("portforward session stopped")
	}()
	// Wait for portforward to be ready.
	<-readyChan
	// Stop the portforward session when the context is cancelled.
	go func() {
		<-ctx.Done()
		close(stopChan)
	}()
	return nil
}

func attachablePodForObject(client *kubernetes.Clientset, object runtime.Object, timeout time.Duration) (*corev1.Pod, error) {
	switch t := object.(type) {
	case *corev1.Pod:
		return t, nil
	}
	namespace, selector, err := polymorphichelpers.SelectorsForObject(object)
	if err != nil {
		return nil, fmt.Errorf("cannot attach to %T: %v", object, err)
	}
	sortBy := func(pods []*corev1.Pod) sort.Interface { return sort.Reverse(podutils.ActivePods(pods)) }
	pod, _, err := polymorphichelpers.GetFirstPod(client.CoreV1(), namespace, selector.String(), timeout, sortBy)
	return pod, err
}
