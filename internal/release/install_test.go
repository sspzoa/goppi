package release_test

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sspzoa/goppi/internal/config"
)

func fakeGoInstallScript(gopath, goVersion string) string {
	return fmt.Sprintf(`#!/bin/sh
case "$1" in
install)
  mkdir -p %q/bin
  cat > %q/bin/goppi <<'EOF'
#!/bin/sh
echo goppi 0.0.0-fake
EOF
  chmod +x %q/bin/goppi
  exit 0
;;
env)
  case "$2" in
  GOVERSION) echo %q ;;
  GOPATH) echo %q ;;
  *) exit 1 ;;
  esac
  ;;
*)
  exit 1
  ;;
esac
`, gopath, gopath, gopath, goVersion, gopath)
}

func writeFakeGo(t *testing.T, goVersion string) string {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	gopath := filepath.Join(t.TempDir(), "gopath")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if goVersion == "" {
		goVersion = "go1.27.0"
	}
	script := fakeGoInstallScript(gopath, goVersion)
	if err := os.WriteFile(filepath.Join(binDir, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir
}

func writeFakeCosign(t *testing.T, exitCode int) string {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`#!/bin/sh
for arg in "$@"; do
  case "$arg" in
  verify-blob) exit %d ;;
  esac
done
exit 1
`, exitCode)
	if err := os.WriteFile(filepath.Join(binDir, "cosign"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir
}

func TestInstallScriptFromLocalRelease(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	dist := t.TempDir()
	ver := "0.0.0-test"
	pack := exec.Command("bash", filepath.Join(root, "scripts/package.sh"), ver, dist, runtime.GOOS, runtime.GOARCH)
	pack.Dir = root
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	sums, err := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sums), "goppi_"+ver+"_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz") {
		t.Fatalf("SHA256SUMS missing archive:\n%s", sums)
	}

	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	prefix := filepath.Join(t.TempDir(), "bin")
	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = append(os.Environ(),
		"GOPPI_RELEASE_BASE="+srv.URL,
		"GOPPI_INSTALL_DIR="+prefix,
		"GOPPI_SKIP_COSIGN=1",
		"GOPPI_INSTALL_FROM=",
	)
	if out, err := inst.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	bin := filepath.Join(prefix, "goppi")
	got, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), ver) {
		t.Fatalf("version %q", got)
	}
}

func TestInstallScriptWithFakeCosign(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	dist := t.TempDir()
	ver := "0.0.0-cosign"
	pack := exec.Command("bash", filepath.Join(root, "scripts/package.sh"), ver, dist, runtime.GOOS, runtime.GOARCH)
	pack.Dir = root
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS.sigstore.json"), []byte(`{"mediaType":"fake"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	cosignBin := writeFakeCosign(t, 0)
	prefix := filepath.Join(t.TempDir(), "bin")
	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=" + cosignBin + ":/usr/bin:/bin",
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_DIR=" + prefix,
		"GOPPI_INSTALL_FROM=",
		"HOME=" + t.TempDir(),
	}
	if out, err := inst.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	bin := filepath.Join(prefix, "goppi")
	got, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), ver) {
		t.Fatalf("version %q", got)
	}
}

func TestInstallScriptRejectsCosignFailure(t *testing.T) {
	root := repoRoot(t)
	dist := t.TempDir()
	name := "goppi_x_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte("deadbeef  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, name), []byte("not-a-tarball"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS.sigstore.json"), []byte(`{"payload":"unsigned"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	cosignBin := writeFakeCosign(t, 1)
	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=" + cosignBin + ":/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_DIR=" + t.TempDir(),
	}
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("cosign failure should abort install\n%s", out)
	}
	if !strings.Contains(string(out), "signature check failed") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptRejectsBadChecksum(t *testing.T) {
	root := repoRoot(t)
	dist := t.TempDir()
	name := "goppi_x_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte("deadbeef  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, name), []byte("not-a-tarball"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = append(os.Environ(),
		"GOPPI_RELEASE_BASE="+srv.URL,
		"GOPPI_INSTALL_DIR="+t.TempDir(),
		"GOPPI_SKIP_COSIGN=1",
		"GOPPI_INSTALL_FROM=",
	)
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("expected checksum failure\n%s", out)
	}
	if !strings.Contains(string(out), "checksum mismatch") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptMirrorRequiresCosign(t *testing.T) {
	root := repoRoot(t)
	dist := t.TempDir()
	name := "goppi_x_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte("deadbeef  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, name), []byte("not-a-tarball"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = append(os.Environ(),
		"GOPPI_RELEASE_BASE="+srv.URL,
		"GOPPI_INSTALL_DIR="+t.TempDir(),
		"GOPPI_SKIP_COSIGN=",
		"GOPPI_REQUIRE_COSIGN=",
		"GOPPI_INSTALL_FROM=",
	)
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("mirror without signature should fail\n%s", out)
	}
	msg := string(out)
	if !strings.Contains(msg, "cosign") && !strings.Contains(msg, "signature") && !strings.Contains(msg, "sigstore") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptBundleRequiresCosign(t *testing.T) {
	root := repoRoot(t)
	dist := t.TempDir()
	name := "goppi_x_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte("deadbeef  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, name), []byte("not-a-tarball"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS.sigstore.json"), []byte(`{"payload":"unsigned"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = append(os.Environ(),
		"GOPPI_RELEASE_BASE="+srv.URL,
		"GOPPI_INSTALL_DIR="+t.TempDir(),
		"GOPPI_SKIP_COSIGN=",
		"GOPPI_REQUIRE_COSIGN=",
		"GOPPI_INSTALL_FROM=",
	)
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("bundle without verified signature should fail\n%s", out)
	}
	msg := string(out)
	if !strings.Contains(msg, "cosign") && !strings.Contains(msg, "signature") && !strings.Contains(msg, "sigstore") {
		t.Fatalf("got %s", out)
	}
}

func TestPackageScriptEmbedsVersion(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	ver := config.Version
	dist := t.TempDir()
	pack := exec.Command("bash", filepath.Join(root, "scripts/package.sh"), ver, dist, runtime.GOOS, runtime.GOARCH)
	pack.Dir = root
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	name := "goppi_" + ver + "_" + runtime.GOOS + "_" + runtime.GOARCH
	extract := t.TempDir()
	tar := exec.Command("tar", "-xzf", filepath.Join(dist, name+".tar.gz"), "-C", extract)
	if out, err := tar.CombinedOutput(); err != nil {
		t.Fatalf("extract: %v\n%s", err, out)
	}
	bin := filepath.Join(extract, name)
	got, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), ver) {
		t.Fatalf("version %q want %s", got, ver)
	}
}

func TestPackageScriptSHA256SUMSValid(t *testing.T) {
	root := repoRoot(t)
	ver := "0.0.0-shasum-test"
	dist := t.TempDir()
	pack := exec.Command("bash", filepath.Join(root, "scripts/package.sh"), ver, dist)
	pack.Dir = root
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	check := "sha256sum -c SHA256SUMS"
	if runtime.GOOS == "darwin" {
		check = "shasum -a 256 -c SHA256SUMS"
	}
	cmd := exec.Command("bash", "-c", check)
	cmd.Dir = dist
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checksum verify: %v\n%s", err, out)
	}
}

func TestPackageScriptTarballLayout(t *testing.T) {
	root := repoRoot(t)
	ver := "0.0.0-layout-test"
	dist := t.TempDir()
	pack := exec.Command("bash", filepath.Join(root, "scripts/package.sh"), ver, dist)
	pack.Dir = root
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	for _, osName := range []string{"darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			name := "goppi_" + ver + "_" + osName + "_" + arch
			cmd := exec.Command("tar", "-tzf", filepath.Join(dist, name+".tar.gz"))
			out, err := cmd.Output()
			if err != nil {
				t.Fatalf("tar %s: %v", name, err)
			}
			if got := strings.TrimSpace(string(out)); got != name {
				t.Fatalf("tarball %s contents %q want %q", name, got, name)
			}
		}
	}
}

func TestPackageScriptSHA256SUMSOnlyBuiltArchives(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	dist := t.TempDir()
	stale := filepath.Join(dist, "goppi_stale_linux_amd64.tar.gz")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	ver := "0.0.0-stale-test"
	pack := exec.Command("bash", filepath.Join(root, "scripts/package.sh"), ver, dist, runtime.GOOS, runtime.GOARCH)
	pack.Dir = root
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	sums, err := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(sums)
	if strings.Contains(text, "goppi_stale_") {
		t.Fatalf("SHA256SUMS must not list stale tarballs:\n%s", text)
	}
	want := "goppi_" + ver + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if !strings.Contains(text, want) {
		t.Fatalf("SHA256SUMS missing %s:\n%s", want, text)
	}
}

func TestInstallScriptBundleRequiresCosignBinary(t *testing.T) {
	root := repoRoot(t)
	dist := t.TempDir()
	name := "goppi_x_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz"
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte("deadbeef  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, name), []byte("not-a-tarball"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS.sigstore.json"), []byte(`{"payload":"unsigned"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_DIR=" + t.TempDir(),
	}
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("bundle without cosign binary should fail\n%s", out)
	}
	if !strings.Contains(string(out), "cosign is required") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptInstallFromGo(t *testing.T) {
	root := repoRoot(t)
	goBin := writeFakeGo(t, "go1.27.0")
	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=" + goBin + ":/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_INSTALL_FROM=go",
	}
	out, err := inst.CombinedOutput()
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "installed:") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptFallbackWhenNoRelease(t *testing.T) {
	root := repoRoot(t)
	goBin := writeFakeGo(t, "go1.27.0")
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=" + goBin + ":/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_FROM=",
	}
	out, err := inst.CombinedOutput()
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	msg := string(out)
	if !strings.Contains(msg, "falling back to go install") || !strings.Contains(msg, "installed:") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptFallbackWhenArchMissing(t *testing.T) {
	root := repoRoot(t)
	goBin := writeFakeGo(t, "go1.27.0")
	dist := t.TempDir()
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte("deadbeef  goppi_x_linux_amd64.tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=" + goBin + ":/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_SKIP_COSIGN=1",
		"GOPPI_INSTALL_FROM=",
	}
	out, err := inst.CombinedOutput()
	if err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	msg := string(out)
	if !strings.Contains(msg, "falling back to go install") || !strings.Contains(msg, "installed:") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptRejectsOldGo(t *testing.T) {
	root := repoRoot(t)
	goBin := writeFakeGo(t, "go1.26.0")
	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=" + goBin + ":/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_INSTALL_FROM=go",
	}
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("old go should fail\n%s", out)
	}
	if !strings.Contains(string(out), "Go 1.27+ required") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptRejectsBrokenArchive(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	dist := t.TempDir()
	ver := "0.0.0-bad-archive"
	name := fmt.Sprintf("goppi_%s_%s_%s", ver, runtime.GOOS, runtime.GOARCH)
	tarPath := filepath.Join(dist, name+".tar.gz")
	wrongDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(wrongDir, "wrong"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tar := exec.Command("tar", "-czf", tarPath, "-C", wrongDir, "wrong")
	if out, err := tar.CombinedOutput(); err != nil {
		t.Fatalf("tar: %v\n%s", err, out)
	}
	data, err := os.ReadFile(tarPath)
	if err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256(data))
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte(sum+"  "+name+".tar.gz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_DIR=" + t.TempDir(),
		"GOPPI_SKIP_COSIGN=1",
		"GOPPI_INSTALL_FROM=",
	}
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("broken archive should fail\n%s", out)
	}
	if !strings.Contains(string(out), "did not contain") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptNoGoWhenNoRelease(t *testing.T) {
	root := repoRoot(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_FROM=",
	}
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("missing release and go should fail\n%s", out)
	}
	msg := string(out)
	if !strings.Contains(msg, "falling back to go install") || !strings.Contains(msg, "go is not installed") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptRejectsDownloadFailure(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	dist := t.TempDir()
	ver := "0.0.0-dl-fail"
	name := fmt.Sprintf("goppi_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	if err := os.WriteFile(filepath.Join(dist, "SHA256SUMS"), []byte("deadbeef  "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS") {
			http.ServeFile(w, r, filepath.Join(dist, "SHA256SUMS"))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=/usr/bin:/bin",
		"HOME=" + t.TempDir(),
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_DIR=" + t.TempDir(),
		"GOPPI_SKIP_COSIGN=1",
		"GOPPI_INSTALL_FROM=",
	}
	out, err := inst.CombinedOutput()
	if err == nil {
		t.Fatalf("download failure should fail\n%s", out)
	}
	if !strings.Contains(string(out), "failed to download") {
		t.Fatalf("got %s", out)
	}
}

func TestInstallScriptInstallsToCustomPrefix(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	dist := t.TempDir()
	ver := "0.0.0-prefix-test"
	pack := exec.Command("bash", filepath.Join(root, "scripts/package.sh"), ver, dist, runtime.GOOS, runtime.GOARCH)
	pack.Dir = root
	if out, err := pack.CombinedOutput(); err != nil {
		t.Fatalf("package: %v\n%s", err, out)
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	prefix := filepath.Join(t.TempDir(), "bin")
	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_DIR=" + prefix,
		"GOPPI_SKIP_COSIGN=1",
		"GOPPI_INSTALL_FROM=",
		"HOME=" + t.TempDir(),
	}
	if out, err := inst.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	bin := filepath.Join(prefix, "goppi")
	if _, err := os.Stat(bin); err != nil {
		t.Fatal(err)
	}
	got, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), ver) {
		t.Fatalf("version %q", got)
	}
}

func TestInstallScriptFromDistDir(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	dist := os.Getenv("GOPPI_DIST")
	if dist == "" {
		t.Skip("set GOPPI_DIST (make verify-dist)")
	}
	if !filepath.IsAbs(dist) {
		dist = filepath.Join(root, dist)
	}
	ver := config.Version
	archive := fmt.Sprintf("goppi_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	if _, err := os.Stat(filepath.Join(dist, archive)); err != nil {
		t.Fatalf("missing %s in %s: %v", archive, dist, err)
	}
	sums, err := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sums), archive) {
		t.Fatalf("SHA256SUMS missing %s:\n%s", archive, sums)
	}

	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	prefix := filepath.Join(t.TempDir(), "bin")
	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_DIR=" + prefix,
		"GOPPI_SKIP_COSIGN=1",
		"GOPPI_INSTALL_FROM=",
		"HOME=" + t.TempDir(),
	}
	if out, err := inst.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	bin := filepath.Join(prefix, "goppi")
	got, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), ver) {
		t.Fatalf("version %q want %s", got, ver)
	}
}

func TestInstallScriptFromDistDirWithCosign(t *testing.T) {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64":
	default:
		t.Skip("no release binary for this GOOS/GOARCH")
	}
	root := repoRoot(t)
	dist := os.Getenv("GOPPI_DIST")
	if dist == "" {
		t.Skip("set GOPPI_DIST (make verify-dist)")
	}
	if !filepath.IsAbs(dist) {
		dist = filepath.Join(root, dist)
	}
	bundle := filepath.Join(dist, "SHA256SUMS.sigstore.json")
	if _, err := os.Stat(bundle); err != nil {
		if err := os.WriteFile(bundle, []byte(`{"mediaType":"fake"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ver := config.Version
	archive := fmt.Sprintf("goppi_%s_%s_%s.tar.gz", ver, runtime.GOOS, runtime.GOARCH)
	if _, err := os.Stat(filepath.Join(dist, archive)); err != nil {
		t.Fatalf("missing %s in %s: %v", archive, dist, err)
	}

	srv := httptest.NewServer(http.FileServer(http.Dir(dist)))
	t.Cleanup(srv.Close)

	cosignBin := writeFakeCosign(t, 0)
	prefix := filepath.Join(t.TempDir(), "bin")
	inst := exec.Command("bash", filepath.Join(root, "install.sh"))
	inst.Env = []string{
		"PATH=" + cosignBin + ":/usr/bin:/bin",
		"GOPPI_RELEASE_BASE=" + srv.URL,
		"GOPPI_INSTALL_DIR=" + prefix,
		"GOPPI_INSTALL_FROM=",
		"HOME=" + t.TempDir(),
	}
	if out, err := inst.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	bin := filepath.Join(prefix, "goppi")
	got, err := exec.Command(bin, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version: %v\n%s", err, got)
	}
	if !strings.Contains(string(got), ver) {
		t.Fatalf("version %q want %s", got, ver)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "install.sh")); err != nil {
		t.Fatal(err)
	}
	return root
}
