package airgap

import (
	"errors"
	"testing"

	"github.com/alknopfler/seactl/pkg/config"
	"github.com/stretchr/testify/assert"
)

func fakeReleaseManifest() (*config.ReleaseManifest, *config.ImagesManifest, error) {
	manifest := &config.ReleaseManifest{}
	manifest.Spec.Components.Workloads.Helm = []struct {
		PrettyName  string "yaml:\"prettyName\""
		ReleaseName string "yaml:\"releaseName\""
		Chart       string "yaml:\"chart\""
		Version     string "yaml:\"version\""
		Repository  string "yaml:\"repository,omitempty\""
		Values      struct {
			PostDelete struct {
				Enabled bool "yaml:\"enabled\""
			} "yaml:\"postDelete\""
		} "yaml:\"values,omitempty\""
		DependencyCharts []struct {
			ReleaseName string "yaml:\"releaseName\""
			Chart       string "yaml:\"chart\""
			Version     string "yaml:\"version\""
			Repository  string "yaml:\"repository\""
		} "yaml:\"dependencyCharts,omitempty\""
		AddonCharts []struct {
			ReleaseName string "yaml:\"releaseName\""
			Chart       string "yaml:\"chart\""
			Version     string "yaml:\"version\""
		} "yaml:\"addonCharts,omitempty\""
	}{{ReleaseName: "test-chart", Chart: "oci://test/chart", Version: "1.0.0"}}

	imagesManifest := &config.ImagesManifest{
		Images: []struct {
			Name string `yaml:"name"`
		}{{Name: "test-image"}},
	}

	return manifest, imagesManifest, nil
}

func TestGenerateAirGapEnvironment_DryRun(t *testing.T) {
	ReadAirgapManifestFunc = func(version, mode string) (*config.ReleaseManifest, *config.ImagesManifest, error) {
		return fakeReleaseManifest()
	}

	err := GenerateAirGapEnvironment(true, "v1.0.0", "factory", "url", "auth", "rancher-auth", "ca", "/tmp", true)
	assert.NoError(t, err)
}

func TestGenerateAirGapEnvironment_ErrorFromManifest(t *testing.T) {
	ReadAirgapManifestFunc = func(version, mode string) (*config.ReleaseManifest, *config.ImagesManifest, error) {
		return nil, nil, errors.New("failed manifest")
	}
	err := GenerateAirGapEnvironment(true, "v1.0.0", "factory", "auth", "auth", "rancher-auth", "ca", "/tmp", true)
	assert.Error(t, err)
}
