package helm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alknopfler/seactl/pkg/logger"
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
	
	if logger.Debug {
		logger.Debugf("Executing command: helm %s\n", strings.Join(args, " "))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = nil
	}

	if err := cmd.Run(); err != nil {
		logger.Printf("failed to download chart %s: %v", h.Chart, err)
		return err
	}

	logger.Debugf("Successfully downloaded chart %s\n", h.Chart)
	return nil
}

func (h *Helm) Verify() error {
	logger.Debugf("Verifying downloaded chart for %s\n", h.Name)
	_, err := h.findDownloadedChart()
	if err != nil {
		logger.Printf("file does not exist to be verified %s", err.Error())
		return err
	}
	return nil
}

func (h *Helm) Upload() error {
	chartPath, err := h.findDownloadedChart()
	if err != nil {
		logger.Printf("file does not exist to be uploaded %s", err.Error())
		return err
	}

	var args []string
	args = append(args, "push", chartPath, "oci://"+h.reg.RegistryURL+"/mirror")

	if h.Insecure {
		args = append(args, "--insecure-skip-tls-verify")
	} else if h.reg.RegistryCACert != "" {
		args = append(args, "--ca-file", h.reg.RegistryCACert)
	}

	cmd := execCommand("helm", args...)
	if logger.Debug {
		logger.Debugf("Executing upload command: helm %s\n", strings.Join(args, " "))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	
	err = cmd.Run()
	if err != nil {
		logger.Printf("failed to push to the registry: %s", err)
		return err
	}
	logger.Debugf("Successfully uploaded chart %s\n", h.Name)
	defer os.Remove(chartPath)
	return nil
}

func (h *Helm) findDownloadedChart() (string, error) {
	parts := strings.Split(h.Chart, "/")
	chartBase := parts[len(parts)-1]
	
	// Helm might save the file with or without a 'v' prefix depending on the chart's Chart.yaml version field.
	// We'll strip any 'v' first, then try both ways.
	cleanVersion := strings.TrimPrefix(h.Version, "v")
	
	pattern := fmt.Sprintf("%s-%s.tgz", chartBase, cleanVersion)
	matches, err := filepath.Glob(filepath.Join(tempDir, pattern))
	if err != nil {
		return "", err
	}
	
	if len(matches) == 0 {
		// Fallback to v-prefixed version
		pattern = fmt.Sprintf("%s-v%s.tgz", chartBase, cleanVersion)
		matches, err = filepath.Glob(filepath.Join(tempDir, pattern))
		if err != nil {
			return "", err
		}
	}
	
	logger.Debugf("Looking for downloaded chart using pattern %s (chart: %s, version: %s), found matches: %v", pattern, h.Chart, h.Version, matches)
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple chart archives found for %s (pattern %s): %v", h.Name, pattern, matches)
	}
	return matches[0], nil
}
