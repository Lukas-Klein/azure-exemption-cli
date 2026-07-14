package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadFromFile(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		path := writeConfig(t, "blocked_policy_definition_ids:\n  - /DEF/One\n  - /def/two\n")
		cfg, err := LoadFromFile(path)
		if err != nil {
			t.Fatalf("LoadFromFile() error = %v", err)
		}
		want := []string{"/DEF/One", "/def/two"}
		if !reflect.DeepEqual(cfg.BlockedPolicyDefinitionIDs, want) {
			t.Fatalf("blocked IDs = %#v, want %#v", cfg.BlockedPolicyDefinitionIDs, want)
		}
	})

	t.Run("empty", func(t *testing.T) {
		cfg, err := LoadFromFile(writeConfig(t, ""))
		if err != nil || len(cfg.BlockedPolicyDefinitionIDs) != 0 {
			t.Fatalf("LoadFromFile(empty) = %#v, %v", cfg, err)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		if _, err := LoadFromFile(writeConfig(t, "blocked_policy_definition_ids: [")); err == nil {
			t.Fatal("LoadFromFile() error = nil, want YAML error")
		}
	})

	t.Run("missing", func(t *testing.T) {
		if _, err := LoadFromFile(filepath.Join(t.TempDir(), "missing.yaml")); !os.IsNotExist(err) {
			t.Fatalf("LoadFromFile() error = %v, want not-exist error", err)
		}
	})
}

func TestLoadFromPaths(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	if err := os.WriteFile(second, []byte("blocked_policy_definition_ids: [second]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFromPaths([]string{first, second})
	if err != nil {
		t.Fatalf("LoadFromPaths() error = %v", err)
	}
	if got := cfg.BlockedPolicyDefinitionIDs; !reflect.DeepEqual(got, []string{"second"}) {
		t.Fatalf("blocked IDs = %#v", got)
	}

	if err := os.WriteFile(first, []byte("blocked_policy_definition_ids: [first]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadFromPaths([]string{first, second})
	if err != nil || !reflect.DeepEqual(cfg.BlockedPolicyDefinitionIDs, []string{"first"}) {
		t.Fatalf("first existing path was not used: %#v, %v", cfg, err)
	}

	empty, err := LoadFromPaths([]string{filepath.Join(dir, "missing-a"), filepath.Join(dir, "missing-b")})
	if err != nil || empty == nil || len(empty.BlockedPolicyDefinitionIDs) != 0 {
		t.Fatalf("all missing = %#v, %v", empty, err)
	}

	bad := writeConfig(t, "blocked_policy_definition_ids: [")
	if _, err := LoadFromPaths([]string{bad, second}); err == nil {
		t.Fatal("malformed first file should stop path search")
	}
}

func TestDefaultConfigPaths(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	want := []string{
		"config.yaml",
		"config.yml",
		filepath.Join(home, ".azexempt", "config.yaml"),
		filepath.Join(home, ".azexempt", "config.yml"),
		filepath.Join(home, ".azure-exemption-cli", "config.yaml"),
		filepath.Join(home, ".azure-exemption-cli", "config.yml"),
		filepath.Join(xdg, "azexempt", "config.yaml"),
		filepath.Join(xdg, "azexempt", "config.yml"),
		filepath.Join(xdg, "azure-exemption-cli", "config.yaml"),
		filepath.Join(xdg, "azure-exemption-cli", "config.yml"),
	}
	if got := DefaultConfigPaths(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultConfigPaths() = %#v, want %#v", got, want)
	}
}

func TestBlockedDefinitionsMap(t *testing.T) {
	cfg := Config{BlockedPolicyDefinitionIDs: []string{"/DEF/One", "/def/TWO", "/DEF/One"}}
	want := map[string]bool{"/def/one": true, "/def/two": true}
	if got := cfg.BlockedDefinitionsMap(); !reflect.DeepEqual(got, want) {
		t.Fatalf("BlockedDefinitionsMap() = %#v, want %#v", got, want)
	}

	if got := (&Config{}).BlockedDefinitionsMap(); got == nil || len(got) != 0 {
		t.Fatalf("empty BlockedDefinitionsMap() = %#v", got)
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
