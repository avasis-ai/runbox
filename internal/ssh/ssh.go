package ssh

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/avasis-ai/runbox/internal/config"
	"github.com/avasis-ai/runbox/pkg/shellquote"
)

type Result struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	DurationMs int64
}

type Opts struct {
	Timeout time.Duration
	Workdir string
	Quiet   bool
	Env     map[string]string
}

func sshTarget(m *config.Machine) string {
	return m.Host
}

func buildSSHArgs(m *config.Machine, remoteCmd string, opts *Opts) []string {
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10"}
	if m.Port != 0 && m.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", m.Port))
	}
	args = append(args, sshTarget(m))

	workdir := m.Workdir
	if opts != nil && opts.Workdir != "" {
		workdir = opts.Workdir
	}

	remote := shellquote.WrapForSSH(workdir, remoteCmd)
	args = append(args, remote)
	return args
}

func Exec(ctx context.Context, m *config.Machine, command string, opts *Opts) (*Result, error) {
	if opts == nil {
		opts = &Opts{}
	}
	args := buildSSHArgs(m, command, opts)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("ssh exec failed: %w\nstderr: %s", err, stderr.String())
		}
	}

	return &Result{
		Stdout:     strings.TrimRight(stdout.String(), "\n"),
		Stderr:     strings.TrimRight(stderr.String(), "\n"),
		ExitCode:   exitCode,
		DurationMs: duration,
	}, nil
}

func ExecStreaming(ctx context.Context, m *config.Machine, command string, opts *Opts) (*Result, error) {
	if opts == nil {
		opts = &Opts{}
	}
	args := buildSSHArgs(m, command, opts)

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("ssh exec failed: %w", err)
		}
	}

	return &Result{
		ExitCode:   exitCode,
		DurationMs: duration,
	}, nil
}

func TestConnection(m *config.Machine) (reachable bool, authMode string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=accept-new",
		sshTarget(m), "echo ok")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err == nil && strings.TrimSpace(stdout.String()) == "ok" {
		return true, "public-key", nil
	}

	cmd2 := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		sshTarget(m), "echo ok")
	var stderr2 bytes.Buffer
	cmd2.Stderr = &stderr2
	err2 := cmd2.Run()
	if err2 != nil {
		stderrStr := stderr.String()
		if stderrStr == "" {
			stderrStr = stderr2.String()
		}
		return false, "", fmt.Errorf("cannot connect to %s\nstderr: %s", m.Host, stderrStr)
	}

	return true, "password", nil
}

func CanConnectBatchMode(m *config.Machine) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5", sshTarget(m), "true")
	return cmd.Run() == nil
}

func InteractiveShell(m *config.Machine, workdir string) error {
	args := []string{sshTarget(m)}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if workdir != "" {
		fmt.Fprintf(os.Stderr, "cd %s\n", workdir)
	}
	return cmd.Run()
}

func CopyID(m *config.Machine, pubKeyPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{"-i", pubKeyPath}
	args = append(args, fmt.Sprintf("%s@%s", m.User, m.Host))

	cmd := exec.CommandContext(ctx, "ssh-copy-id", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
