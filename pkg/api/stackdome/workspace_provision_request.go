package stackdome

type WorkspaceProvisionRequestState string

const (
	WorkspaceProvisionRequestStateCompleted = "Completed"
	WorkspaceProvisionRequestStatePending   = "Pending"
	WorkspaceProvisionRequestStateError     = "Error"
)

type WorkspaceProvisionRequest struct {
	Id           string
	UserId       string
	OrgId        int32
	SshPublicKey string
	Status       *WorkspaceProvisionRequestStatus
	State        WorkspaceProvisionRequestState
	Message      string
}

type WorkspaceProvisionRequestStatus struct {
	WorkspaceNamespace           string
	WorkspaceServiceAccountname  string
	WorkspaceServiceaccountToken string
	ClusterCaCert                string
	ClusterUrl                   string
	Domain                       string
}
