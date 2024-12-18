package v1alpha1

type ResourceBuild struct {
	ID                    string
	WorkspaceID           string
	WorkspaceName         string
	WorkspaceResourceID   string
	WorkspaceResourceName string
	SourceHash            string
	ImageRegistry         string
	Current               bool
	Status                *ResourceBuildStatus
}

type ResourceBuildStatus struct {
	State      string
	Conditions []Condition
	ImageURL   string
	SourceHash string
}
