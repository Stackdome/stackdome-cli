package sync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/ashishmax31/voyager-cli/pkg/config"
	"github.com/ashishmax31/voyager-cli/pkg/tools"
	"github.com/fsnotify/fsnotify"
	"github.com/gofrs/flock"
)

const (
	LOCAL_PORT_FOR_SSH_TUNNEL = 17892
)

type DestinationSSHTunnelHandler interface {
	SetupSSHTunnel(ctx context.Context) error
}

type SourceDestintionPair struct {
	Source      string
	Destination string
}

type SourceDestintionList []SourceDestintionPair

type mutagenSync struct {
	cfg             *config.Config
	lockPath        string
	lockDir         string
	mutgenBinaryDir string
}

func NewMutagenSyncer(cfg *config.Config, lockDir string, mutgenBinaryDir string) Syncer {
	w := &mutagenSync{
		cfg:             cfg,
		lockDir:         lockDir,
		mutgenBinaryDir: mutgenBinaryDir,
	}

	w.lockPath = filepath.Join(lockDir, "voyager-daemon.lock")
	return w
}

func (m *mutagenSync) Initialized(context.Context) (bool, error) {
	lock := flock.New(m.lockPath)
	locked, err := lock.TryLock()
	if err != nil {
		return false, fmt.Errorf("failed to acquire file lock: %w", err)
	}
	defer lock.Close()
	return !locked, nil
}

func (m *mutagenSync) Sync(context.Context) error {
	println("in mutagen sync")
	syncProcess := exec.Command(m.mutagenBinaryPath(), "sync", "flush", "--all")
	syncProcess.Stdout = os.Stdout
	syncProcess.Stderr = os.Stderr
	// Wait for the forked process to complete.
	if err := syncProcess.Run(); err != nil {
		return fmt.Errorf("error when flushing mutagen sync sessions: %w", err)
	}
	return nil
}

func (m *mutagenSync) SetupSyncSession(ctx context.Context, spec SourceDestintionList, dstSSHhandler DestinationSSHTunnelHandler) error {
	lock := flock.New(m.lockPath)
	locked, err := lock.TryLock()
	if err != nil {
		return fmt.Errorf("failed to acquire file lock: %w", err)
	}
	if !locked {
		println("not locked! Some other instance already running")
		return nil
	}
	println("in mutagen SetupSyncSession")
	defer lock.Close()
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	if err := dstSSHhandler.SetupSSHTunnel(ctx); err != nil {
		return err
	}
	if err := m.ensureSSHConfig(); err != nil {
		return err
	}
	// Start sync sessions for SourceDestintionList.
	for _, srcDestPair := range spec {
		err := m.createMutgenSync(ctx, srcDestPair.Source, srcDestPair.Destination)
		if err != nil {
			return err
		}
	}
	defer m.cleanupMutagenDaemons()

	watcher := tools.NewFileSystemWatcher(m.lockDir,
		tools.WithOperationFilter(fsnotify.Remove), tools.WithFileWatches(m.lockPath))
	err = watcher.StartWatch()
	if err != nil {
		return err
	}

	// We wait for either the context to be cancelled or the lockfile to be deleted
	// to stop all the daemons and exit.
	defer watcher.Stop()
	println("watching for lock file to be deleted")
	select {
	case <-watcher.NotifyChan():
		fmt.Println("lock file deleted, stopping all daemons")
		return nil
	case <-ctx.Done():
		return nil
	}
}

func (m *mutagenSync) cleanupMutagenDaemons() error {
	if err := m.stopAllMutgenSyncSessions(); err != nil {
		return err
	}
	if err := m.stopMutagenDaemon(); err != nil {
		return err
	}
	return nil
}

func (m *mutagenSync) createMutgenSync(ctx context.Context, srcPath string, dstPath string) error {
	user := "root"
	alpha := srcPath
	beta := fmt.Sprintf("%s@localhost:%d:%s", user, LOCAL_PORT_FOR_SSH_TUNNEL, dstPath)
	daemonProcess := exec.Command(m.mutagenBinaryPath(), "sync", "create", alpha, beta, "-m", "two-way-safe")
	daemonProcess.Stdout = os.Stdout
	daemonProcess.Stderr = os.Stderr
	// Wait for the forked process to complete.
	if err := daemonProcess.Run(); err != nil {
		return fmt.Errorf("error when creating mutagen sync sessions: %w", err)
	}
	return nil
}

func (m *mutagenSync) stopAllMutgenSyncSessions() error {
	cleanupProcess := exec.Command(m.mutagenBinaryPath(), "sync", "terminate", "--all")
	cleanupProcess.Stdout = os.Stdout
	cleanupProcess.Stderr = os.Stderr
	// Wait for the forked process to complete.
	if err := cleanupProcess.Run(); err != nil {
		return fmt.Errorf("error when removing mutagen sync sessions: %w", err)
	}
	return nil
}

func (m *mutagenSync) stopMutagenDaemon() error {
	stopDaemonProcess := exec.Command(m.mutagenBinaryPath(), "daemon", "stop")
	stopDaemonProcess.Stdout = os.Stdout
	stopDaemonProcess.Stderr = os.Stderr
	// Wait for the forked process to complete.
	if err := stopDaemonProcess.Run(); err != nil {
		return fmt.Errorf("error when stopping mutagen daemon: %w", err)
	}
	return nil
}

func (m *mutagenSync) mutagenBinaryPath() string {
	return filepath.Join(m.mutgenBinaryDir, "mutagen")
}
func (m *mutagenSync) ensureSSHConfig() error {
	sshConfigPath, err := ensureSSHConfigFileExists()
	if err != nil {
		return fmt.Errorf("failed to create ssh config file: %w", err)
	}

	voyagerConfigDir, err := m.cfg.ConfigDir()
	if err != nil {
		return err
	}
	if err := tools.EnsureVoyagerSshConfig(sshConfigPath, voyagerConfigDir, &tools.SSHConfig{
		Port:             LOCAL_PORT_FOR_SSH_TUNNEL,
		User:             "root",
		IdentityFilePath: m.cfg.UserPrivateKeyPath,
	}); err != nil {
		return err
	}
	return nil
}

func ensureSSHConfigFileExists() (string, error) {
	// Get the current user
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	// Construct the SSH directory path
	sshDirPath := filepath.Join(currentUser.HomeDir, ".ssh")

	// Create the SSH directory if it doesn't exist
	if _, err := os.Stat(sshDirPath); os.IsNotExist(err) {
		err = os.Mkdir(sshDirPath, 0700)
		if err != nil {
			return "", err
		}
	}

	// Construct the SSH config file path
	sshConfigPath := filepath.Join(sshDirPath, "config")

	// Check if the SSH config file exists
	_, err = os.Stat(sshConfigPath)
	if os.IsNotExist(err) {
		// Create the SSH config file if it doesn't exist
		file, err := os.Create(sshConfigPath)
		if err != nil {
			return "", err
		}
		file.Close()
	} else if err != nil {
		return "", err
	}

	return sshConfigPath, nil
}
