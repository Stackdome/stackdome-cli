package init

import (
	"context"
	"fmt"
	"os"

	"github.com/ashishmax31/voyager-cli/cmd/common"
	"github.com/ashishmax31/voyager-cli/pkg/api/userworkspace"
	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/session"
	"github.com/ashishmax31/voyager-cli/pkg/workspace"
	"github.com/spf13/cobra"
)

var initArgs struct {
	voyagerFilePath string
}

// initCmd represents the init command

func NewInitCommand() *cobra.Command {
	var initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize your voyager workspace environment",
		Long:  `Initialize your voyager workspace environment`,
		RunE:  run,
		Args:  cobra.NoArgs,
	}
	initCmd.Flags().StringVar(&initArgs.voyagerFilePath, "voyagerfile-path", "", "--voyagerfile-path=voyagerfile.yaml")
	return initCmd
}

func run(_ *cobra.Command, _ []string) error {
	var voyagerFilePath string
	if len(initArgs.voyagerFilePath) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		voyagerFilePath, err = common.FindVoyagerFile(cwd)
		if err != nil {
			return err
		}
	} else {
		voyagerFilePath = initArgs.voyagerFilePath
	}
	if len(voyagerFilePath) == 0 {
		return fmt.Errorf("voyager file missing")
	}
	_, err := os.Stat(voyagerFilePath)
	if err != nil {
		return fmt.Errorf("failed to stat voyagerfile at %s: %w", voyagerFilePath, err)
	}

	if err := userworkspace.Validate(voyagerFilePath); err != nil {
		return fmt.Errorf("voyager file not valid: %w", err)
	}

	// Provider initialized.
	currSession, err := initializeProvider(context.Background())
	if err != nil {
		return err
	}
	userWorkspace, err := userworkspace.Unmarshal(voyagerFilePath)
	if err != nil {
		return err
	}
	if err := userWorkspace.Process(); err != nil {
		return err
	}
	handler, err := workspace.NewWorkspaceStorageHandler(currSession, *userWorkspace)
	if err != nil {
		return err
	}

	return handler.Init(context.Background())
}

func initializeProvider(ctx context.Context) (session.Session, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	s := session.NewSession(cfg)
	providerInfo, err := s.InitializeProvider(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.ProviderConfig == nil {
		cfg.ProviderConfig = &config.ComputeProviderConfig{}
	}
	cfg.ProviderConfig.CaCert = providerInfo.Cacrt
	cfg.ProviderConfig.Namespace = providerInfo.Namespace
	cfg.ProviderConfig.Token = providerInfo.Token
	cfg.ProviderConfig.ServerUrl = providerInfo.ServerUrl
	cfg.UserPrivateKeyPath = "/Users/ashishanand/.voyager/id_rsa"

	if err := config.Save(cfg); err != nil {
		return nil, err
	}
	return session.NewSessionWithProvider(cfg)
}
