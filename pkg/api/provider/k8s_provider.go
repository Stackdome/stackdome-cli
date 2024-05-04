package provider

type KubernetesProviderInfo struct {
	Namespace          string `json:"namespace"`
	Cacrt              []byte `json:"cacrt"`
	Token              string `json:"token"`
	ServiceAccountName string `json:"serviceAccountName"`
	ServerUrl          string `json:"serverUrl"`
}
