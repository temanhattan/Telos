package plugin

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writePlugin(t *testing.T, root, name, manifest string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		manifest = stringReplaceExecutable(manifest)
	}
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	body := "#!/bin/sh\necho '{\"status\":\"success\"}'\n"
	file := "plugin"
	if runtime.GOOS == "windows" {
		body = "@echo {\"status\":\"success\"}"
		file = "plugin.cmd"
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
}

func stringReplaceExecutable(manifest string) string {
	return manifest[:len(manifest)-len("plugin")] + "plugin.cmd"
}

const validManifest = `id: io.telos.discovery.test
name: Test
version: 1.0.0
interface_version: 1
type: Discovery
author: test
executable: plugin`

func TestDiscoverRegistersValidAndSkipsInvalid(t *testing.T) {
	root := t.TempDir()
	writePlugin(t, root, "valid", validManifest)
	writePlugin(t, root, "invalid", `id: io.telos.bad
name: Bad
version: 1.0.0
interface_version: 2
type: Discovery
author: test
executable: plugin`)
	h := New(Options{PluginDirs: []string{root}})
	failures := h.Discover()
	if len(failures) != 1 {
		t.Fatalf("got %d failures", len(failures))
	}
	if got := h.Plugins(); len(got) != 1 || got[0].ID != "io.telos.discovery.test" {
		t.Fatal("valid plugin was not registered")
	}
}

func TestInvokeUsesJSONProtocol(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture differs on Windows")
	}
	root := t.TempDir()
	writePlugin(t, root, "valid", validManifest)
	h := New(Options{PluginDirs: []string{root}})
	h.Discover()
	got, err := h.Invoke(context.Background(), "io.telos.discovery.test", map[string]string{"operation": "discover"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"status":"success"}` {
		t.Fatalf("unexpected response %s", got)
	}
}
