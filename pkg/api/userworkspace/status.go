package userworkspace

type WorkspaceStatus struct {
	WorkspaceName              string
	WorkspaceAvailablityStatus WorkspaceAvailablityStatus
	ResourceStatuses           []ResourceStatus
	VolumeStatuses             []VolumeStatus
}

type ResourceStatus struct {
	ResourceName string
	Available    bool
	Reason       string
	Message      string
	Addresses    []Address
	BuildStatus  BuildStatus
}

type BuildStatus struct {
	BuildName  string
	SourceHash string
	Completed  bool
	Reason     string
	Message    string
}

type VolumeStatus struct {
	VolumeName   string
	LocalPath    *string
	LastSyncedAt *string
	Available    bool
}

type Address struct {
	Port int
	Url  string
}

type WorkspaceAvailablityStatus struct {
	Available bool
	Reason    string
	Message   string
}
