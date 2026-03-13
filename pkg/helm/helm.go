package helm

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alknopfler/seactl/pkg/registry"
)

const (
	tempDir = "./"
)

type Helm struct {
	Name     string // release name (e.g., "rancher")
	Chart    string // chart name or full OCI reference
	Version  string
	URL      string // optional repo URL (for HTTPS charts)
	TmpDir   string
	Insecure bool
	reg      *registry.Registry
}

var execCommand = exec.Command

func New(name, version, chart, url string, reg *registry.Registry) *Helm {
	return &Helm{
		Name:    name,
		Version: version,
		Chart:   chart,
		URL:     url,
		reg:     reg,
	}
}

func (h *Helm) Download() error {
	var args []string

	// Determine chart reference
	if strings.HasPrefix(h.Chart, "oci://") {
		// OCI chart: full reference is already in h.Chart
		args = []string{"pull", h.Chart, "--version", h.Version, "-d", tempDir}
	} else {
		if h.URL == "" {
			return fmt.Errorf("repository URL is missing for chart %s", h.Name)
		}

		// Regular Helm repo chart
		args = []string{
			"pull", h.Chart,
			"--repo", strings.TrimSuffix(h.URL, "/"),
			"--version", h.Version,
			"-d", tempDir,
		}
	}
	// Execute the command
	cmd := execCommand("helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		log.Printf("failed to download chart %s: %v", h.Chart, err)
		return err
	}

	return nil
}

func (h *Helm) Verify() error {
	_, err := h.findDownloadedChart()
	if err != nil {
		log.Printf("file does not exist to be verified %s", err.Error())
		return err
	}
	return nil
}

func (h *Helm) Upload() error {
	chartPath, err := h.findDownloadedChart()
	if err != nil {
		log.Printf("file does not exist to be uploaded %s", err.Error())
		return err
	}

	var args []string
	args = append(args, "push", chartPath, "oci://"+h.reg.RegistryURL)

	if h.Insecure {
		args = append(args, "--insecure-skip-tls-verify")
	} else if h.reg.RegistryCACert != "" {
		args = append(args, "--ca-file", h.reg.RegistryCACert)
	}

	cmd := execCommand("helm", args...)
	err = cmd.Run()
	if err != nil {
		log.Printf("failed to push to the registry: %s", err)
		return err
	}
	defer os.Remove(chartPath)
	return nil
}

func (h *Helm) findDownloadedChart() (string, error) {
	pattern := fmt.Sprintf("%s*.tgz", h.Name)
	matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple chart archives found for %s", h.Name)
	}
	return matches[0], nil
}
