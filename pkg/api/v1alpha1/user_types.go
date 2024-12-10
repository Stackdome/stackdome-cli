package v1alpha1

import "time"

const (
	WorkspaceUserAvailableCondition = "Available"

	ConditionTrue    = "True"
	ConditionFalse   = "False"
	ConditionUnknown = "Unknown"
)

type WorkspaceUser struct {
	ID           string
	UserID       string
	OrgID        string
	SshPublicKey string
	Workspaces   []string
	Version      int32
	Status       *WorkspaceUserStatus
	State        string
	Message      string
}

func (w *WorkspaceUser) IsAvailable() bool {
	return w.Status != nil && w.Status.IsAvailable() && w.Version == w.Status.ObservedVersion
}

type WorkspaceUserStatus struct {
	ObservedVersion       int32
	ProvisionedWorkspaces []ProvisionedWorkspace
	ServiceAccountname    string
	ServiceAccountToken   string
	ClusterCaCert         string
	ClusterUrl            string
	Conditions            []Condition
}

func (w WorkspaceUserStatus) GetServiceAccountName() string {
	return w.ServiceAccountname
}

func (w WorkspaceUserStatus) GetServiceAccountToken() string {
	return w.ServiceAccountToken
}

func (w WorkspaceUserStatus) GetClusterCaCert() string {
	return w.ClusterCaCert
}

func (w WorkspaceUserStatus) GetClusterUrl() string {
	return w.ClusterUrl
}

func (w WorkspaceUserStatus) GetProvisionedWorkspaces() []ProvisionedWorkspace {
	return w.ProvisionedWorkspaces
}

type ProvisionedWorkspace struct {
	WorkspaceName string `json:"workspaceName"`
	Namespace     string `json:"namespace"`
}

type Condition struct {
	Type               string
	Status             string
	LastTransitionTime time.Time
	Reason             string
	Message            string
}

func (w *WorkspaceUserStatus) IsAvailable() bool {
	availableCond := GetCondition(w.Conditions, WorkspaceUserAvailableCondition)
	return availableCond != nil && availableCond.Status == ConditionTrue
}

func GetCondition(conditions []Condition, conditionType string) *Condition {
	for _, cond := range conditions {
		if cond.Type == conditionType {
			return &cond
		}
	}
	return nil
}

func (w *WorkspaceUserStatus) ContainsWorkspace(workspaceName string) bool {
	for _, ns := range w.ProvisionedWorkspaces {
		if ns.WorkspaceName == workspaceName {
			return true
		}
	}
	return false
}

type User struct {
	// User's ID
	Id string
	// User's name
	Name string
	// User's username
	Username string
	// User's email address
	Email string
	// User's organisation
	Organisation string
	// User's role
	Role           string
	OrganisationID string
}
