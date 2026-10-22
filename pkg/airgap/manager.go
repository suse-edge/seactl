package airgap

import (
	"errors"
	"fmt"
	"github.com/alknopfler/seactl/pkg/logger"
	"os/exec"
	"strings"
	"sync"
	"time"

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

// phaseResult tallies the outcome of one of the three mirroring phases (RKE2, Helm, Images) for the final summary
type phaseResult struct {
	Tag       string
	Succeeded int
	Skipped   int
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
	skipRke2, skipHelm, skipImages bool,
) error {
	fatalErrors := make(chan error)
	wgDone := make(chan bool)
	var wg sync.WaitGroup
	wg.Add(3)

	startTime := time.Now()

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

	var rke2Result, helmResult, imagesResult phaseResult

	go func() {
		defer wg.Done()
		if skipRke2 {
			rke2Result = phaseResult{Tag: "[RKE2]", Skipped: 1}
			logger.Println(color.InYellow(logger.StatusFmt("[RKE2]", "⊘", "Skipping RKE2 artifacts download as requested by --skip-rke2")))
			return
		}
		result, err := generateRKE2Artifacts(dryrun, releaseManifest, outputDirTarball)
		rke2Result = result
		if err != nil {
			fatalErrors <- err
		}
	}()

	go func() {
		defer wg.Done()
		if skipHelm {
			helmResult = phaseResult{Tag: "[HELM]", Skipped: 1}
			logger.Println(color.InYellow(logger.StatusFmt("[HELM]", "⊘", "Skipping Helm chart artifacts download as requested by --skip-helm")))
			return
		}
		result, err := generateHelmArtifacts(dryrun, releaseManifest, reg, rancherAppsReg, susePrivateRegistryReg)
		helmResult = result
		if err != nil {
			fatalErrors <- err
		}
	}()

	go func() {
		defer wg.Done()
		if skipImages {
			imagesResult = phaseResult{Tag: "[IMAGES]", Skipped: 1}
			logger.Println(color.InYellow(logger.StatusFmt("[IMAGES]", "⊘", "Skipping images artifacts download as requested by --skip-images")))
			return
		}
		result, err := generateImagesArtifacts(dryrun, imagesManifest, reg, rancherAppsReg, susePrivateRegistryReg)
		imagesResult = result
		if err != nil {
			fatalErrors <- err
		}
	}()

	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		printSummary(time.Since(startTime), rke2Result, helmResult, imagesResult)
		return nil
	case err = <-fatalErrors:
		close(fatalErrors)
		return err
	}
}

func printSummary(duration time.Duration, results ...phaseResult) {
	logger.Println(color.InCyan("▶ Summary"))
	for _, r := range results {
		symbol, text := formatPhaseResult(r)
		logger.Println(logger.StatusFmt(r.Tag, symbol, text))
	}
	logger.Printf("%-*s\t\t%s\n", logger.StatusTagWidth, "Duration:", duration.Round(time.Second))
}

func formatPhaseResult(r phaseResult) (symbol, text string) {
	if r.Succeeded == 0 && r.Skipped > 0 {
		return color.InYellow("⊘"), color.InYellow(fmt.Sprintf("skipped (%d)", r.Skipped))
	}
	text = fmt.Sprintf("%d succeeded", r.Succeeded)
	if r.Skipped > 0 {
		text += fmt.Sprintf(", %d skipped", r.Skipped)
	}
	return color.InGreen("✔"), color.InGreen(text)
}

func tally(result *phaseResult, skipped bool) {
	if skipped {
		result.Skipped++
	} else {
		result.Succeeded++
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

func generateRKE2Artifacts(dryrun bool, airgapManifest *config.ReleaseManifest, outputDirTarball string) (phaseResult, error) {
	result := phaseResult{Tag: "[RKE2]"}
	r := rke2.New(airgapManifest.Spec.Components.Kubernetes.Rke2.Version, outputDirTarball)
	if !dryrun {
		if err := r.Download(); err != nil {
			logger.Printf(color.InRed(logger.StatusFmt("[RKE2]", "✖", "failed to download RKE2 artifacts: %v\n")), err)
			return result, err
		}
		if err := r.Verify(); err != nil {
			logger.Printf(color.InRed(logger.StatusFmt("[RKE2]", "✖", "failed to verify RKE2 artifacts: %v\n")), err)
			return result, err
		}
	} else {
		logger.Println("[RKE2] Dry run mode enabled, skipping download and verification of RKE2 images.")
	}
	logger.Println(color.InGreen(logger.StatusFmt("[RKE2]", "✔", "RKE2 Images downloaded and verified successfully! you can find them in: "+outputDirTarball)))
	result.Succeeded = 1
	return result, nil
}

func generateHelmArtifacts(dryrun bool, releaseManifest *config.ReleaseManifest, reg *registry.Registry, rancherAppsReg *registry.Registry, susePrivateRegistryReg *registry.Registry) (phaseResult, error) {
	result := phaseResult{Tag: "[HELM]"}
	for _, value := range releaseManifest.Spec.Components.Workloads.Helm {
		skipped, err := processHelmChart(dryrun, value.ReleaseName, value.Chart, value.Version, value.Repository, reg, susePrivateRegistryReg)
		if err != nil {
			return result, err
		}
		tally(&result, skipped)

		for _, dependencyChart := range value.DependencyCharts {
			skipped, err := processHelmChart(dryrun, dependencyChart.ReleaseName, dependencyChart.Chart, dependencyChart.Version, dependencyChart.Repository, reg, susePrivateRegistryReg)
			if err != nil {
				return result, err
			}
			tally(&result, skipped)
		}

		for _, addonChart := range value.AddonCharts {
			skipped, err := processHelmChart(dryrun, addonChart.ReleaseName, addonChart.Chart, addonChart.Version, addonChart.Repository, reg, susePrivateRegistryReg)
			if err != nil {
				return result, err
			}
			tally(&result, skipped)
		}
	}
	logger.Println(color.InGreen(logger.StatusFmt("[HELM]", "✔", "Helm Chart artifacts pre-loaded in registry successfully!")))
	return result, nil
}

func processHelmChart(dryrun bool, releaseName, chart, version, repository string, reg *registry.Registry, susePrivateRegistryReg *registry.Registry) (skipped bool, err error) {
	if shouldSkipSUSEPrivateRegistryChart(chart, susePrivateRegistryReg) {
		logger.Printf(color.InYellow(logger.StatusFmt("[HELM]", "⊘", "Skipping chart %s because --suse-private-registry-authfile was not provided")), chart)
		return true, nil
	}

	h := helm.New(releaseName, version, chart, repository, reg)
	if !dryrun {
		if err := h.Download(); err != nil {
			logger.Printf(color.InRed(logger.StatusFmt("[HELM]", "✖", "failed to download chart %s: %v\n")), releaseName, err)
			return false, err
		}
		if err := h.Verify(); err != nil {
			logger.Printf(color.InRed(logger.StatusFmt("[HELM]", "✖", "failed to verify chart %s: %v\n")), releaseName, err)
			return false, err
		}
		if reg.RegistryInsecure {
			h.Insecure = true
		}
		if err := h.Upload(); err != nil {
			logger.Printf(color.InRed(logger.StatusFmt("[HELM]", "✖", "failed to upload chart %s: %v\n")), releaseName, err)
			return false, err
		}
		logger.Printf(color.InGreen(logger.StatusFmt("[HELM]", "✔", "Helm chart %s prepared and uploaded successfully!\n")), releaseName)
	} else {
		logger.Println("[HELM] DryRun mode - Helm Chart Info:")
		logger.Printf("\nName: %s\nVersion: %s\nURL: %s\nChart: %s\n", h.Name, h.Version, h.URL, h.Chart)
	}
	return false, nil
}

func generateImagesArtifacts(dryrun bool, imagesManifest *config.ImagesManifest, reg *registry.Registry, rancherAppsReg *registry.Registry, susePrivateRegistryReg *registry.Registry) (phaseResult, error) {
	result := phaseResult{Tag: "[IMAGES]"}
	for _, value := range imagesManifest.Images {
		if shouldSkipSUSEPrivateRegistryImage(value.Name, susePrivateRegistryReg) {
			logger.Printf(color.InYellow(logger.StatusFmt("[IMAGES]", "⊘", "Skipping image %s because --suse-private-registry-authfile was not provided")), value.Name)
			result.Skipped++
			continue
		}

		img := images.New(value.Name, reg, rancherAppsReg, susePrivateRegistryReg)
		if !dryrun {
			if err := img.Download(); err != nil {
				logger.Printf(color.InRed(logger.StatusFmt("[IMAGES]", "✖", "failed to download image %s: %v\n")), value.Name, err)
				return result, err
			}
			if err := img.Verify(); err != nil {
				logger.Printf(color.InRed(logger.StatusFmt("[IMAGES]", "✖", "failed to verify image %s: %v\n")), value.Name, err)
				return result, err
			}
			if reg.RegistryInsecure {
				img.Insecure = true
			}
			if err := img.Upload(); err != nil {
				logger.Printf(color.InRed(logger.StatusFmt("[IMAGES]", "✖", "failed to upload image %s: %v\n")), value.Name, err)
				return result, err
			}
			logger.Printf(color.InGreen(logger.StatusFmt("[IMAGES]", "✔", "Image %s prepared and uploaded successfully!\n")), img.Name)
		} else {
			logger.Println("[IMAGES] DryRun mode - Image Info:")
			logger.Printf("\nName: %s\n", img.Name)
		}
		result.Succeeded++
	}
	logger.Println(color.InGreen(logger.StatusFmt("[IMAGES]", "✔", "Images artifacts pre-loaded in registry successfully!")))
	return result, nil
}
