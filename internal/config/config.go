package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version  int                 `yaml:"version"`
	Machines map[string]*Machine `yaml:"machines"`
	Agents   map[string]*Agent   `yaml:"agents,omitempty"`
	Policies *Policies           `yaml:"policies,omitempty"`
}

type Machine struct {
	Host      string `yaml:"host"`
	User      string `yaml:"user"`
	Port      int    `yaml:"port"`
	Workdir   string `yaml:"workdir"`
	Transport string `yaml:"transport"`
	Auth      string `yaml:"auth"`
	Multiplex bool   `yaml:"multiplex"`
}

type Agent struct {
	Command     string            `yaml:"command"`
	DefaultArgs []string          `yaml:"default_args,omitempty"`
	Env         map[string]string `yaml:"env,omitempty"`
	Cwd         string            `yaml:"cwd,omitempty"`
}

type Policies struct {
	RequireApproval []string `yaml:"require_approval,omitempty"`
	Deny            []string `yaml:"deny,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Version:  1,
		Machines: make(map[string]*Machine),
		Agents:   make(map[string]*Agent),
		Policies: &Policies{
			RequireApproval: []string{
				"sudo *",
				"rm -rf *",
				"docker system prune*",
				"git push*",
			},
			Deny: []string{
				"rm -rf /",
				":(){ :|:& };:",
			},
		},
	}
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".runbox")
}

func Path() string {
	return filepath.Join(Dir(), "config.yaml")
}

func Exists() bool {
	_, err := os.Stat(Path())
	return err == nil
}

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Machines == nil {
		cfg.Machines = make(map[string]*Machine)
	}
	if cfg.Agents == nil {
		cfg.Agents = make(map[string]*Agent)
	}
	return &cfg, nil
}

func (c *Config) Save() error {
	dir := Dir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(Path(), data, 0644)
}

func (c *Config) AddMachine(name string, m *Machine) error {
	if _, exists := c.Machines[name]; exists {
		return fmt.Errorf("machine %q already exists; use runbox remove %s first", name, name)
	}
	if m.Port == 0 {
		m.Port = 22
	}
	if m.Transport == "" {
		m.Transport = "ssh"
	}
	if m.Auth == "" {
		m.Auth = "ssh-agent"
	}
	c.Machines[name] = m
	return c.Save()
}

func (c *Config) RemoveMachine(name string) error {
	if _, exists := c.Machines[name]; !exists {
		return fmt.Errorf("machine %q not found", name)
	}
	delete(c.Machines, name)
	return c.Save()
}

func (c *Config) GetMachine(name string) (*Machine, error) {
	m, ok := c.Machines[name]
	if !ok {
		return nil, fmt.Errorf("machine %q not found; run: runbox init %s", name, name)
	}
	return m, nil
}

func (c *Config) AddAgent(name string, a *Agent) error {
	if _, exists := c.Agents[name]; exists {
		return fmt.Errorf("agent %q already exists", name)
	}
	if a.Command == "" {
		return fmt.Errorf("agent command is required")
	}
	c.Agents[name] = a
	return c.Save()
}
