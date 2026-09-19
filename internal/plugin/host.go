// Package plugin implements the Plugin Host (S12).
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	log "telos/internal/logger"
	"telos/internal/model"
	"telos/internal/plugin/sandbox"

	"gopkg.in/yaml.v3"
)

// SupportedInterfaceVersion specifies the plugin interface version supported by this host.
const SupportedInterfaceVersion = 1
const defaultOutputLimit int64 = 4 << 20

var semver = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)

// Options configures a Host. A zero timeout uses five minutes, matching the
// discovery default. OutputLimit bounds data returned by an untrusted plugin.
type Options struct {
	PluginDirs  []string
	Timeout     time.Duration
	OutputLimit int64
	Logger      log.Logger
	// Sandbox provides OS-level plugin isolation when set. If nil, plugins
	// run without sandbox enforcement (direct subprocess execution).
	Sandbox sandbox.Sandbox
}

// Plugin is a validated, registered plugin and its package location.
type Plugin struct {
	Manifest   model.PluginManifest
	ID         string
	Dir        string
	Executable string
}

// Failure describes a plugin that was skipped or an invocation that failed.
type Failure struct{ PluginID, Path, Reason string }

type manifestFile struct {
	ID               string           `yaml:"id"`
	Name             string           `yaml:"name"`
	Version          string           `yaml:"version"`
	InterfaceVersion int              `yaml:"interface_version"`
	Type             string           `yaml:"type"`
	Author           string           `yaml:"author"`
	Description      string           `yaml:"description"`
	Capabilities     capabilitiesFile `yaml:"capabilities"`
	Permissions      permissionsFile  `yaml:"permissions"`
	Executable       string           `yaml:"executable"`
	Signature        string           `yaml:"signature"`
}
type capabilitiesFile struct {
	OSFamilies          []string `yaml:"os_families"`
	PackageManagers     []string `yaml:"package_managers"`
	CloudProviders      []string `yaml:"cloud_providers"`
	DiscoveryCategories []string `yaml:"discovery_categories"`
	StorageProtocols    []string `yaml:"storage_protocols"`
}
type permissionsFile struct {
	FilesystemRead  []string `yaml:"filesystem_read"`
	FilesystemWrite []string `yaml:"filesystem_write"`
	Network         bool     `yaml:"network"`
	Subprocess      []string `yaml:"subprocess"`
}

// Host manages the discovery, loading, and invocation of plugins.
type Host struct {
	opts    Options
	plugins map[string]Plugin
}

// New creates a new Host with the provided options.
func New(opts Options) *Host {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}
	if opts.OutputLimit <= 0 {
		opts.OutputLimit = defaultOutputLimit
	}
	if opts.Logger == nil {
		opts.Logger = log.New(log.InfoLevel, io.Discard)
	}
	return &Host{opts: opts, plugins: make(map[string]Plugin)}
}

// Discover scans configured directories and registers valid packages. A bad
// package is isolated and returned as a failure; other packages continue.
func (h *Host) Discover() []Failure {
	var failures []Failure
	for _, root := range h.opts.PluginDirs {
		entries, err := os.ReadDir(root)
		if err != nil {
			failures = append(failures, Failure{Path: root, Reason: err.Error()})
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			p, err := h.load(filepath.Join(root, entry.Name()))
			if err != nil {
				failures = append(failures, Failure{Path: filepath.Join(root, entry.Name()), Reason: err.Error()})
				continue
			}
			if _, exists := h.plugins[p.ID]; exists {
				failures = append(failures, Failure{PluginID: p.ID, Path: p.Dir, Reason: "duplicate plugin id"})
				continue
			}
			h.plugins[p.ID] = p
			h.opts.Logger.Info("plugin registered: " + p.ID)
		}
	}
	return failures
}

func (h *Host) load(dir string) (Plugin, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest")) // #nosec G304 -- dir is a fixed, non-user-controlled base path from configuration
	if err != nil {
		return Plugin{}, fmt.Errorf("read manifest: %w", err)
	}
	var mf manifestFile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err2 := dec.Decode(&mf); err2 != nil {
		return Plugin{}, fmt.Errorf("parse manifest: %w", err2)
	}
	if mf.ID == "" || !strings.Contains(mf.ID, ".") {
		return Plugin{}, errors.New("id must use reverse-domain form")
	}
	if mf.Name == "" || mf.Author == "" || !semver.MatchString(mf.Version) {
		return Plugin{}, errors.New("name, author, and semantic version are required")
	}
	if mf.InterfaceVersion != SupportedInterfaceVersion {
		return Plugin{}, fmt.Errorf("unsupported interface version %d", mf.InterfaceVersion)
	}
	if !validType(mf.Type) {
		return Plugin{}, fmt.Errorf("unknown plugin type %q", mf.Type)
	}
	if len(mf.Permissions.FilesystemWrite) > 0 && mf.Type != "Restore" && mf.Type != "Storage" {
		return Plugin{}, errors.New("filesystem write permission is not allowed for this plugin type")
	}
	if mf.Type == "ClassificationRule" && (mf.Permissions.Network || len(mf.Permissions.Subprocess) > 0 || len(mf.Permissions.FilesystemRead) > 0 || len(mf.Permissions.FilesystemWrite) > 0) {
		return Plugin{}, errors.New("classification plugins cannot request permissions")
	}
	if mf.Permissions.Network && (mf.Type == "Discovery" || mf.Type == "Capture") {
		return Plugin{}, fmt.Errorf("plugin %s of type %s: network permission is not allowed", mf.ID, mf.Type)
	}
	readPaths, err := sandbox.NormalizeManifestPaths(dir, mf.Permissions.FilesystemRead)
	if err != nil {
		return Plugin{}, fmt.Errorf("invalid filesystem_read paths: %w", err)
	}
	writePaths, err := sandbox.NormalizeManifestPaths(dir, mf.Permissions.FilesystemWrite)
	if err != nil {
		return Plugin{}, fmt.Errorf("invalid filesystem_write paths: %w", err)
	}
	exe, err := executablePath(dir, mf.Executable)
	if err != nil {
		return Plugin{}, err
	}
	return Plugin{ID: mf.ID, Dir: dir, Executable: exe, Manifest: model.PluginManifest{Name: mf.Name, Version: model.Version(mf.Version), Author: mf.Author, Description: mf.Description, InterfaceVersion: model.Version(fmt.Sprint(mf.InterfaceVersion)), Type: mf.Type, Capabilities: flattenCapabilities(&mf.Capabilities), FilesystemRead: readPaths, FilesystemWrite: writePaths, NetworkAllowed: mf.Permissions.Network, Subprocesses: mf.Permissions.Subprocess, Signature: mf.Signature}}, nil
}

func executablePath(dir, declared string) (string, error) {
	if declared != "" {
		if filepath.IsAbs(declared) || filepath.Clean(declared) != declared || strings.HasPrefix(declared, "..") {
			return "", errors.New("executable must be a relative package path")
		}
		p := filepath.Join(dir, declared)
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			return "", fmt.Errorf("resolve executable: %w", err)
		}
		resolved = filepath.Clean(resolved)
		if !sandbox.IsSubpath(dir, resolved) {
			return "", errors.New("executable escapes plugin directory")
		}
		if st, e := os.Stat(resolved); e != nil || st.IsDir() {
			return "", errors.New("declared executable not found or is a directory")
		}
		return resolved, nil
	}
	for _, name := range []string{"plugin", "plugin.exe", "executable", "executable.exe"} {
		p := filepath.Join(dir, name)
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			resolved = filepath.Clean(resolved)
			if sandbox.IsSubpath(dir, resolved) {
				if st, e := os.Stat(resolved); e == nil && !st.IsDir() {
					return resolved, nil
				}
			}
		}
	}
	return "", errors.New("manifest must declare an executable")
}

func validType(v string) bool {
	switch v {
	case "Discovery", "ClassificationRule", "Capture", "Restore", "Storage":
		return true
	}
	return false
}
func flattenCapabilities(c *capabilitiesFile) []string {
	return append(append(append(append(append([]string{}, c.OSFamilies...), c.PackageManagers...), c.CloudProviders...), c.DiscoveryCategories...), c.StorageProtocols...)
}

// Plugins returns a stable snapshot of registered plugins.
func (h *Host) Plugins() []Plugin {
	out := make([]Plugin, 0, len(h.plugins))
	//nolint:gocritic // intentional value copy: Plugins() returns a defensive snapshot
	for _, p := range h.plugins {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Invoke runs exactly one fresh plugin process and returns its JSON response.
func (h *Host) Invoke(ctx context.Context, id string, request any) (json.RawMessage, error) {
	p, ok := h.plugins[id]
	if !ok {
		return nil, fmt.Errorf("plugin %q is not registered", id)
	}
	input, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, h.opts.Timeout)
	defer cancel()
	cmd := exec.CommandContext(callCtx, p.Executable) // #nosec G204 -- p.Executable only comes from a trusted, already-validated plugin manifest we control
	cmd.Dir = p.Dir
	cmd.Stdin = bytes.NewReader(append(input, '\n'))
	var out bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &out, n: h.opts.OutputLimit}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Run()
	if callCtx.Err() != nil {
		return nil, fmt.Errorf("plugin %q timed out", id)
	}
	if err != nil {
		return nil, fmt.Errorf("plugin %q failed: %w (%s)", id, err, strings.TrimSpace(stderr.String()))
	}
	var response json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("plugin %q returned malformed JSON: %w", id, err)
	}
	return response, nil
}

type limitedWriter struct {
	w          io.Writer
	n, written int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if int64(len(p))+w.written > w.n {
		return 0, errors.New("plugin output exceeds limit")
	}
	n, e := w.w.Write(p)
	w.written += int64(n)
	return n, e
}

// InvokeSandboxed runs a plugin through the sandbox if configured, falling
// back to direct invocation otherwise. This is the preferred entry point
// for production plugin execution.
func (h *Host) InvokeSandboxed(ctx context.Context, id string, request any) (json.RawMessage, error) {
	if h.opts.Sandbox == nil {
		return h.Invoke(ctx, id, request)
	}
	p, ok := h.plugins[id]
	if !ok {
		return nil, fmt.Errorf("plugin %q is not registered", id)
	}
	input, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}

	policy := h.buildPolicy(&p)
	result, err := h.opts.Sandbox.Exec(ctx, p.Executable, p.Dir,
		append(input, '\n'), policy)
	if err != nil {
		return nil, fmt.Errorf("plugin %q sandbox exec: %w", id, err)
	}
	var response json.RawMessage
	if err := json.Unmarshal(result.Stdout, &response); err != nil {
		return nil, fmt.Errorf("plugin %q returned malformed JSON: %w", id, err)
	}
	return response, nil
}

// buildPolicy translates a plugin's manifest permissions into a sandbox Policy.
func (h *Host) buildPolicy(p *Plugin) *sandbox.Policy {
	return &sandbox.Policy{
		ReadPaths:   append([]string{p.Dir}, p.Manifest.FilesystemRead...),
		WritePaths:  p.Manifest.FilesystemWrite,
		Executables: []string{p.Executable},
		Network:     p.Manifest.NetworkAllowed,
		TimeoutSec:  int(h.opts.Timeout.Seconds()),
		OutputLimit: h.opts.OutputLimit,
	}
}
