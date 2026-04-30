package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	VaultPath string                   `yaml:"vault_path" json:"vault_path"`
	Projects  map[string]ProjectConfig `yaml:"projects" json:"projects"`
	Warnings  []string                 `yaml:"-" json:"warnings,omitempty"`
}

type ProjectConfig struct {
	Folders []string `yaml:"folders" json:"folders"`
	Aliases []string `yaml:"aliases" json:"aliases"`
}

func DefaultGlobalPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".config", "zettelbrief", "config.yaml")
	}
	return filepath.Join(home, ".config", "zettelbrief", "config.yaml")
}

func LoadGlobal(path string) (*Config, error) {
	if path == "" {
		path = DefaultGlobalPath()
	}
	cfg, err := loadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return empty(), nil
	}
	if err != nil {
		return nil, err
	}
	cfg.VaultPath = expandHome(cfg.VaultPath)
	return cfg, nil
}

func WriteGlobal(path string, cfg *Config, force bool) error {
	if path == "" {
		path = DefaultGlobalPath()
	}
	if cfg == nil {
		return errors.New("config is nil")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config %s already exists (use --force to overwrite)", path)
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	_ = os.Chmod(filepath.Dir(path), 0o700)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func DiscoverProjectConfig(start string) (string, bool, error) {
	if start == "" {
		var err error
		start, err = os.Getwd()
		if err != nil {
			return "", false, err
		}
	}
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", false, err
	}
	info, err := os.Stat(abs)
	if err == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	for {
		candidate := filepath.Join(abs, ".zettelbrief", "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true, nil
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false, nil
		}
		abs = parent
	}
}

func LoadProject(path string) (*Config, error) {
	return loadFile(path)
}

func Load() (*Config, error) {
	global, err := LoadGlobal("")
	if err != nil {
		return nil, err
	}
	projectPath, ok, err := DiscoverProjectConfig("")
	if err != nil {
		return nil, err
	}
	if !ok {
		return global, nil
	}
	project, err := LoadProject(projectPath)
	if err != nil {
		return nil, err
	}
	return Merge(global, project), nil
}

func Merge(global, project *Config) *Config {
	out := clone(global)
	if project == nil {
		return out
	}
	if strings.TrimSpace(project.VaultPath) != "" {
		out.Warnings = append(out.Warnings, "vault_path can only be set in global config; ignoring project override")
	}
	if out.Projects == nil {
		out.Projects = map[string]ProjectConfig{}
	}
	for name, pc := range project.Projects {
		out.Projects[name] = pc
	}
	return out
}

func (c *Config) ValidateForScan(project string) error {
	if c == nil {
		return errors.New("config is nil")
	}
	if strings.TrimSpace(c.VaultPath) == "" {
		return errors.New("vault_path is required for scan operations; set vault_path in ~/.config/zettelbrief/config.yaml")
	}
	vault, err := filepath.Abs(expandHome(c.VaultPath))
	if err != nil {
		return fmt.Errorf("resolving vault_path %q: %w", c.VaultPath, err)
	}
	info, err := os.Stat(vault)
	if err != nil {
		return fmt.Errorf("vault_path %q does not exist: %w", vault, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vault_path %q is not a directory", vault)
	}
	c.VaultPath = vault
	if project != "" {
		if _, ok := c.Projects[project]; !ok {
			return fmt.Errorf("unknown project %q (configured: %s)", project, strings.Join(c.SortedProjectNames(), ", "))
		}
		return c.validateProject(project)
	}
	for _, name := range c.SortedProjectNames() {
		if err := c.validateProject(name); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validateProject(name string) error {
	pc := c.Projects[name]
	for _, folder := range pc.Folders {
		if err := ValidateVaultRelativeFolder(c.VaultPath, folder); err != nil {
			return fmt.Errorf("project %q folder %q: %w", name, folder, err)
		}
	}
	return nil
}

func ValidateVaultRelativeFolder(vaultPath, folder string) error {
	if strings.TrimSpace(folder) == "" {
		return errors.New("project folders must be non-empty")
	}
	if filepath.IsAbs(folder) {
		return errors.New("project folders must be vault-relative")
	}
	clean := filepath.Clean(filepath.FromSlash(folder))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("project folder escapes the vault root")
	}
	vaultAbs, err := filepath.Abs(vaultPath)
	if err != nil {
		return err
	}
	candidate := filepath.Join(vaultAbs, clean)
	resolvedVault, err := filepath.EvalSymlinks(vaultAbs)
	if err != nil {
		return err
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("cannot resolve folder: %w", err)
	}
	inside, err := IsInside(resolvedVault, resolvedCandidate)
	if err != nil {
		return err
	}
	if !inside {
		return errors.New("project folder resolves outside the vault root")
	}
	info, err := os.Stat(resolvedCandidate)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("project folder is not a directory")
	}
	return nil
}

func IsInside(root, path string) (bool, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}

func (c *Config) SortedProjectNames() []string {
	names := make([]string, 0, len(c.Projects))
	for name := range c.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := empty()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Projects == nil {
		cfg.Projects = map[string]ProjectConfig{}
	}
	return cfg, nil
}

func empty() *Config {
	return &Config{Projects: map[string]ProjectConfig{}}
}

func clone(in *Config) *Config {
	if in == nil {
		return empty()
	}
	out := &Config{VaultPath: in.VaultPath, Projects: map[string]ProjectConfig{}, Warnings: append([]string(nil), in.Warnings...)}
	for k, v := range in.Projects {
		out.Projects[k] = ProjectConfig{Folders: append([]string(nil), v.Folders...), Aliases: append([]string(nil), v.Aliases...)}
	}
	return out
}

func expandHome(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
