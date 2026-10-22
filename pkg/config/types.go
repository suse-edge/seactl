package config

// ReleaseManifest is the struct that represents the airgap manifest from the release container
type ReleaseManifest struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		ReleaseVersion string `yaml:"releaseVersion"`
		Components     struct {
			Kubernetes struct {
				K3S struct {
					Version        string `yaml:"version"`
					CoreComponents []struct {
						Name       string `yaml:"name"`
						Version    string `yaml:"version,omitempty"`
						Type       string `yaml:"type"`
						Containers []struct {
							Name  string `yaml:"name"`
							Image string `yaml:"image"`
						} `yaml:"containers,omitempty"`
					} `yaml:"coreComponents"`
				} `yaml:"k3s"`
				Rke2 struct {
					Version        string `yaml:"version"`
					CoreComponents []struct {
						Name    string `yaml:"name"`
						Version string `yaml:"version"`
						Type    string `yaml:"type"`
					} `yaml:"coreComponents"`
				} `yaml:"rke2"`
			} `yaml:"kubernetes"`
			OperatingSystem struct {
				Version        string   `yaml:"version"`
				ZypperID       string   `yaml:"zypperID"`
				CpeScheme      string   `yaml:"cpeScheme"`
				PrettyName     string   `yaml:"prettyName"`
				SupportedArchs []string `yaml:"supportedArchs"`
			} `yaml:"operatingSystem"`
			Workloads struct {
				Helm []struct {
					PrettyName  string `yaml:"prettyName"`
					ReleaseName string `yaml:"releaseName"`
					Chart       string `yaml:"chart"`
					Version     string `yaml:"version"`
					Repository  string `yaml:"repository,omitempty"`
					Values      struct {
						PostDelete struct {
							Enabled bool `yaml:"enabled"`
						} `yaml:"postDelete"`
					} `yaml:"values,omitempty"`
					DependencyCharts []struct {
						ReleaseName string `yaml:"releaseName"`
						Chart       string `yaml:"chart"`
						Version     string `yaml:"version"`
						Repository  string `yaml:"repository"`
					} `yaml:"dependencyCharts,omitempty"`
					AddonCharts []struct {
						ReleaseName string `yaml:"releaseName"`
						Chart       string `yaml:"chart"`
						Version     string `yaml:"version"`
						Repository  string `yaml:"repository"`
					} `yaml:"addonCharts,omitempty"`
				} `yaml:"helm"`
			} `yaml:"workloads"`
		} `yaml:"components"`
	} `yaml:"spec"`
}

// ImagesManifest is the struct that represents the images manifest from the release container
type ImagesManifest struct {
	Images []struct {
		Name string `yaml:"name"`
	} `yaml:"images"`
}
