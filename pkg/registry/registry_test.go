package registry

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"net/http"
	"net/http/httptest"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "authfile")
	err := os.WriteFile(tmp, []byte(content), 0600)
	require.NoError(t, err)
	return tmp
}

var (
	origExecCommand   = execCommand
	origRemoteCatalog = remoteCatalog
)

func setupTest(t *testing.T) {
	execCommand = origExecCommand
	remoteCatalog = origRemoteCatalog
}

func TestNew(t *testing.T) {
	setupTest(t)

	r := New("auth.json", "my-registry.io", "ca.crt", true)
	assert.Equal(t, "auth.json", r.RegistryAuthFile)
	assert.Equal(t, "my-registry.io", r.RegistryURL)
	assert.Equal(t, "ca.crt", r.RegistryCACert)
	assert.True(t, r.RegistryInsecure)
}

func TestGetUserFromAuthFile_Success(t *testing.T) {
	setupTest(t)

	userEnc := "dXNlcg=="     // base64 "user"
	passEnc := "cGFzc3dvcmQ=" // base64 "password"
	authFile := writeTempFile(t, userEnc+":"+passEnc)

	r := New(authFile, "my-registry.io", "", false)
	creds, err := r.GetUserFromAuthFile()
	require.NoError(t, err)
	assert.Equal(t, []string{"user", "password"}, creds)
}

func TestGetUserFromAuthFile_InvalidFormat(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "invalid-data")

	r := New(authFile, "my-registry.io", "", false)
	_, err := r.GetUserFromAuthFile()
	assert.Error(t, err)
}

func TestGetUserFromAuthFile_InvalidBase64(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "notb64:notb64")

	r := New(authFile, "my-registry.io", "", false)
	_, err := r.GetUserFromAuthFile()
	assert.Error(t, err)
}

func TestRegistryHelmLogin_WithCredentials(t *testing.T) {
	setupTest(t)

	userEnc := "dXNlcg=="
	passEnc := "cGFzcw=="
	authFile := writeTempFile(t, userEnc+":"+passEnc)

	r := New(authFile, "my-registry.io", "", true)

	// Mock exec.Command
	execCommand = func(command string, args ...string) *exec.Cmd {
		assert.Equal(t, "helm", command)
		assert.Contains(t, args, "login")
		assert.Contains(t, args, "--username")
		assert.Contains(t, args, "--password")
		return exec.Command("echo") // always "success"
	}

	err := r.RegistryHelmLogin()
	assert.NoError(t, err)
}

func TestRegistryHelmLogin_FailExec(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "dXNlcg==:cGFzcw==")
	r := New(authFile, "my-registry.io", "", false)

	execCommand = func(command string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	err := r.RegistryHelmLogin()
	assert.Error(t, err)
}

func TestRegistryLogin_InvalidCACert(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "dXNlcg==:cGFzc3dvcmQ=")
	r := New(authFile, "my-registry.io", "missing-ca.crt", false)

	err := r.RegistryLogin()
	assert.Error(t, err)
}

func TestRegistryLogin_InvalidAuthFile(t *testing.T) {
	setupTest(t)

	r := New("not-exists.json", "my-registry.io", "", false)
	err := r.RegistryLogin()
	assert.Error(t, err)
}

func TestRegistryLogin_Success(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "dXNlcg==:cGFzc3dvcmQ=")
	r := New(authFile, "my-registry.io", "", true)

	// Mock  remote.Catalog
	remoteCatalog = func(ctx context.Context, reg name.Registry, opts ...remote.Option) ([]string, error) {
		return []string{"repo1", "repo2"}, nil
	}

	err := r.RegistryLogin()
	assert.NoError(t, err)
}

func TestRegistryLogin_FailCatalog(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "dXNlcg==:cGFzc3dvcmQ=")
	r := New(authFile, "my-registry.io", "", true)

	// Mock  remote.Catalog
	remoteCatalog = func(ctx context.Context, reg name.Registry, opts ...remote.Option) ([]string, error) {
		return nil, errors.New("fake error")
	}

	err := r.RegistryLogin()
	assert.Error(t, err)
}

func TestAuthTransport_RoundTrip(t *testing.T) {
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
w.Header().Set("WWW-Authenticate", "Basic realm=\"Registry Realm\"")
w.WriteHeader(http.StatusUnauthorized)
}))
defer ts.Close()

transport := &authTransport{
base: http.DefaultTransport,
username:  "testuser",
password:  "testpass",
}

req, _ := http.NewRequest("GET", ts.URL, nil)
resp, err := transport.RoundTrip(req)
assert.NoError(t, err)
assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreateHarborProject(t *testing.T) {
ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
assert.Equal(t, "/api/v2.0/projects", r.URL.Path)
assert.Equal(t, "POST", r.Method)
w.WriteHeader(http.StatusCreated)
}))
defer ts.Close()

r := New(ts.URL, "user", "/dev/null", false)
err := r.CreateHarborProject("edge")
assert.NoError(t, err)
}
