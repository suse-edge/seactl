package cmd

import (
	"fmt"

	"github.com/alknopfler/seactl/pkg/airgap"
	"github.com/alknopfler/seactl/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	releaseVersion      string
	releaseMode         string
	registryAuthFile    string
	rancherAppsAuthFile string
	registryURL         string
	registryCACert      string
	registryInsecure    bool
	outputDirTarball    string
	dryRun              bool
	debug               bool
)

func NewAirGapCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "generate",
		Short: "Command to generate the air-gap artifacts from the airgap manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Set global debug flag
			logger.Debug = debug

			// Check helm
			if err := airgap.CheckHelmCommand(); err != nil {
				return err
			}

			// Validate release mode
			if releaseMode != "factory" && releaseMode != "production" {
				return fmt.Errorf("invalid value for --release-mode: %s, allowed: 'factory' or 'production'", releaseMode)
			}

			// Validate release version format X.Y.Z
			if releaseVersion == "" || len(releaseVersion) < 5 || releaseVersion[1] != '.' || releaseVersion[3] != '.' {
				return fmt.Errorf("invalid release version format: %s, expected format X.Y.Z", releaseVersion)
			}

			// Call airgap generation
			return airgap.GenerateAirGapEnvironment(
				dryRun, releaseVersion, releaseMode,
				registryURL, registryAuthFile, rancherAppsAuthFile, registryCACert,
				outputDirTarball, registryInsecure,
			)
		},
	}

	flags := c.Flags()
	flags.StringVarP(&releaseVersion, "release-version", "v", "", "SUSE Edge release version (X.Y.Z)")
	flags.StringVarP(&releaseMode, "release-mode", "m", "factory", "Release mode: factory or production")
	flags.StringVarP(&registryURL, "registry-url", "r", "", "Registry URL")
	flags.StringVarP(&registryCACert, "registry-cacert", "c", "", "Registry CA Certificate")
	flags.StringVarP(&registryAuthFile, "registry-authfile", "a", "", "Registry Auth file (base64 user:pass)")
	flags.StringVar(&rancherAppsAuthFile, "rancher-apps-authfile", "", "Rancher Apps registry auth file (base64 user:pass)")
	flags.BoolVarP(&registryInsecure, "insecure", "k", false, "Skip TLS verification")
	flags.StringVarP(&outputDirTarball, "output", "o", "", "Output directory for tarball files")
	flags.BoolVarP(&dryRun, "dry-run", "d", false, "Dry run mode")
	flags.BoolVar(&debug, "debug", false, "Enable debug logging")

	// Required flags
	c.MarkFlagRequired("release-version")
	c.MarkFlagRequired("output")
	c.MarkFlagRequired("registry-url")
	c.MarkFlagRequired("rancher-apps-authfile")

	return c
}
