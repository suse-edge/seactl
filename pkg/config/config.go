package config

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"github.com/alknopfler/seactl/pkg/logger"
	"os/exec"
	"strings"
)

const (
	// releaseMode factory url
	factoryReleaseURL    = "registry.opensuse.org/isv/suse/edge/factory/test_manifest_images/release-manifest:%s"
	productionReleaseURL = "registry.suse.com/edge/%s/release-manifest:%s"
	releaseManifestPath  = "/release_manifest.yaml"
	releaseImagesPath    = "/release_images.yaml"
)

var execCommand = exec.Command

// Func ReadAirgapManifest from a release-version and pull it from release container, and return a ReleaseManifest struct or an error if something goes wrong
func ReadAirgapManifest(version, mode string) (*ReleaseManifest, *ImagesManifest, error) {

	// Determine the input based on the mode
	var input string
	if mode == "factory" {
		// Use the factory release URL to replace the param version content in %s inside factoryReleaseURL
		input = fmt.Sprintf(factoryReleaseURL, version)
	} else if mode == "production" {
		// Use the production release URL to replace the param version content in %s inside productionReleaseURL
		input = fmt.Sprintf(productionReleaseURL, strings.Join(strings.Split(version, ".")[:2], "."), version)
	} else {
		return nil, nil, errors.New("invalid release mode, must be either 'factory' or 'production'")
	}

	// Read files content
	releaseManifestData, err := extractFileFromContainer(input, releaseManifestPath)
	if err != nil {
		logger.Printf("failed to read file: %v", err)
		return nil, nil, err
	}

	releaseImagesData, err := extractFileFromContainer(input, releaseImagesPath)
	if err != nil {
		logger.Printf("failed to read file: %v", err)
		return nil, nil, err
	}

	// Unmarshal YAML into struct
	var releaseManifest ReleaseManifest
	if err := yaml.Unmarshal(releaseManifestData, &releaseManifest); err != nil {
		logger.Printf("failed to unmarshal YAML: %v", err)
		return nil, nil, err
	}
	var releaseImages ImagesManifest
	if err := yaml.Unmarshal(releaseImagesData, &releaseImages); err != nil {
		logger.Printf("failed to unmarshal YAML: %v", err)
		return nil, nil, err
	}

	return &releaseManifest, &releaseImages, nil
}

var extractFileFromContainer = func(imageURL, filePath string) ([]byte, error) {
	// Pull image
	if err := execCommand("podman", "pull", imageURL).Run(); err != nil {
		return nil, fmt.Errorf("failed to pull image: %s %w", imageURL, err)
	}

	// Create container
	containerIDRaw, err := execCommand("podman", "create", imageURL).Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	containerID := strings.TrimSpace(string(containerIDRaw))

	// Extract file using podman cp (into tar stream)
	var buf bytes.Buffer
	cmd := execCommand("podman", "cp", fmt.Sprintf("%s:%s", containerID, filePath), "-")
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		execCommand("podman", "rm", containerID).Run()
		return nil, fmt.Errorf("failed to extract file: %s %w", filePath, err)
	}

	// Cleanup container
	_ = execCommand("podman", "rm", containerID).Run()

	// Read TAR stream and extract file content
	tarReader := tar.NewReader(&buf)
	for {
		header, err := tarReader.Next()
		if err != nil {
			return nil, fmt.Errorf("failed to read tar header: %w", err)
		}
		if header.Typeflag == tar.TypeReg {
			var fileContent bytes.Buffer
			if _, err := fileContent.ReadFrom(tarReader); err != nil {
				return nil, fmt.Errorf("failed to read file from tar: %w", err)
			}
			return fileContent.Bytes(), nil
		}
	}
}
