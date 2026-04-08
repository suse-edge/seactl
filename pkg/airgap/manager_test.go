package airgap

import (
	"errors"
	"testing"

	"github.com/alknopfler/seactl/pkg/config"
	"github.com/alknopfler/seactl/pkg/registry"
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

	err := GenerateAirGapEnvironment(true, "v1.0.0", "factory", "url", "auth", "rancher-auth", "suse-auth", "ca", "/tmp", true)
	assert.NoError(t, err)
}

func TestGenerateAirGapEnvironment_ErrorFromManifest(t *testing.T) {
	ReadAirgapManifestFunc = func(version, mode string) (*config.ReleaseManifest, *config.ImagesManifest, error) {
		return nil, nil, errors.New("failed manifest")
	}
	err := GenerateAirGapEnvironment(true, "v1.0.0", "factory", "auth", "auth", "rancher-auth", "suse-auth", "ca", "/tmp", true)
	assert.Error(t, err)
}

func TestGenerateRKE2Artifacts_NotDryRun(t *testing.T) {
	manifest, _, _ := fakeReleaseManifest()
	err := generateRKE2Artifacts(false, manifest, "/tmp")
	assert.Error(t, err)
}

func TestGenerateHelmArtifacts_NotDryRun(t *testing.T) {
	manifest, _, _ := fakeReleaseManifest()
	err := generateHelmArtifacts(false, manifest, &registry.Registry{}, &registry.Registry{}, nil)
	assert.Error(t, err)
}

func TestGenerateImagesArtifacts_NotDryRun(t *testing.T) {
	_, imagesManifest, _ := fakeReleaseManifest()
	err := generateImagesArtifacts(false, imagesManifest, &registry.Registry{}, &registry.Registry{}, nil)
	assert.Error(t, err)
}

func TestShouldSkipSUSEPrivateRegistryChart(t *testing.T) {
	assert.True(t, shouldSkipSUSEPrivateRegistryChart("oci://registry.suse.com/private-registry/private-registry-helm", nil))
	assert.False(t, shouldSkipSUSEPrivateRegistryChart("oci://registry.suse.com/private-registry/private-registry-helm", &registry.Registry{}))
	assert.False(t, shouldSkipSUSEPrivateRegistryChart("oci://registry.opensuse.org/isv/suse/edge/factory/charts/metallb", nil))
}

func TestShouldSkipSUSEPrivateRegistryImage(t *testing.T) {
	assert.True(t, shouldSkipSUSEPrivateRegistryImage("registry.suse.com/private-registry/harbor-core:1.1.1-1.19", nil))
	assert.False(t, shouldSkipSUSEPrivateRegistryImage("registry.suse.com/private-registry/harbor-core:1.1.1-1.19", &registry.Registry{}))
	assert.False(t, shouldSkipSUSEPrivateRegistryImage("registry.suse.com/rancher/cluster-api-provider-rke2-bootstrap:v0.21.1", nil))
}
