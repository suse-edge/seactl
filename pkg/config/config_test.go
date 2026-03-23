package config

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

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
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestHelperProcessSuccess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	if len(args) > 3 && args[3] == "create" {
		os.Stdout.WriteString("fake-container-id\n")
	}
	os.Exit(0)
}

func TestHelperProcessFail(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	os.Exit(1)
}

func TestReadAirgapManifest_InvalidMode(t *testing.T) {
	_, _, err := ReadAirgapManifest("1.0.0", "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid release mode")
}

func TestExtractFileFromContainer_Success(t *testing.T) {
	oldExtract := extractFileFromContainer
	extractFileFromContainer = func(imageURL, filePath string) ([]byte, error) {
		return []byte("content"), nil
	}
	defer func() { extractFileFromContainer = oldExtract }()

	data, err := extractFileFromContainer("fake-image", "/release_manifest.yaml")
	assert.NoError(t, err)
	assert.Equal(t, "content", string(data))
}

func TestExtractFileFromContainer_FailPull(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeExecCommandFail
	defer func() { execCommand = oldExec }()

	_, err := extractFileFromContainer("fake-image", "/release_manifest.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to pull image")
}

func TestExtractFileFromContainer_NoRegularFile(t *testing.T) {
	oldExtract := extractFileFromContainer
	extractFileFromContainer = func(imageURL, filePath string) ([]byte, error) {
		return nil, fmt.Errorf("failed to read tar header: EOF")
	}
	defer func() { extractFileFromContainer = oldExtract }()

	_, err := extractFileFromContainer("fake-image", "/release_manifest.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EOF")
}

func TestReadAirgapManifest_SuccessFactory(t *testing.T) {
	oldExtract := extractFileFromContainer
	extractFileFromContainer = func(imageURL, filePath string) ([]byte, error) {
		if strings.Contains(filePath, "release_manifest") {
			return []byte("kind: ReleaseManifest"), nil
		}
		return []byte("kind: ImagesManifest"), nil
	}
	defer func() { extractFileFromContainer = oldExtract }()

	rm, im, err := ReadAirgapManifest("1.0.0", "factory")
	assert.NoError(t, err)
	assert.NotNil(t, rm)
	assert.NotNil(t, im)
}

func TestReadAirgapManifest_SuccessProduction(t *testing.T) {
	oldExtract := extractFileFromContainer
	extractFileFromContainer = func(imageURL, filePath string) ([]byte, error) {
		if strings.Contains(filePath, "release_manifest") {
			return []byte("kind: ReleaseManifest"), nil
		}
		return []byte("kind: ImagesManifest"), nil
	}
	defer func() { extractFileFromContainer = oldExtract }()

	rm, im, err := ReadAirgapManifest("1.2.3", "production")
	assert.NoError(t, err)
	assert.NotNil(t, rm)
	assert.NotNil(t, im)
}

func TestReadAirgapManifest_InvalidYAMLManifest(t *testing.T) {
	oldExtract := extractFileFromContainer
	extractFileFromContainer = func(imageURL, filePath string) ([]byte, error) {
		if strings.Contains(filePath, "release_manifest") {
			return []byte("invalid: yaml: :::"), nil
		}
		return []byte("kind: ImagesManifest"), nil
	}
	defer func() { extractFileFromContainer = oldExtract }()

	_, _, err := ReadAirgapManifest("1.0.0", "factory")
	assert.Error(t, err)
}

func TestReadAirgapManifest_InvalidYAMLImages(t *testing.T) {
	oldExtract := extractFileFromContainer
	extractFileFromContainer = func(imageURL, filePath string) ([]byte, error) {
		if strings.Contains(filePath, "release_manifest") {
			return []byte("kind: ReleaseManifest"), nil
		}
		return []byte("invalid: yaml: :::"), nil
	}
	defer func() { extractFileFromContainer = oldExtract }()

	_, _, err := ReadAirgapManifest("1.0.0", "factory")
	assert.Error(t, err)
}

func TestRealExtractFileFromContainer_Failure(t *testing.T) {
	oldExec := execCommand
	execCommand = fakeExecCommandFail
	defer func() { execCommand = oldExec }()
	_, err := extractFileFromContainer("fake-image", "fake-path")
	assert.Error(t, err)
}

func TestRealExtractFileFromContainer_CreateFailure(t *testing.T) {
	oldExec := execCommand
	execCommand = func(command string, args ...string) *exec.Cmd {
		if command == "podman" && args[0] == "pull" {
			return fakeExecCommandSuccess(command, args...)
		}
		if command == "podman" && args[0] == "create" {
			return fakeExecCommandFail(command, args...)
		}
		return fakeExecCommandSuccess(command, args...)
	}
	defer func() { execCommand = oldExec }()
	_, err := extractFileFromContainer("fake-image", "fake-path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create container")
}

func TestRealExtractFileFromContainer_ExtractFailure(t *testing.T) {
	oldExec := execCommand
	execCommand = func(command string, args ...string) *exec.Cmd {
		if command == "podman" && args[0] == "pull" {
			return fakeExecCommandSuccess(command, args...)
		}
		if command == "podman" && args[0] == "create" {
			return fakeExecCommandSuccess(command, args...)
		}
		if command == "podman" && args[0] == "cp" {
			return fakeExecCommandFail(command, args...)
		}
		return fakeExecCommandSuccess(command, args...)
	}
	defer func() { execCommand = oldExec }()
	_, err := extractFileFromContainer("fake-image", "fake-path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to extract file")
}

func TestReadAirgapManifest_LocalSuccess(t *testing.T) {
	// Create temp files
	f1, err := os.CreateTemp("", "release_manifest.yaml")
	assert.NoError(t, err)
	defer os.Remove(f1.Name())
	f1.WriteString("kind: ReleaseManifest")
	f1.Close()

	f2, err := os.CreateTemp("", "release_images.yaml")
	assert.NoError(t, err)
	defer os.Remove(f2.Name())
	f2.WriteString("kind: ImagesManifest")
	f2.Close()

	// Override paths
	oldRM := ReleaseManifestPath
	oldRI := ReleaseImagesPath
	ReleaseManifestPath = f1.Name()
	ReleaseImagesPath = f2.Name()
	defer func() {
		ReleaseManifestPath = oldRM
		ReleaseImagesPath = oldRI
	}()

	rm, im, err := ReadAirgapManifest("", "")
	assert.NoError(t, err)
	assert.NotNil(t, rm)
	assert.NotNil(t, im)
}

func TestReadAirgapManifest_LocalMissingFile(t *testing.T) {
	oldRM := ReleaseManifestPath
	ReleaseManifestPath = "/path/does/not/exist"
	defer func() { ReleaseManifestPath = oldRM }()

	_, _, err := ReadAirgapManifest("", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read local release manifest")
}
