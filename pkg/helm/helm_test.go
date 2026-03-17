package helm

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alknopfler/seactl/pkg/registry"
	"github.com/stretchr/testify/assert"
)

func fakeExecCommandSuccess(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessSuccess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func fakeExecCommandFail(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcessFail", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS_FAIL=1"}
	return cmd
}

func TestHelperProcessSuccess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(0)
}

func TestHelperProcessFail(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_FAIL") != "1" {
		return
	}
	os.Exit(1)
}

func TestDownload_OCI_Success(t *testing.T) {
	h := New("mychart", "1.0.0", "oci://registry.io/mychart", "", nil)
	execCommand = fakeExecCommandSuccess
	defer func() { execCommand = exec.Command }()

	err := h.Download()
	assert.NoError(t, err)
}

func TestDownload_Repo_Success(t *testing.T) {
	h := New("mychart", "1.0.0", "chartname", "https://charts.io", nil)
	execCommand = fakeExecCommandSuccess
	defer func() { execCommand = exec.Command }()

	err := h.Download()
	assert.NoError(t, err)
}

func TestDownload_Repo_MissingURL(t *testing.T) {
	h := New("mychart", "1.0.0", "chartname", "", nil)
	err := h.Download()
	assert.Error(t, err)
}

func TestDownload_Fail(t *testing.T) {
	h := New("mychart", "1.0.0", "oci://registry.io/mychart", "", nil)
	execCommand = fakeExecCommandFail
	defer func() { execCommand = exec.Command }()

	err := h.Download()
	assert.Error(t, err)
}

func TestVerify_Success(t *testing.T) {
	h := New("mychart", "1.0.0", "chart", "", nil)
	file := filepath.Join(tempDir, "chart-1.0.0.tgz")
	err := os.WriteFile(file, []byte("dummy"), 0600)
	assert.NoError(t, err)
	defer os.Remove(file)

	err = h.Verify()
	assert.NoError(t, err)
}

func TestVerify_Fail(t *testing.T) {
	h := New("mychart", "1.0.0", "chart", "", nil)
	err := h.Verify()
	assert.Error(t, err)
}

func TestUpload_Success_Insecure(t *testing.T) {
	reg := registry.New("auth.json", "registry.io", "", false)
	h := New("mychart", "1.0.0", "chart", "", reg)
	h.Insecure = true
	file := filepath.Join(tempDir, "chart-1.0.0.tgz")
	err := os.WriteFile(file, []byte("dummy"), 0600)
	assert.NoError(t, err)
	defer os.Remove(file)

	execCommand = fakeExecCommandSuccess
	defer func() { execCommand = exec.Command }()

	err = h.Upload()
	assert.NoError(t, err)
}

func TestUpload_Success_WithCA(t *testing.T) {
	caFile := filepath.Join(t.TempDir(), "ca.crt")
	err := os.WriteFile(caFile, []byte("dummy-ca"), 0600)
	assert.NoError(t, err)

	reg := registry.New("auth.json", "registry.io", caFile, false)
	h := New("mychart", "1.0.0", "chart", "", reg)
	file := filepath.Join(tempDir, "chart-1.0.0.tgz")
	err = os.WriteFile(file, []byte("dummy"), 0600)
	assert.NoError(t, err)
	defer os.Remove(file)

	execCommand = fakeExecCommandSuccess
	defer func() { execCommand = exec.Command }()

	err = h.Upload()
	assert.NoError(t, err)
}

func TestUpload_Fail(t *testing.T) {
	reg := registry.New("auth.json", "registry.io", "", false)
	h := New("mychart", "1.0.0", "chart", "", reg)
	file := filepath.Join(tempDir, "chart-1.0.0.tgz")
	err := os.WriteFile(file, []byte("dummy"), 0600)
	assert.NoError(t, err)
	defer os.Remove(file)

	execCommand = fakeExecCommandFail
	defer func() { execCommand = exec.Command }()

	err = h.Upload()
	assert.Error(t, err)
}
