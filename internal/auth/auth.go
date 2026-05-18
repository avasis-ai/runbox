package auth

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/avasis-ai/runbox/internal/config"
	"github.com/avasis-ai/runbox/internal/ssh"
	"github.com/avasis-ai/runbox/internal/sshconfig"
)

func GenerateKey(noPassphrase bool) error {
	if err := sshconfig.EnsureSSHDir(); err != nil {
		return fmt.Errorf("ensuring .ssh dir: %w", err)
	}

	keyPath := sshconfig.KeyPath()
	if _, err := os.Stat(keyPath); err == nil {
		fmt.Printf("  Key already exists: %s\n", keyPath)
		return nil
	}

	args := []string{
		"-t", "ed25519",
		"-f", keyPath,
		"-C", "runbox@" + hostname(),
	}

	if noPassphrase {
		args = append(args, "-N", "")
		fmt.Printf("  Generated key without passphrase: %s\n", keyPath)
	} else {
		fmt.Printf("  Generating ed25519 key with passphrase: %s\n", keyPath)
		fmt.Printf("  You will be prompted for a passphrase.\n")
	}

	cmd := exec.Command("ssh-keygen", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-keygen failed: %w", err)
	}

	if err := os.Chmod(keyPath, 0600); err != nil {
		return fmt.Errorf("chmod key: %w", err)
	}

	fmt.Printf("  wrote %s (chmod 600)\n", keyPath)
	return nil
}

func AddToAgent() error {
	if !commandExists("ssh-add") {
		return fmt.Errorf("ssh-add not found")
	}

	keyPath := sshconfig.KeyPath()
	if _, err := os.Stat(keyPath); err != nil {
		return fmt.Errorf("key not found: %s", keyPath)
	}

	cmd := exec.Command("ssh-add", keyPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-add failed: %w", err)
	}

	fmt.Printf("  added %s to ssh-agent\n", keyPath)
	return nil
}

func InstallPublicKey(m *config.Machine, name string) error {
	pubKeyPath := sshconfig.KeyPubPath()
	if _, err := os.Stat(pubKeyPath); err != nil {
		return fmt.Errorf("public key not found: %s", pubKeyPath)
	}

	if commandExists("ssh-copy-id") {
		fmt.Printf("  Installing public key on %s...\n", m.Host)
		fmt.Printf("  You may be prompted for the remote password.\n")
		return ssh.CopyID(m, name, pubKeyPath)
	}

	pubKey, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("reading public key: %w", err)
	}

	fmt.Printf("  ssh-copy-id not found. Manual install:\n")
	fmt.Printf("  Run on the remote machine:\n")
	fmt.Printf("    echo '%s' >> ~/.ssh/authorized_keys\n", strings.TrimSpace(string(pubKey)))
	return fmt.Errorf("manual key install required")
}

func SetupAuth(m *config.Machine, name string, noPassphrase bool) error {
	fmt.Println("Setting up SSH authentication...")

	fmt.Println("\n[1/3] Generating SSH key...")
	if err := GenerateKey(noPassphrase); err != nil {
		return err
	}

	fmt.Println("\n[2/3] Adding key to ssh-agent...")
	if err := AddToAgent(); err != nil {
		fmt.Printf("  Warning: %v\n", err)
		fmt.Printf("  You can add manually: ssh-add %s\n", sshconfig.KeyPath())
	}

	fmt.Println("\n[3/3] Installing public key on remote...")
	if err := InstallPublicKey(m, name); err != nil {
		return err
	}

	fmt.Println("\nAuthentication setup complete.")
	return nil
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
