package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Safety tests ---

func TestIsActiveVenv_Match(t *testing.T) {
	t.Setenv("VIRTUAL_ENV", "/home/user/project/.venv")
	if !isActiveVenv("/home/user/project/.venv") {
		t.Error("expected active venv match")
	}
}

func TestIsActiveVenv_NoMatch(t *testing.T) {
	t.Setenv("VIRTUAL_ENV", "/home/user/project/.venv")
	if isActiveVenv("/home/user/other/.venv") {
		t.Error("expected no match for different path")
	}
}

func TestIsActiveVenv_Unset(t *testing.T) {
	t.Setenv("VIRTUAL_ENV", "")
	if isActiveVenv("/home/user/project/.venv") {
		t.Error("expected no match when VIRTUAL_ENV is empty")
	}
}

func TestIsProtectedPath_System(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/usr", true},
		{"/usr/local", true},
		{"/System", true},
		{"/System/Library", true},
		{"/Library", true},
		{"/bin", true},
		{"/sbin", true},
		{"/etc", true},
		{"/var", true},
		{"/tmp", true},
		{"/private", true},
		{"/Users/fred/dev/project/.venv", false},
		{"/home/user/project/node_modules", false},
	}
	for _, tt := range tests {
		got := isProtectedPath(tt.path)
		if got != tt.want {
			t.Errorf("isProtectedPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsProtectedPath_HomeAncestor(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	parent := filepath.Dir(home)
	if !isProtectedPath(parent) {
		t.Errorf("expected parent of home %q to be protected", parent)
	}
	if !isProtectedPath(home) {
		t.Errorf("expected home %q to be protected", home)
	}
	sub := filepath.Join(home, "dev/project/.venv")
	if isProtectedPath(sub) {
		t.Errorf("expected subdirectory of home %q to NOT be protected", sub)
	}
}

func TestIsValidVenv_OnlyPyvenvCfg(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyvenv.cfg"), []byte("home = /usr/bin\n"), 0644)
	if isValidVenv(dir) {
		t.Error("expected dir with only pyvenv.cfg to be invalid")
	}
}

func TestIsValidVenv_WithBin(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyvenv.cfg"), []byte("home = /usr/bin\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "bin"), 0755)
	if !isValidVenv(dir) {
		t.Error("expected dir with pyvenv.cfg + bin/ to be valid")
	}
}

func TestIsValidVenv_WithScripts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyvenv.cfg"), []byte("home = /usr/bin\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "Scripts"), 0755)
	if !isValidVenv(dir) {
		t.Error("expected dir with pyvenv.cfg + Scripts/ to be valid")
	}
}

func TestIsValidVenv_NoPyvenvCfg(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "bin"), 0755)
	if isValidVenv(dir) {
		t.Error("expected dir without pyvenv.cfg to be invalid")
	}
}

func TestGetSitePackagesUsage(t *testing.T) {
	dir := t.TempDir()
	spDir := filepath.Join(dir, "lib", "python3.11", "site-packages", "somepkg")
	os.MkdirAll(spDir, 0755)

	f := filepath.Join(spDir, "__init__.py")
	os.WriteFile(f, []byte("# test"), 0644)
	target := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	os.Chtimes(f, target, target)

	got, ok := getSitePackagesUsage(dir)
	if !ok {
		t.Fatal("expected to find site-packages usage")
	}
	if got.Sub(target).Abs() > time.Second {
		t.Errorf("got mtime %v, want ~%v", got, target)
	}
}

func TestGetSitePackagesUsage_NoSitePackages(t *testing.T) {
	dir := t.TempDir()
	_, ok := getSitePackagesUsage(dir)
	if ok {
		t.Error("expected no site-packages usage for empty dir")
	}
}

// --- Usage heuristic tests ---

func TestGetNodeModulesUsage_PackageLock(t *testing.T) {
	dir := t.TempDir()
	nmDir := filepath.Join(dir, "node_modules")
	os.MkdirAll(nmDir, 0755)

	// Create .package-lock.json inside node_modules (npm >=7)
	lockFile := filepath.Join(nmDir, ".package-lock.json")
	os.WriteFile(lockFile, []byte("{}"), 0644)
	target := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	os.Chtimes(lockFile, target, target)

	got, ok := getNodeModulesUsage(nmDir)
	if !ok {
		t.Fatal("expected to find node_modules usage")
	}
	if got.Sub(target).Abs() > time.Second {
		t.Errorf("got mtime %v, want ~%v", got, target)
	}
}

func TestGetNodeModulesUsage_ParentLockfile(t *testing.T) {
	dir := t.TempDir()
	nmDir := filepath.Join(dir, "node_modules")
	os.MkdirAll(nmDir, 0755)

	// No .package-lock.json in nm, but parent has package-lock.json
	lockFile := filepath.Join(dir, "package-lock.json")
	os.WriteFile(lockFile, []byte("{}"), 0644)
	target := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	os.Chtimes(lockFile, target, target)

	got, ok := getNodeModulesUsage(nmDir)
	if !ok {
		t.Fatal("expected to find node_modules usage from parent lockfile")
	}
	if got.Sub(target).Abs() > time.Second {
		t.Errorf("got mtime %v, want ~%v", got, target)
	}
}

func TestGetNodeModulesUsage_Fallback(t *testing.T) {
	dir := t.TempDir()
	nmDir := filepath.Join(dir, "node_modules")
	os.MkdirAll(nmDir, 0755)

	// No lockfiles anywhere -- falls back to dir mtime
	got, ok := getNodeModulesUsage(nmDir)
	if !ok {
		t.Fatal("expected fallback to dir mtime")
	}
	info, _ := os.Stat(nmDir)
	if got.Sub(info.ModTime()).Abs() > time.Second {
		t.Errorf("got mtime %v, want ~%v (dir mtime)", got, info.ModTime())
	}
}

func TestGetCacheUsage(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	os.MkdirAll(sub, 0755)

	f := filepath.Join(sub, "cache.json")
	os.WriteFile(f, []byte("{}"), 0644)
	target := time.Now().Add(-12 * time.Hour).Truncate(time.Second)
	os.Chtimes(f, target, target)

	got, ok := getCacheUsage(dir)
	if !ok {
		t.Fatal("expected to find cache usage")
	}
	if got.Sub(target).Abs() > time.Second {
		t.Errorf("got mtime %v, want ~%v", got, target)
	}
}

func TestGetCacheUsage_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, ok := getCacheUsage(dir)
	if !ok {
		t.Fatal("expected fallback to dir mtime")
	}
	info, _ := os.Stat(dir)
	if got.Sub(info.ModTime()).Abs() > time.Second {
		t.Errorf("got mtime %v, want ~%v (dir mtime)", got, info.ModTime())
	}
}

func TestGetBuildUsage(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "output.whl")
	os.WriteFile(f, []byte("fake"), 0644)
	target := time.Now().Add(-6 * time.Hour).Truncate(time.Second)
	os.Chtimes(f, target, target)

	got, ok := getBuildUsage(dir)
	if !ok {
		t.Fatal("expected to find build usage")
	}
	if got.Sub(target).Abs() > time.Second {
		t.Errorf("got mtime %v, want ~%v", got, target)
	}
}

func TestHasBuildParent_PyprojectToml(t *testing.T) {
	dir := t.TempDir()
	distDir := filepath.Join(dir, "dist")
	os.MkdirAll(distDir, 0755)
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0644)

	if !hasBuildParent(distDir) {
		t.Error("expected hasBuildParent=true with pyproject.toml in parent")
	}
}

func TestHasBuildParent_PackageJSON(t *testing.T) {
	dir := t.TempDir()
	distDir := filepath.Join(dir, "dist")
	os.MkdirAll(distDir, 0755)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)

	if !hasBuildParent(distDir) {
		t.Error("expected hasBuildParent=true with package.json in parent")
	}
}

func TestHasBuildParent_NoBuildFiles(t *testing.T) {
	dir := t.TempDir()
	distDir := filepath.Join(dir, "dist")
	os.MkdirAll(distDir, 0755)

	if hasBuildParent(distDir) {
		t.Error("expected hasBuildParent=false with no build files in parent")
	}
}
