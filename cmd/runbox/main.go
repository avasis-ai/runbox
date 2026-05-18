package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/avasis-ai/runbox/internal/auth"
	"github.com/avasis-ai/runbox/internal/config"
	"github.com/avasis-ai/runbox/internal/doctor"
	"github.com/avasis-ai/runbox/internal/policy"
	"github.com/avasis-ai/runbox/internal/ssh"
	"github.com/avasis-ai/runbox/internal/sshconfig"
	"github.com/spf13/cobra"
)

var (
	flagJSON     bool
	flagQuiet    bool
	flagVerbose  bool
	flagHost     string
	flagUser     string
	flagWorkdir  string
	flagPort     int
	flagForce    bool
	flagTail     bool
	flagDelete   bool
	flagNoPass   bool
	flagAll      bool
	flagAuth     bool
	flagSSHCfg   bool
	flagMultiplex bool
	flagRuntime  bool
	flagDetached bool
	flagSession  string
	flagApprove  bool
)

var rootCmd = &cobra.Command{
	Use:   "runbox",
	Short: "Turn any remote machine into an agent runtime",
	Long: `Runbox turns any remote machine — Mac mini, Linux box, GPU PC, Jetson, or edge device —
into a persistent agent runtime. No IPs. No repeated passwords. No lost SSH sessions.`,
	Version: "0.1.0",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVarP(&flagQuiet, "quiet", "q", false, "suppress output")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(fixCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(shellCmd)
	rootCmd.AddCommand(sessionCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(pullCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(bootstrapCmd)
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print runbox version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("runbox v0.1.0")
		},
	})
}

func loadConfig() *config.Config {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	return cfg
}

func getMachine(cfg *config.Config, name string) *config.Machine {
	m, err := cfg.GetMachine(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	return m
}

func outputJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

var initCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Register a remote machine",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := loadConfig()

		m := &config.Machine{
			Host:      flagHost,
			User:      flagUser,
			Port:      flagPort,
			Workdir:   flagWorkdir,
			Transport: "ssh",
			Auth:      "ssh-agent",
			Multiplex: true,
		}

		if m.Host == "" {
			m.Host = name
		}
		if m.User == "" {
			m.User = "root"
		}

		if err := cfg.AddMachine(name, m); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if flagJSON {
			outputJSON(map[string]string{
				"machine": name,
				"host":    m.Host,
				"user":    m.User,
				"status":  "registered",
			})
			return
		}

		fmt.Printf("Machine %q registered.\n", name)
		fmt.Printf("  host: %s\n", m.Host)
		fmt.Printf("  user: %s\n", m.User)
		fmt.Printf("  workdir: %s\n", m.Workdir)
		fmt.Println()
		fmt.Printf("Next steps:\n")
		fmt.Printf("  runbox doctor %s\n", name)
		fmt.Printf("  runbox fix %s --all\n", name)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered machines",
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()

		if flagJSON {
			outputJSON(cfg.Machines)
			return
		}

		if len(cfg.Machines) == 0 {
			fmt.Println("No machines registered. Run: runbox init <name>")
			return
		}

		fmt.Printf("%-15s %-25s %-10s %-20s\n", "NAME", "HOST", "USER", "WORKDIR")
		for name, m := range cfg.Machines {
			fmt.Printf("%-15s %-25s %-10s %-20s\n", name, m.Host, m.User, m.Workdir)
		}
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show machine details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])

		if flagJSON {
			outputJSON(m)
			return
		}

		fmt.Printf("Machine: %s\n", args[0])
		fmt.Printf("  host:      %s\n", m.Host)
		fmt.Printf("  user:      %s\n", m.User)
		fmt.Printf("  port:      %d\n", m.Port)
		fmt.Printf("  workdir:   %s\n", m.Workdir)
		fmt.Printf("  transport: %s\n", m.Transport)
		fmt.Printf("  auth:      %s\n", m.Auth)
		fmt.Printf("  multiplex: %v\n", m.Multiplex)
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a registered machine",
	Args:  cobra.ExactArgs(1),
	Aliases: []string{"rm"},
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := loadConfig()

		if err := cfg.RemoveMachine(name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		sshconfig.RemoveBlock(name)

		fmt.Printf("Machine %q removed.\n", name)
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor [name]",
	Short: "Diagnose machine connectivity and runtime",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			runLocalDoctor()
			return
		}

		cfg := loadConfig()
		m := getMachine(cfg, args[0])
		result := doctor.Run(m, args[0])

		if flagJSON {
			outputJSON(result)
			return
		}

		result.PrintText()
	},
}

func runLocalDoctor() {
	fmt.Println("Runbox Local Environment Check")
	fmt.Println()

	checks := []struct {
		name string
		fn   func() (bool, string)
	}{
		{"ssh", func() (bool, string) { return commandExists("ssh"), "" }},
		{"ssh-agent", checkSSHAgent},
		{"ssh-add", func() (bool, string) { return commandExists("ssh-add"), "" }},
		{"rsync", func() (bool, string) { return commandExists("rsync"), "" }},
		{"tmux", func() (bool, string) { return commandExists("tmux"), "" }},
		{"tailscale", func() (bool, string) { return commandExists("tailscale"), "" }},
	}

	allOK := true
	for _, c := range checks {
		ok, detail := c.fn()
		status := "OK"
		if !ok {
			status = "missing"
			allOK = false
		}
		line := fmt.Sprintf("  %-15s %s", c.name, status)
		if detail != "" {
			line += " (" + detail + ")"
		}
		fmt.Println(line)
	}

	cfgPath := config.Path()
	fmt.Printf("  %-15s %s\n", "config", cfgPath)
	fmt.Println()

	if allOK {
		fmt.Println("All checks passed. Register a machine: runbox init <name>")
	} else {
		fmt.Println("Some dependencies are missing. Install them before continuing.")
	}
}

func checkSSHAgent() (bool, string) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return false, "SSH_AUTH_SOCK not set"
	}
	if _, err := os.Stat(sock); err != nil {
		return false, "socket not found: " + sock
	}
	return true, sock
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

var fixCmd = &cobra.Command{
	Use:   "fix <name>",
	Short: "Fix connectivity issues for a machine",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		cfg := loadConfig()
		m := getMachine(cfg, name)

		if flagAll {
			runFixSSHConfig(name, m)
			runFixMultiplex()
			runFixAuth(name, m)
			runFixRuntime(name, m)
			fmt.Println()
			fmt.Println("All fixes applied. Run: runbox doctor", name)
			return
		}

		any := false
		if flagSSHCfg {
			runFixSSHConfig(name, m)
			any = true
		}
		if flagMultiplex {
			runFixMultiplex()
			any = true
		}
		if flagAuth {
			runFixAuth(name, m)
			any = true
		}
		if flagRuntime {
			runFixRuntime(name, m)
			any = true
		}

		if !any {
			fmt.Println("Specify a fix to apply:")
			fmt.Println("  --auth          Configure passwordless SSH")
			fmt.Println("  --ssh-config    Generate SSH config entry")
			fmt.Println("  --multiplex     Enable SSH multiplexing")
			fmt.Println("  --remote-runtime  Create remote dirs and check deps")
			fmt.Println("  --all           Apply all recommended fixes")
		}
	},
}

func runFixSSHConfig(name string, m *config.Machine) {
	fmt.Println("Fixing SSH config...")

	keyPath := ""
	if _, err := os.Stat(sshconfig.KeyPath()); err == nil {
		keyPath = sshconfig.KeyPath()
	}

	lines := sshconfig.GenerateBlock(name, m.Host, m.User, m.Port, keyPath, m.Multiplex)
	if err := sshconfig.WriteBlock(name, lines, flagForce); err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}

	fmt.Printf("  wrote SSH config block for %s\n", name)
	fmt.Printf("  %s\n", sshconfig.ConfigPath())
}

func runFixMultiplex() {
	fmt.Println("Enabling SSH multiplexing...")
	if err := sshconfig.EnsureControlSocketDir(); err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
		return
	}
	fmt.Printf("  created %s (chmod 700)\n", sshconfig.ControlSocketDir())
}

func runFixAuth(name string, m *config.Machine) {
	if err := auth.SetupAuth(m, name, flagNoPass); err != nil {
		fmt.Fprintf(os.Stderr, "  Auth error: %v\n", err)
		return
	}
}

func runFixRuntime(name string, m *config.Machine) {
	fmt.Println("Setting up remote runtime...")

	if !ssh.CanConnectBatchMode(m, name) {
		fmt.Fprintf(os.Stderr, "  Cannot connect to %s with public-key auth. Run: runbox fix %s --auth\n", name, name)
		return
	}

	commands := []struct {
		desc string
		cmd  string
	}{
		{"Creating ~/.runbox", "mkdir -p ~/.runbox/logs ~/.runbox/sessions ~/.runbox/artifacts"},
	}

	if m.Workdir != "" {
		commands = append(commands, struct {
			desc string
			cmd  string
		}{"Creating workdir", "mkdir -p " + shellQuote(m.Workdir)})
	}

	for _, c := range commands {
		result, err := ssh.Exec(context.Background(), m, name, c.cmd, &ssh.Opts{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  %s: error: %v\n", c.desc, err)
			continue
		}
		if result.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "  %s: exit %d: %s\n", c.desc, result.ExitCode, result.Stderr)
			continue
		}
		fmt.Printf("  %s: OK\n", c.desc)
	}

	checkRemote := func(cmd string) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", name, "command -v "+cmd)
		return c.Run() == nil
	}

	if !checkRemote("tmux") {
		fmt.Printf("  tmux: missing (install manually or via package manager)\n")
	}
	if !checkRemote("rsync") {
		fmt.Printf("  rsync: missing (install manually or via package manager)\n")
	}
}

var execCmd = &cobra.Command{
	Use:   "exec <machine> <command>",
	Short: "Execute a command on a remote machine",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])
		command := args[1]

		if cfg.Policies != nil {
			engine := policy.NewEngine(cfg.Policies.RequireApproval, cfg.Policies.Deny)
			decision := engine.Check(command)
			if !decision.Allowed {
				fmt.Fprintf(os.Stderr, "\nCommand denied:\n  %s\nReason:\n  Matches deny policy: %s\n", command, decision.Pattern)
				os.Exit(1)
			}
			if decision.Requires && !flagApprove {
				if !isInteractive() {
					fmt.Fprintf(os.Stderr, "\nApproval required:\n  %s\nReason:\n  Matches policy: %s\nUse --approve to bypass.\n", command, decision.Pattern)
					os.Exit(1)
				}
				fmt.Printf("\nApproval required:\n  %s\nReason:\n  Matches policy: %s\nApprove? [y/N] ", command, decision.Pattern)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" {
					fmt.Println("Cancelled.")
					os.Exit(1)
				}
			}
		}

		opts := &ssh.Opts{
			Workdir: flagWorkdir,
		}

		result, err := ssh.ExecStreaming(context.Background(), m, args[0], command, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if flagJSON {
			outputJSON(map[string]interface{}{
				"machine":    args[0],
				"command":    command,
				"exitCode":   result.ExitCode,
				"durationMs": result.DurationMs,
			})
			return
		}

		os.Exit(result.ExitCode)
	},
}

var shellCmd = &cobra.Command{
	Use:   "shell <machine>",
	Short: "Open an interactive shell on a remote machine",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])

		wd := m.Workdir
		if flagWorkdir != "" {
			wd = flagWorkdir
		}

		if err := ssh.InteractiveShell(m, args[0], wd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage persistent tmux sessions",
}

var sessionCreateCmd = &cobra.Command{
	Use:   "create <machine> <session-name>",
	Short: "Create a new tmux session",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])
		sessionName := "runbox-" + args[1]

		cmdStr := fmt.Sprintf("tmux new-session -d -s %s", shellQuote(sessionName))
		if m.Workdir != "" {
			cmdStr = fmt.Sprintf("tmux new-session -d -s %s -c %s", shellQuote(sessionName), shellQuote(m.Workdir))
		}

		result, err := ssh.Exec(context.Background(), m, args[0], cmdStr, &ssh.Opts{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if result.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Stderr)
			os.Exit(result.ExitCode)
		}

		fmt.Printf("Session %q created on %s.\n", args[1], args[0])
	},
}

var sessionAttachCmd = &cobra.Command{
	Use:   "attach <machine> <session-name>",
	Short: "Attach to a tmux session",
	Args:  cobra.ExactArgs(2),
	Aliases: []string{"at"},
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		_ = getMachine(cfg, args[0])
		sessionName := "runbox-" + args[1]

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		sshCmd := exec.CommandContext(ctx, "ssh", "-t", args[0],
			fmt.Sprintf("tmux attach-session -t %s || tmux new-session -A -s %s",
				shellQuote(sessionName), shellQuote(sessionName)))
		sshCmd.Stdin = os.Stdin
		sshCmd.Stdout = os.Stdout
		sshCmd.Stderr = os.Stderr
		sshCmd.Run()
	},
}

var sessionListCmd = &cobra.Command{
	Use:   "list <machine>",
	Short: "List tmux sessions on a machine",
	Args:  cobra.ExactArgs(1),
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])

		result, err := ssh.Exec(context.Background(), m, args[0],
			"tmux list-sessions -F '#{session_name}' 2>/dev/null | grep '^runbox-' || echo 'none'",
			&ssh.Opts{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		sessions := strings.Split(result.Stdout, "\n")
		if len(sessions) == 0 || sessions[0] == "none" {
			fmt.Println("No runbox sessions.")
			return
		}

		if flagJSON {
			outputJSON(sessions)
			return
		}

		for _, s := range sessions {
			s = strings.TrimSpace(s)
			if s != "" && s != "none" {
				fmt.Println(strings.TrimPrefix(s, "runbox-"))
			}
		}
	},
}

var sessionKillCmd = &cobra.Command{
	Use:   "kill <machine> <session-name>",
	Short: "Kill a tmux session",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])
		sessionName := "runbox-" + args[1]

		result, err := ssh.Exec(context.Background(), m, args[0],
			fmt.Sprintf("tmux kill-session -t %s", shellQuote(sessionName)),
			&ssh.Opts{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if result.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Stderr)
			os.Exit(result.ExitCode)
		}

		fmt.Printf("Session %q killed.\n", args[1])
	},
}

var sessionExecCmd = &cobra.Command{
	Use:   "exec <machine> <session-name> <command>",
	Short: "Send a command to a tmux session",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])
		sessionName := "runbox-" + args[1]
		command := args[2]

		cmdStr := fmt.Sprintf("tmux send-keys -t %s %s Enter",
			shellQuote(sessionName), shellQuote(command))

		result, err := ssh.Exec(context.Background(), m, args[0], cmdStr, &ssh.Opts{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if result.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "Error: %s\n", result.Stderr)
			os.Exit(result.ExitCode)
		}

		if !flagQuiet {
			fmt.Printf("Sent to %q: %s\n", args[1], command)
		}
	},
}

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage and run agent profiles",
}

var agentAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add an agent profile",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		command, _ := cmd.Flags().GetString("command")
		if command == "" {
			fmt.Fprintf(os.Stderr, "Error: --command is required\n")
			os.Exit(1)
		}

		cwd, _ := cmd.Flags().GetString("cwd")

		a := &config.Agent{
			Command: command,
			Cwd:     cwd,
		}

		if err := cfg.AddAgent(args[0], a); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Agent %q added (command: %s).\n", args[0], command)
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agent profiles",
	Aliases: []string{"ls"},
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()

		if len(cfg.Agents) == 0 {
			fmt.Println("No agent profiles. Run: runbox agent add <name> --command <cmd>")
			return
		}

		if flagJSON {
			outputJSON(cfg.Agents)
			return
		}

		fmt.Printf("%-15s %-25s %s\n", "NAME", "COMMAND", "CWD")
		for name, a := range cfg.Agents {
			fmt.Printf("%-15s %-25s %s\n", name, a.Command, a.Cwd)
		}
	},
}

var agentRunCmd = &cobra.Command{
	Use:   "run <machine> <agent>",
	Short: "Run an agent on a remote machine",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])

		agentDef, ok := cfg.Agents[args[1]]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: agent %q not found\n", args[1])
			os.Exit(1)
		}

		sessionName := args[1]
		if flagSession != "" {
			sessionName = flagSession
		}
		fullSession := "runbox-" + sessionName

		cwd := agentDef.Cwd
		if cwd == "" {
			cwd = m.Workdir
		}

		createCmd := fmt.Sprintf("tmux new-session -d -s %s", shellQuote(fullSession))
		if cwd != "" {
			createCmd = fmt.Sprintf("tmux new-session -d -s %s -c %s", shellQuote(fullSession), shellQuote(cwd))
		}

		ssh.Exec(context.Background(), m, args[0], createCmd, &ssh.Opts{})

		agentCmd := agentDef.Command
		if len(agentDef.DefaultArgs) > 0 {
			agentCmd += " " + strings.Join(agentDef.DefaultArgs, " ")
		}

		sendCmd := fmt.Sprintf("tmux send-keys -t %s %s Enter", shellQuote(fullSession), shellQuote(agentCmd))
		result, err := ssh.Exec(context.Background(), m, args[0], sendCmd, &ssh.Opts{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if flagDetached {
			fmt.Printf("Agent %q running in session %q on %s.\n", args[1], sessionName, args[0])
			fmt.Printf("Attach: runbox session attach %s %s\n", args[0], sessionName)
			fmt.Printf("Logs:   runbox logs %s %s\n", args[0], sessionName)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		attachCmd := exec.CommandContext(ctx, "ssh", "-t", args[0],
			fmt.Sprintf("tmux attach-session -t %s", shellQuote(fullSession)))
		attachCmd.Stdin = os.Stdin
		attachCmd.Stdout = os.Stdout
		attachCmd.Stderr = os.Stderr
		attachCmd.Run()
		_ = result
	},
}

var syncCmd = &cobra.Command{
	Use:   "sync <machine> <local-path> <remote-path>",
	Short: "Sync local files to a remote machine",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		_ = getMachine(cfg, args[0])
		localPath := args[1]
		remotePath := args[2]

		rsyncArgs := []string{"-avz", "--progress"}
		if flagDelete {
			rsyncArgs = append(rsyncArgs, "--delete")
		}

		excludes, _ := cmd.Flags().GetStringArray("exclude")
		for _, ex := range excludes {
			rsyncArgs = append(rsyncArgs, "--exclude", ex)
		}

		if flagVerbose {
			rsyncArgs = append(rsyncArgs, "-v")
		}

		src := localPath
		info, err := os.Stat(localPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if info.IsDir() {
			src = localPath + "/"
		}

		rsyncArgs = append(rsyncArgs, src, fmt.Sprintf("%s:%s", args[0], remotePath))

		rsyncCmd := exec.Command("rsync", rsyncArgs...)
		rsyncCmd.Stdin = os.Stdin
		rsyncCmd.Stdout = os.Stdout
		rsyncCmd.Stderr = os.Stderr

		if err := rsyncCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: rsync failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var pullCmd = &cobra.Command{
	Use:   "pull <machine> <remote-path> <local-path>",
	Short: "Pull files from a remote machine",
	Args:  cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		_ = getMachine(cfg, args[0])
		remotePath := args[1]
		localPath := args[2]

		rsyncArgs := []string{"-avz", "--progress"}
		rsyncArgs = append(rsyncArgs, fmt.Sprintf("%s:%s", args[0], remotePath), localPath)

		rsyncCmd := exec.Command("rsync", rsyncArgs...)
		rsyncCmd.Stdin = os.Stdin
		rsyncCmd.Stdout = os.Stdout
		rsyncCmd.Stderr = os.Stderr

		if err := rsyncCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: rsync failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs <machine> [session-name]",
	Short: "View logs from a remote machine",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])

		var cmdStr string
		if len(args) > 1 {
			sessionName := "runbox-" + args[1]
			if flagTail {
				cmdStr = fmt.Sprintf("tmux capture-pane -t %s -p -S -100", shellQuote(sessionName))
			} else {
				cmdStr = fmt.Sprintf("tmux capture-pane -t %s -p -S -50", shellQuote(sessionName))
			}
		} else {
			cmdStr = "ls -lt ~/.runbox/logs/ 2>/dev/null | head -20"
			if flagTail {
				cmdStr = "find ~/.runbox/logs/ -name '*.log' -exec tail -20 {} + 2>/dev/null"
			}
		}

		result, err := ssh.Exec(context.Background(), m, args[0], cmdStr, &ssh.Opts{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(result.Stdout)
		if result.Stderr != "" {
			fmt.Fprintf(os.Stderr, "%s\n", result.Stderr)
		}
	},
}

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap <machine>",
	Short: "Bootstrap the remote runtime environment",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		cfg := loadConfig()
		m := getMachine(cfg, args[0])

		fmt.Printf("Bootstrapping %s...\n\n", args[0])

		if !ssh.CanConnectBatchMode(m, args[0]) {
			fmt.Fprintf(os.Stderr, "Cannot connect with public-key auth. Run: runbox fix %s --auth\n", args[0])
			os.Exit(1)
		}

		steps := []struct {
			desc string
			cmd  string
		}{
			{"Creating ~/.runbox", "mkdir -p ~/.runbox/logs ~/.runbox/sessions ~/.runbox/artifacts"},
		}

		if m.Workdir != "" {
			steps = append(steps, struct {
				desc string
				cmd  string
			}{"Creating workdir", "mkdir -p " + shellQuote(m.Workdir)})
		}

		steps = append(steps, []struct {
			desc string
			cmd  string
		}{
			{"Checking tmux", "command -v tmux && echo ok || echo missing"},
			{"Checking rsync", "command -v rsync && echo ok || echo missing"},
			{"Checking shell", "echo shell-ok"},
		}...)

		for _, s := range steps {
			result, err := ssh.Exec(context.Background(), m, args[0], s.cmd, &ssh.Opts{})
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %-30s error: %v\n", s.desc, err)
				continue
			}
			status := "OK"
			if strings.Contains(result.Stdout, "missing") {
				status = "MISSING"
			}
			fmt.Printf("  %-30s %s\n", s.desc, status)
			if flagVerbose && result.Stdout != "" {
				fmt.Printf("    %s\n", result.Stdout)
			}
		}

		fmt.Println()
		fmt.Printf("Bootstrap complete. Run: runbox doctor %s\n", args[0])
	},
}

func init() {
	initCmd.Flags().StringVar(&flagHost, "host", "", "hostname or IP")
	initCmd.Flags().StringVar(&flagUser, "user", "", "remote user")
	initCmd.Flags().StringVar(&flagWorkdir, "workdir", "", "default working directory")
	initCmd.Flags().IntVar(&flagPort, "port", 22, "SSH port")

	fixCmd.Flags().BoolVar(&flagAll, "all", false, "apply all recommended fixes")
	fixCmd.Flags().BoolVar(&flagAuth, "auth", false, "configure passwordless SSH")
	fixCmd.Flags().BoolVar(&flagSSHCfg, "ssh-config", false, "generate SSH config entry")
	fixCmd.Flags().BoolVar(&flagMultiplex, "multiplex", false, "enable SSH multiplexing")
	fixCmd.Flags().BoolVar(&flagRuntime, "remote-runtime", false, "create remote runtime dirs")
	fixCmd.Flags().BoolVar(&flagNoPass, "no-passphrase", false, "generate key without passphrase")
	fixCmd.Flags().BoolVar(&flagForce, "force", false, "overwrite existing config")

	execCmd.Flags().StringVar(&flagWorkdir, "workdir", "", "override working directory")
	execCmd.Flags().BoolVar(&flagApprove, "approve", false, "approve policy-restricted commands")
	shellCmd.Flags().StringVar(&flagWorkdir, "workdir", "", "override working directory")

	sessionCmd.AddCommand(sessionCreateCmd)
	sessionCmd.AddCommand(sessionAttachCmd)
	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionKillCmd)
	sessionCmd.AddCommand(sessionExecCmd)

	agentAddCmd.Flags().String("command", "", "agent command")
	agentAddCmd.Flags().String("cwd", "", "working directory for agent")
	agentCmd.AddCommand(agentAddCmd)
	agentCmd.AddCommand(agentListCmd)
	agentRunCmd.Flags().BoolVar(&flagDetached, "detached", false, "run in background")
	agentRunCmd.Flags().StringVar(&flagSession, "session", "", "session name")
	agentCmd.AddCommand(agentRunCmd)

	syncCmd.Flags().BoolVar(&flagDelete, "delete", false, "delete remote files not present locally")
	syncCmd.Flags().StringArray("exclude", nil, "exclude patterns")

	logsCmd.Flags().BoolVar(&flagTail, "tail", false, "tail recent output")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
