package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ashishmax31/voyager-cli/pkg/api/v1alpha1"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/tools"
)

type Args struct {
	ResourceName     *string
	WorkspaceName    *string
	AllResources     *bool
	Interactive      *bool
	TailLines        *int64
	Follow           *bool
	StackFilePath    *string
	ExecuteCmd       []string
	AllWorkspaces    *bool
	RemoveStorage    *bool
	CurrentWorkspace *bool
}

func (a *Args) IsAllResources() bool {
	return a.AllResources != nil && *a.AllResources
}

func (a *Args) IsCurrentWorkspace() bool {
	return a.CurrentWorkspace != nil && *a.CurrentWorkspace
}

func (a *Args) IsInteractive() bool {
	return a.Interactive != nil && *a.Interactive
}

func (a *Args) IsFollow() bool {
	return a.Follow != nil && *a.Follow
}

func (a *Args) IsTailLines() bool {
	return a.TailLines != nil
}

func (a *Args) GetResourceName() string {
	if a.ResourceName == nil {
		return ""
	}
	return *a.ResourceName
}

func (a *Args) GetWorkspaceName() string {
	if a.WorkspaceName == nil {
		return ""
	}
	return *a.WorkspaceName
}

func (a *Args) GetStackFilePath() string {
	if a.StackFilePath == nil {
		return ""
	}
	return *a.StackFilePath
}

func (a *Args) GetTailLines() int64 {
	if a.TailLines == nil {
		return 0
	}
	return *a.TailLines
}

func (a *Args) IsAllWorkspaces() bool {
	return a.AllWorkspaces != nil && *a.AllWorkspaces
}

func (a *Args) IsRemoveStorage() bool {
	return a.RemoveStorage != nil && *a.RemoveStorage
}

type Runtime struct {
	Command   string
	cfg       *Config
	ConfigDir string
	DepsDir   string
	LogDir    string
	Session   session.Session
	Args      Args
}

func (a *Runtime) UserStack() (*v1alpha1.UserStack, error) {
	if a.Args.StackFilePath == nil {
		return nil, errors.New("stack file path is not set")
	}
	var res v1alpha1.UserStack
	if err := tools.UnmarshalYamlFile(*a.Args.StackFilePath, &res); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stack file: %w", err)
	}
	if a.Config().CurrentWorkspace != nil {
		res.Name = *a.Config().CurrentWorkspace
	}
	if err := res.ReadEnvFiles(); err != nil {
		return nil, fmt.Errorf("failed to read env files: %w", err)
	}

	return &res, nil
}

func NewRuntime(command string, args Args) (*Runtime, error) {
	cfg, err := Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	dir, err := ConfigDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get config dir: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	depsDir := filepath.Join(dir, "bin")

	if err := os.MkdirAll(depsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create deps dir: %w", err)
	}

	logsDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs dir: %w", err)
	}

	var s session.Session
	if cfg.ProviderConfigPresent() {
		s, err = session.NewSession(cfg, true)
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
	} else {
		s, err = session.NewSession(cfg, false)
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
	}

	return &Runtime{
		cfg:       cfg,
		Command:   command,
		Args:      args,
		Session:   s,
		ConfigDir: dir,
		DepsDir:   depsDir,
		LogDir:    logsDir,
	}, nil
}

func (r *Runtime) Config() *Config {
	return r.cfg
}

func (r *Runtime) SaveConfig() error {
	return Save(r.cfg)
}

func (r *Runtime) CreateLogFile(name string) (*os.File, error) {
	return os.Create(filepath.Join(r.LogDir, name))
}

func (r *Runtime) CurrentWorkspaceStorageName() (string, error) {
	currentWorkspaceName := r.Config().CurrentWorkspace
	if currentWorkspaceName == nil {
		return "", fmt.Errorf("current workspace not set")
	}
	return fmt.Sprintf("%s-%s", *currentWorkspaceName, "storage"), nil
}
