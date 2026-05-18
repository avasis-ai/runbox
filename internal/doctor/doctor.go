package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/avasis-ai/runbox/internal/config"
	"github.com/avasis-ai/runbox/internal/ssh"
	"github.com/avasis-ai/runbox/internal/sshconfig"
)

type NetworkChecks struct {
	TailscaleDetected bool   `json:"tailscaleDetected"`
	MagicDNS          string `json:"magicDNS"`
	HostResolution    string `json:"hostResolution"`
	ResolvedIP        string `json:"resolvedIP,omitempty"`
}

type SSHChecks struct {
	Reachable     bool   `json:"reachable"`
	AuthMode      string `json:"authMode"`
	PublicKeyAuth bool   `json:"publicKeyAuth"`
	SSHAgent      bool   `json:"sshAgent"`
	Multiplexing  bool   `json:"multiplexing"`
}

type RemoteChecks struct {
	Reachable       bool   `json:"reachable"`
	AuthMode        string `json:"authMode"`
	PublicKeyAuth   bool   `json:"publicKeyAuth"`
	OS              string `json:"os"`
	TmuxInstalled   bool   `json:"tmuxInstalled"`
	RsyncInstalled  bool   `json:"rsyncInstalled"`
	WorkdirExists   bool   `json:"workdirExists"`
	RunboxDirExists bool   `json:"runboxDirExists"`
	ShellUsable     bool   `json:"shellUsable"`
	TestCommand     bool   `json:"testCommand"`
}

type DoctorResult struct {
	Machine          string        `json:"machine"`
	Network          NetworkChecks `json:"network"`
	SSH              SSHChecks     `json:"ssh"`
	Remote           RemoteChecks  `json:"remote"`
	RecommendedFixes []string      `json:"recommendedFixes"`
}

type LocalChecks struct {
	SSHInstalled       bool
	SSHAgentRunning    bool
	SSHAddAvailable    bool
	RsyncInstalled     bool
	TmuxInstalled      bool
	TailscaleInstalled bool
	TailscaleRunning   bool
	MagicDNSResolves   bool
	ResolvedIP         string
	SSHConfigWritable  bool
	SSHConfigExists    bool
	ControlSocketDir   bool
	RunboxKeyExists    bool
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func RunLocal(host string) *LocalChecks {
	c := &LocalChecks{}
	c.SSHInstalled = commandExists("ssh")
	c.SSHAddAvailable = commandExists("ssh-add")
	c.RsyncInstalled = commandExists("rsync")
	c.TmuxInstalled = commandExists("tmux")

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if _, err := os.Stat(sock); err == nil {
			c.SSHAgentRunning = true
		}
	}

	c.TailscaleInstalled = commandExists("tailscale")
	if c.TailscaleInstalled {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
		out, err := cmd.Output()
		if err == nil && len(out) > 0 {
			c.TailscaleRunning = true
		}
	}

	if host != "" && c.TailscaleRunning {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "tailscale", "ip", host)
		out, err := cmd.Output()
		if err == nil {
			ip := strings.TrimSpace(string(out))
			if ip != "" {
				c.MagicDNSResolves = true
				c.ResolvedIP = ip
			}
		}
	}

	configPath := sshconfig.ConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		c.SSHConfigExists = true
		f, err := os.Open(configPath)
		if err == nil {
			f.Close()
			c.SSHConfigWritable = true
		}
	} else {
		dir := sshconfig.SSHDir()
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			c.SSHConfigWritable = true
		}
	}

	ctrlDir := sshconfig.ControlSocketDir()
	if _, err := os.Stat(ctrlDir); err == nil {
		c.ControlSocketDir = true
	}

	if _, err := os.Stat(sshconfig.KeyPath()); err == nil {
		c.RunboxKeyExists = true
	}

	return c
}

func RunRemote(m *config.Machine) *RemoteChecks {
	rc := &RemoteChecks{}

	reachable, authMode, _ := ssh.TestConnection(m)
	rc.Reachable = reachable
	rc.AuthMode = authMode

	if !reachable {
		return rc
	}

	rc.PublicKeyAuth = ssh.CanConnectBatchMode(m)

	if !rc.PublicKeyAuth {
		return rc
	}

	ctx := context.Background()

	if out, err := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", m.Host, "uname -s").Output(); err == nil {
		rc.OS = strings.ToLower(strings.TrimSpace(string(out)))
	}

	checkRemote := func(cmd string) bool {
		ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel2()
		c := exec.CommandContext(ctx2, "ssh", "-o", "BatchMode=yes", m.Host, "command -v "+shellQuote(cmd))
		return c.Run() == nil
	}

	rc.TmuxInstalled = checkRemote("tmux")
	rc.RsyncInstalled = checkRemote("rsync")
	rc.ShellUsable = true

	if m.Workdir != "" {
		ctx3, cancel3 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel3()
		c := exec.CommandContext(ctx3, "ssh", "-o", "BatchMode=yes", m.Host,
			"test -d "+shellQuote(m.Workdir)+" && echo ok")
		if out, err := c.Output(); err == nil && strings.TrimSpace(string(out)) == "ok" {
			rc.WorkdirExists = true
		}
	} else {
		rc.WorkdirExists = true
	}

	{
		ctx4, cancel4 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel4()
		c := exec.CommandContext(ctx4, "ssh", "-o", "BatchMode=yes", m.Host,
			"test -d ~/.runbox && echo ok")
		if out, err := c.Output(); err == nil && strings.TrimSpace(string(out)) == "ok" {
			rc.RunboxDirExists = true
		}
	}

	{
		ctx5, cancel5 := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel5()
		c := exec.CommandContext(ctx5, "ssh", "-o", "BatchMode=yes", m.Host, "echo runbox-ok")
		if out, err := c.Output(); err == nil && strings.TrimSpace(string(out)) == "runbox-ok" {
			rc.TestCommand = true
		}
	}

	return rc
}

func Run(m *config.Machine) *DoctorResult {
	local := RunLocal(m.Host)
	remote := RunRemote(m)

	result := &DoctorResult{
		Machine: m.Host,
		Network: NetworkChecks{
			TailscaleDetected: local.TailscaleRunning,
			ResolvedIP:        local.ResolvedIP,
		},
		SSH: SSHChecks{
			Reachable:     remote.Reachable,
			AuthMode:      remote.AuthMode,
			PublicKeyAuth: remote.PublicKeyAuth,
			SSHAgent:      local.SSHAgentRunning,
			Multiplexing:  local.ControlSocketDir,
		},
		Remote: *remote,
	}

	if local.MagicDNSResolves {
		result.Network.MagicDNS = "ok"
		result.Network.HostResolution = "ok"
	} else if local.TailscaleRunning {
		result.Network.MagicDNS = "unresolved"
		result.Network.HostResolution = "unresolved"
	} else {
		result.Network.MagicDNS = "tailscale-not-running"
		result.Network.HostResolution = "tailscale-not-running"
	}

	var fixes []string
	if !remote.PublicKeyAuth && remote.Reachable {
		fixes = append(fixes, "auth")
	}
	if !local.ControlSocketDir {
		fixes = append(fixes, "multiplex")
	}
	hasExisting, _ := sshconfig.HasBlock(m.Host)
	if !hasExisting {
		fixes = append(fixes, "ssh-config")
	}
	if remote.Reachable && remote.PublicKeyAuth {
		if !remote.WorkdirExists || !remote.RunboxDirExists || !remote.TmuxInstalled || !remote.RsyncInstalled {
			fixes = append(fixes, "remote-runtime")
		}
	}
	result.RecommendedFixes = fixes

	return result
}

func (r *DoctorResult) PrintText() {
	fmt.Printf("Machine\n")
	fmt.Printf("  Host: %s\n", r.Machine)
	fmt.Println()

	fmt.Printf("Network\n")
	ts := "not detected"
	if r.Network.TailscaleDetected {
		ts = "detected"
	}
	fmt.Printf("  Tailscale: %s\n", ts)
	fmt.Printf("  MagicDNS: %s\n", r.Network.MagicDNS)
	if r.Network.ResolvedIP != "" {
		fmt.Printf("  Host resolution: %s -> %s\n", r.Machine, r.Network.ResolvedIP)
	}
	fmt.Println()

	fmt.Printf("SSH\n")
	fmt.Printf("  Reachable: %s\n", boolStr(r.SSH.Reachable))
	fmt.Printf("  Auth: %s\n", r.SSH.AuthMode)
	fmt.Printf("  Public key auth: %s\n", boolStr(r.SSH.PublicKeyAuth))
	fmt.Printf("  ssh-agent: %s\n", boolStr(r.SSH.SSHAgent))
	fmt.Printf("  Multiplexing: %s\n", boolStr(r.SSH.Multiplexing))
	fmt.Println()

	fmt.Printf("Remote runtime\n")
	fmt.Printf("  OS: %s\n", r.Remote.OS)
	fmt.Printf("  tmux: %s\n", installedStr(r.Remote.TmuxInstalled))
	fmt.Printf("  rsync: %s\n", installedStr(r.Remote.RsyncInstalled))
	fmt.Printf("  workdir: %s\n", existsStr(r.Remote.WorkdirExists))
	fmt.Printf("  ~/.runbox: %s\n", existsStr(r.Remote.RunboxDirExists))
	fmt.Println()

	if len(r.RecommendedFixes) > 0 {
		fmt.Printf("Recommended fixes\n")
		for i, fix := range r.RecommendedFixes {
			fmt.Printf("  [%d] %s\n", i+1, fixDescription(fix))
		}
		fmt.Println()
		fmt.Printf("Run:\n")
		fmt.Printf("  runbox fix %s --all\n", r.Machine)
	}
}

func boolStr(b bool) string {
	if b {
		return "OK"
	}
	return "not configured"
}

func installedStr(b bool) string {
	if b {
		return "installed"
	}
	return "missing"
}

func existsStr(b bool) string {
	if b {
		return "exists"
	}
	return "missing"
}

func fixDescription(fix string) string {
	switch fix {
	case "auth":
		return "Configure passwordless SSH using ssh-agent"
	case "multiplex":
		return "Enable SSH multiplexing"
	case "remote-runtime":
		return "Create remote workdir and ~/.runbox directory"
	case "ssh-config":
		return "Generate SSH config entry"
	default:
		return fix
	}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
