package cmd

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/alknopfler/seactl/pkg/airgap"
	"github.com/stretchr/testify/assert"
)

var (
	origCheckHelm = airgap.CheckHelmCommand
	origGenerate  = airgap.GenerateAirGapEnvironment

	helmCalled  bool
	generateErr error

	generateParams struct {
		dryRun              bool
		releaseVersion      string
		releaseMode         string
		registryURL         string
		registryAuthFile    string
		rancherAppsAuthFile string
		registryCACert      string
		outputDir           string
		registryInsecure    bool
	}
)

// Mock functions
func fakeCheckHelm() error {
	helmCalled = true
	return nil
}

func fakeGenerate(
	dryRun bool, rv, rm, url, auth, rancherAuth, cacert, out string, insecure bool,
) error {
	generateParams = struct {
		dryRun              bool
		releaseVersion      string
		releaseMode         string
		registryURL         string
		registryAuthFile    string
		rancherAppsAuthFile string
		registryCACert      string
		outputDir           string
		registryInsecure    bool
	}{dryRun, rv, rm, url, auth, rancherAuth, cacert, out, insecure}
	return generateErr
}

// Helper function to run command and capture stdout/stderr
func runCommand(args []string) (stdout, stderr string, err error) {
	cmd := NewAirGapCommand()

	// Set args for Cobra to parse
	cmd.SetArgs(args)

	// Save original stdout/stderr
	origStdout := os.Stdout
	origStderr := os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	defer func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	// Run the command (Cobra parses flags from args)
	err = cmd.Execute()

	// Close writers so readers can finish
	wOut.Close()
	wErr.Close()

	var bufOut, bufErr bytes.Buffer
	io.Copy(&bufOut, rOut)
	io.Copy(&bufErr, rErr)

	return bufOut.String(), bufErr.String(), err
}

func TestMain(m *testing.M) {
	// Setup fakes
	airgap.CheckHelmCommand = fakeCheckHelm
	airgap.GenerateAirGapEnvironment = fakeGenerate

	code := m.Run()

	// Teardown
	airgap.CheckHelmCommand = origCheckHelm
	airgap.GenerateAirGapEnvironment = origGenerate

	os.Exit(code)
}

func TestInvalidReleaseMode_Error(t *testing.T) {
	_, stderr, err := runCommand([]string{
		"--release-mode", "invalid",
		"--release-version", "1.2.3",
		"--registry-url", "url",
		"--rancher-apps-authfile", "rancher-auth",
		"--output", "out",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for --release-mode")
	assert.Contains(t, stderr, "Error: invalid value for --release-mode")
}

func TestInvalidVersionFormat_Error(t *testing.T) {
	_, stderr, err := runCommand([]string{
		"--release-mode", "factory",
		"--release-version", "badver",
		"--registry-url", "url",
		"--rancher-apps-authfile", "rancher-auth",
		"--output", "out",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid release version format")
	assert.Contains(t, stderr, "Error: invalid release version format")
}

func TestGenerate_Success(t *testing.T) {
	helmCalled = false
	generateParams = struct {
		dryRun              bool
		releaseVersion      string
		releaseMode         string
		registryURL         string
		registryAuthFile    string
		rancherAppsAuthFile string
		registryCACert      string
		outputDir           string
		registryInsecure    bool
	}{}

	stdout, stderr, err := runCommand([]string{
		"--release-mode", "production",
		"--release-version", "1.2.3",
		"--registry-url", "reg",
		"--registry-authfile", "auth",
		"--rancher-apps-authfile", "rancher-auth",
		"--registry-cacert", "cacert",
		"--output", "out",
		"--dry-run",
		"--insecure",
	})

	assert.NoError(t, err)
	assert.Equal(t, "", stdout)
	assert.Equal(t, "", stderr)

	assert.True(t, helmCalled)
	assert.Equal(t, "production", generateParams.releaseMode)
	assert.Equal(t, "1.2.3", generateParams.releaseVersion)
	assert.Equal(t, "reg", generateParams.registryURL)
	assert.Equal(t, "auth", generateParams.registryAuthFile)
	assert.Equal(t, "rancher-auth", generateParams.rancherAppsAuthFile)
	assert.Equal(t, "cacert", generateParams.registryCACert)
	assert.Equal(t, "out", generateParams.outputDir)
	assert.True(t, generateParams.dryRun)
	assert.True(t, generateParams.registryInsecure)
}
