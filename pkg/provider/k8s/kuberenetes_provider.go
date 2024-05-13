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
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
	"k8s.io/kubectl/pkg/polymorphichelpers"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/podutils"
)

const (
	MaxRetryOnConnectionLoss = 20
)

const (
	SERVICE_TARGET = "service"
	POD_TARGET     = "pod"
)

type portForwardTarget struct {
	podName    *string
	svcName    *string
	targetType string
}

func NewServiceTarget(svcName string) provider.Target {
	return &portForwardTarget{
		svcName:    &svcName,
		targetType: SERVICE_TARGET,
	}
}

func NewPodTarget(podName string) provider.Target {
	return &portForwardTarget{
		podName:    &podName,
		targetType: POD_TARGET,
	}
}

func (p *portForwardTarget) TargetName() string {
	if p.podName != nil {
		return *p.podName
	} else {
		return *p.svcName
	}
}

func (p *portForwardTarget) TargetType() string {
	return p.targetType
}

type k8sProvider struct {
	cfg            *config.Config
	providerClient client.ProviderClient
}

func NewK8sProvider(cfg *config.Config, client client.ProviderClient) provider.Provider {
	return &k8sProvider{
		cfg:            cfg,
		providerClient: client,
	}
}

func (k *k8sProvider) SetupSSHTunnel(ctx context.Context, localPort int, target provider.Target) (chan struct{}, error) {
	logrus.Debug("in setupSSHTunnel")
	return k.SetupPortForward(ctx, localPort, 22, target)
}

func (k *k8sProvider) SSHUser() string {
	return k.cfg.ProviderConfig.SSHUserName
}

func (k *k8sProvider) Execute(ctx context.Context, target provider.Target, cmd []string, interactive bool) error {
	clientset, err := kubernetes.NewForConfig(k.providerClient.RestConfig)
	if err != nil {
		return err
	}
	attachablePod, err := k.attachablePodFromTarget(ctx, clientset, target)
	if err != nil {
		return err
	}

	req := clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(attachablePod.Name).
		Namespace(attachablePod.Namespace).
		SubResource("exec")

	execOptions := &corev1.PodExecOptions{
		Command: cmd,
		Stdout:  true,
		Stderr:  true,
	}

	streamOptions := remotecommand.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if interactive {
		execOptions.TTY = true
		execOptions.Stdin = true
		streamOptions.Tty = true
		streamOptions.Stdin = os.Stdin
	}

	req.VersionedParams(execOptions, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(k.providerClient.RestConfig, "POST", req.URL())
	if err != nil {
		return err
	}
	return executor.StreamWithContext(ctx, streamOptions)
}

func (k *k8sProvider) SetupPortForward(ctx context.Context, localPort int, targetPort int, target provider.Target) (chan struct{}, error) {
	stopChan := make(chan struct{})
	portForwardExitChan := make(chan struct{})
	go k.runPortforwarder(ctx, localPort, targetPort, target, stopChan, portForwardExitChan)
	go func() {
		<-ctx.Done()
		close(stopChan)
	}()
	return portForwardExitChan, nil
}

func (k *k8sProvider) runPortforwarder(ctx context.Context, localport int, targetPort int, target provider.Target, stopChan, exitChan chan struct{}) {
	defer close(exitChan)
	defer logrus.Info("portforward session stopped")
	for attempt := 1; attempt <= MaxRetryOnConnectionLoss; attempt++ {
		currentReadyChan := make(chan struct{})
		pf, err := k.newPortForwarder(ctx, localport, targetPort, target, stopChan, currentReadyChan)
		if err != nil {
			logrus.Errorf("failed to create portforwarder: %s", err.Error())
			return
		}
		if err := pf.ForwardPorts(); err != nil {
			if err == portforward.ErrLostConnectionToPod {
				logrus.Warnf("lost connection to pod... retrying to establish connection, attempt: %d", attempt)
				continue
			} else {
				logrus.Errorf("portforward session errored: %s", err.Error())
				return
			}
		}
	}
}

func (k *k8sProvider) newPortForwarder(
	ctx context.Context,
	localPort int,
	targetPort int,
	target provider.Target,
	stopChan, readyChan chan struct{}) (*portforward.PortForwarder, error) {
	clientSet, err := kubernetes.NewForConfig(k.providerClient.RestConfig)
	if err != nil {
		return nil, err
	}
	attachablePod, err := k.attachablePodFromTarget(ctx, clientSet, target)
	if err != nil {
		return nil, err
	}

	req := clientSet.CoreV1().RESTClient().Post().Resource("pods").Namespace(attachablePod.Namespace).Name(attachablePod.Name).SubResource("portforward")
	transport, upgrader, err := spdy.RoundTripperFor(k.providerClient.RestConfig)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	ports := []string{fmt.Sprintf("%d:%d", localPort, targetPort)}
	return portforward.New(dialer, ports, stopChan, readyChan, os.Stdout, os.Stderr)
}

func (k *k8sProvider) attachablePodFromTarget(ctx context.Context, client *kubernetes.Clientset, target provider.Target) (*corev1.Pod, error) {
	if target.TargetType() == SERVICE_TARGET {
		storageSvc := &corev1.Service{}
		if err := k.providerClient.Get(
			ctx, types.NamespacedName{
				Name:      target.TargetName(),
				Namespace: k.cfg.ProviderConfig.Namespace,
			},
			storageSvc,
		); err != nil {
			return nil, err
		}
		attachablePod, err := attachablePodForObject(client, storageSvc, time.Second*10)
		if err != nil {
			return nil, err
		}
		return attachablePod, nil
	}
	referencedPod := &corev1.Pod{}
	if err := k.providerClient.Get(
		ctx, types.NamespacedName{
			Name:      target.TargetName(),
			Namespace: k.cfg.ProviderConfig.Namespace,
		},
		referencedPod,
	); err != nil {
		return nil, err
	}
	return referencedPod, nil
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
