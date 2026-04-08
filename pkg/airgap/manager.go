package airgap

import (
	"errors"
	"fmt"
	"github.com/alknopfler/seactl/pkg/logger"
	"os/exec"
	"strings"
	"sync"

	"github.com/TwiN/go-color"
	"github.com/alknopfler/seactl/pkg/config"
	"github.com/alknopfler/seactl/pkg/helm"
	"github.com/alknopfler/seactl/pkg/images"
	"github.com/alknopfler/seactl/pkg/registry"
	"github.com/alknopfler/seactl/pkg/rke2"
)

type Manager interface {
	Download() error
	Verify() error
	Upload() error
}

// CheckHelmCommand is assignable for testing
var CheckHelmCommand = func() error {
	if _, err := exec.LookPath("helm"); err != nil {
		return errors.New("Helm command not found in the system. You need to install it to continue")
	}
	return nil
}

// ReadAirgapManifestFunc is assignable for testing
var ReadAirgapManifestFunc = config.ReadAirgapManifest

const (
	susePrivateRegistryURL = "registry.suse.com"
	rancherAppsRegistryURL = "dp.apps.rancher.io"
)
// GenerateAirGapEnvironment is assignable for testing
var GenerateAirGapEnvironment = func(
	dryrun bool,
	releaseVersion, releaseMode,
	registryURL, registryAuthFile, rancherAppsAuthFile, susePrivateRegistryAuthFile, registryCACert,
	outputDirTarball string,
	insecure bool,
) error {
	fatalErrors := make(chan error)
	wgDone := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(3)

	releaseManifest, imagesManifest, err := ReadAirgapManifestFunc(releaseVersion, releaseMode)
	if err != nil {
		return err
	}

	reg := registry.New(registryAuthFile, registryURL, registryCACert, insecure)
	rancherAppsReg := registry.New(rancherAppsAuthFile, rancherAppsRegistryURL, "", false)
	var susePrivateRegistryReg *registry.Registry
	if susePrivateRegistryAuthFile != "" {
		susePrivateRegistryReg = registry.New(susePrivateRegistryAuthFile, susePrivateRegistryURL, "", false)
	}

	if !dryrun {
		if err := reg.RegistryLogin(); err != nil {
			return err
		}
		if err := rancherAppsReg.RegistryHelmLogin(); err != nil {
			return err
		}
		if susePrivateRegistryReg != nil {
			if err := susePrivateRegistryReg.RegistryHelmLogin(); err != nil {
				return err
			}
		}
		if err := reg.RegistryHelmLogin(); err != nil {
			return err
		}

		// Attempt to create the 'edge' project on Harbor specifically, ignores errors if not Harbor
		reg.CreateHarborProject("mirror")
	}

	go func() {
		err := generateRKE2Artifacts(dryrun, releaseManifest, outputDirTarball)
		if err != nil {
			fatalErrors <- err
		}
		wg.Done()
	}()

	go func() {
		err = generateHelmArtifacts(dryrun, releaseManifest, reg, rancherAppsReg, susePrivateRegistryReg)
		if err != nil {
			fatalErrors <- err
		}
		wg.Done()
	}()

	go func() {
		err = generateImagesArtifacts(dryrun, imagesManifest, reg, rancherAppsReg, susePrivateRegistryReg)
		if err != nil {
			fatalErrors <- err
		}
		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		return nil
	case err = <-fatalErrors:
		close(fatalErrors)
		logger.Fatal("Error found running the program: ", err)
		return err
	}
}

func shouldSkipSUSEPrivateRegistryChart(chart string, susePrivateRegistryReg *registry.Registry) bool {
	if susePrivateRegistryReg != nil {
		return false
	}

	return chart == "oci://registry.suse.com/private-registry/private-registry-helm"
}

func shouldSkipSUSEPrivateRegistryImage(image string, susePrivateRegistryReg *registry.Registry) bool {
	if susePrivateRegistryReg != nil {
		return false
	}

	return strings.HasPrefix(image, "registry.suse.com/private-registry/harbor")
}

func generateRKE2Artifacts(dryrun bool, airgapManifest *config.ReleaseManifest, outputDirTarball string) error {
	r := rke2.New(airgapManifest.Spec.Components.Kubernetes.Rke2.Version, outputDirTarball)
	if !dryrun {
		if err := r.Download(); err != nil {
			return err
		}
		if err := r.Verify(); err != nil {
			return err
		}
	} else {
		logger.Println("Dry run mode enabled, skipping download and verification of RKE2 images.")
	}
	logger.Println(color.InGreen("RKE2 Images downloaded and verified successfully! you can find them in: " + outputDirTarball))
	return nil
}

func generateHelmArtifacts(dryrun bool, releaseManifest *config.ReleaseManifest, reg *registry.Registry, rancherAppsReg *registry.Registry, susePrivateRegistryReg *registry.Registry) error {
	for _, value := range releaseManifest.Spec.Components.Workloads.Helm {
		if shouldSkipSUSEPrivateRegistryChart(value.Chart, susePrivateRegistryReg) {
			logger.Printf("Skipping chart %s because --suse-private-registry-authfile was not provided", value.Chart)
			continue
		}

		h := helm.New(value.ReleaseName, value.Version, value.Chart, value.Repository, reg)
		if !dryrun {
			if err := h.Download(); err != nil {
				return err
			}
			if err := h.Verify(); err != nil {
				return err
			}
			if reg.RegistryInsecure {
				h.Insecure = true
			}
			if err := h.Upload(); err != nil {
				return err
			}
			logger.Printf(color.InGreen("Helm chart %s prepared and uploaded successfully!\n"), value.ReleaseName)
		} else {
			logger.Println("DryRun mode - Helm Chart Info:")
			logger.Printf("\nName: %s\nVersion: %s\nURL: %s\nChart: %s\n", h.Name, h.Version, h.URL, h.Chart)
		}
	}
	logger.Println(color.InGreen("Helm Chart artifacts pre-loaded in registry successfully!"))
	return nil
}

func generateImagesArtifacts(dryrun bool, imagesManifest *config.ImagesManifest, reg *registry.Registry, rancherAppsReg *registry.Registry, susePrivateRegistryReg *registry.Registry) error {
	for _, value := range imagesManifest.Images {
		if shouldSkipSUSEPrivateRegistryImage(value.Name, susePrivateRegistryReg) {
			logger.Printf("Skipping image %s because --suse-private-registry-authfile was not provided", value.Name)
			continue
		}

		img := images.New(value.Name, reg, rancherAppsReg, susePrivateRegistryReg)
		if !dryrun {
			if err := img.Download(); err != nil {
				return err
			}
			if err := img.Verify(); err != nil {
				return err
			}
			if reg.RegistryInsecure {
				img.Insecure = true
			}
			fmt.Println("Image Info:")
			fmt.Printf("Name: %s\n", img.Name)
			if err := img.Upload(); err != nil {
				return err
			}
		} else {
			logger.Println("DryRun mode - Image Info:")
			logger.Printf("\nName: %s\n", img.Name)
		}
	}
	logger.Println(color.InGreen("Images artifacts pre-loaded in registry successfully!"))
	return nil
}
