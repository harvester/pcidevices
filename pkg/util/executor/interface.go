package executor

import (
	"os/exec"
	"path/filepath"
)

type Executor interface {
	CheckReady() ([]byte, error)
	Run(cmd string, args []string) ([]byte, error)
}

type LocalExecutor struct {
	envVars []string
}

func NewLocalExecutor(envVars []string) Executor {
	return &LocalExecutor{
		envVars: envVars,
	}
}

const (
	localExecutorPrefix = "/host"
	sriovManageCommand  = "/usr/lib/nvidia/sriov-manage"
	fileCommand         = "/usr/bin/file"
)

func (l *LocalExecutor) Run(cmd string, args []string) ([]byte, error) {
	// localExecutor is run inside pcidevices pod, so need to add `/host` to command
	cmd = filepath.Join(localExecutorPrefix, cmd)
	localCommand := exec.Command(cmd, args...)
	localCommand.Env = append(localCommand.Env, l.envVars...)
	return localCommand.Output()
}

// CheckReady checks if /host/usr/lib/nvidia/sriov-manage exists
func (l *LocalExecutor) CheckReady() ([]byte, error) {
	return l.Run(fileCommand, []string{filepath.Join(localExecutorPrefix, sriovManageCommand)})
}
