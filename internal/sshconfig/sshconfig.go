package sshconfig

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	beginMarkerFmt = "# BEGIN RUNBOX %s"
	endMarkerFmt   = "# END RUNBOX %s"
)

func SSHDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh")
}

func ConfigPath() string {
	return filepath.Join(SSHDir(), "config")
}

func ControlSocketDir() string {
	return filepath.Join(SSHDir(), "runbox")
}

func KeyPath() string {
	return filepath.Join(SSHDir(), "runbox_ed25519")
}

func KeyPubPath() string {
	return KeyPath() + ".pub"
}

func EnsureSSHDir() error {
	dir := SSHDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.Chmod(dir, 0700)
}

func EnsureControlSocketDir() error {
	dir := ControlSocketDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.Chmod(dir, 0700)
}

func GenerateBlock(name string, host string, user string, port int, keyPath string, multiplex bool) []string {
	var lines []string
	lines = append(lines, fmt.Sprintf(beginMarkerFmt, name))
	lines = append(lines, fmt.Sprintf("Host %s", name))
	lines = append(lines, fmt.Sprintf("  HostName %s", host))
	lines = append(lines, fmt.Sprintf("  User %s", user))
	lines = append(lines, fmt.Sprintf("  Port %d", port))
	if keyPath != "" {
		lines = append(lines, fmt.Sprintf("  IdentityFile %s", keyPath))
		lines = append(lines, "  AddKeysToAgent yes")
	}
	if multiplex {
		lines = append(lines, "  ControlMaster auto")
		lines = append(lines, fmt.Sprintf("  ControlPath %s/cm-%%r@%%h:%%p", ControlSocketDir()))
		lines = append(lines, "  ControlPersist 30m")
	}
	lines = append(lines, "  ServerAliveInterval 30")
	lines = append(lines, "  ServerAliveCountMax 3")
	lines = append(lines, fmt.Sprintf(endMarkerFmt, name))
	return lines
}

func HasBlock(name string) (bool, error) {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	begin := fmt.Sprintf(beginMarkerFmt, name)
	end := fmt.Sprintf(endMarkerFmt, name)
	content := string(data)
	return strings.Contains(content, begin) && strings.Contains(content, end), nil
}

func WriteBlock(name string, lines []string, force bool) error {
	if err := EnsureSSHDir(); err != nil {
		return err
	}
	path := ConfigPath()

	var existing []string
	data, err := os.ReadFile(path)
	if err == nil {
		existing = strings.Split(string(data), "\n")
	} else if !os.IsNotExist(err) {
		return err
	}

	begin := fmt.Sprintf(beginMarkerFmt, name)
	end := fmt.Sprintf(endMarkerFmt, name)

	hasExisting := false
	for _, line := range existing {
		if strings.TrimSpace(line) == begin {
			hasExisting = true
			break
		}
	}

	if hasExisting && !force {
		return fmt.Errorf("SSH config block for %q already exists; use --force to overwrite", name)
	}

	var result []string
	if hasExisting {
		inBlock := false
		for _, line := range existing {
			if strings.TrimSpace(line) == begin {
				inBlock = true
				continue
			}
			if strings.TrimSpace(line) == end {
				inBlock = false
				continue
			}
			if !inBlock {
				result = append(result, line)
			}
		}
	} else {
		result = existing
	}

	if len(result) > 0 && result[len(result)-1] != "" {
		result = append(result, "")
	}
	result = append(result, lines...)
	result = append(result, "")

	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644)
}

func RemoveBlock(name string) error {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	begin := fmt.Sprintf(beginMarkerFmt, name)
	end := fmt.Sprintf(endMarkerFmt, name)

	var result []string
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == begin {
			inBlock = true
			continue
		}
		if strings.TrimSpace(line) == end {
			inBlock = false
			continue
		}
		if !inBlock {
			result = append(result, line)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(result, "\n")), 0644)
}

func ParseExistingHosts() (map[string]bool, error) {
	hosts := make(map[string]bool)
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return hosts, nil
		}
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Host ") {
			host := strings.TrimPrefix(line, "Host ")
			hosts[strings.TrimSpace(host)] = true
		}
	}
	return hosts, nil
}
