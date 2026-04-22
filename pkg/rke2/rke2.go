package rke2

import (
	"fmt"
	"github.com/alknopfler/seactl/pkg/logger"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	RKE2ReleaseURL = "https://prime.ribs.rancher.io/rke2/"
	RKE2URL        = "https://get.rke2.io"
)

var (
	listRKE2Images = map[string]string{
		"RKE2ImagesLinux":   "rke2-images.linux-amd64.tar.zst",
		"RKE2ImagesCalico":  "rke2-images-calico.linux-amd64.tar.zst",
		"RKE2ImagesFlannel": "rke2-images-flannel.linux-amd64.tar.zst",
		"RKE2ImagesCilium":  "rke2-images-cilium.linux-amd64.tar.zst",
		"RKE2ImagesCanal":   "rke2-images-canal.linux-amd64.tar.zst",
		"RKE2ImagesMultus":  "rke2-images-multus.linux-amd64.tar.zst",
		"RKE2ImagesCore":    "rke2-images-core.linux-amd64.tar.zst",
		"RKE2Linux":         "rke2.linux-amd64.tar.gz",
		"RKE2SHA256":        "sha256sum-amd64.txt",
	}
)

type RKE2 struct {
	Version          string
	OutputDirTarball string
	ReleaseURL       string
}

func New(version, outputDirTarball string) *RKE2 {
	return &RKE2{
		Version:          version,
		OutputDirTarball: outputDirTarball,
		ReleaseURL:       RKE2ReleaseURL,
	}
}

func (r *RKE2) Download() error {
	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(r.OutputDirTarball, os.ModePerm); err != nil {
		logger.Printf("failed to create destination directory: %v", err)
		return err
	}

	// Download the install.sh script
	if getFileFromURL(RKE2URL, "install.sh", ensureTrailingSlash(r.OutputDirTarball)) != nil {
		return fmt.Errorf("failed to download the install.sh script")
	}

	// Download the tarball files for the current release
	for _, image := range listRKE2Images {
		if getFileFromURL(r.ReleaseURL+replaceVersionLink(r.Version)+"/"+image, image, ensureTrailingSlash(r.OutputDirTarball)) != nil {
			return fmt.Errorf("failed to download the file: %s", image)
		}
	}
	return nil
}

func (r *RKE2) Verify() error {
	// verify if all images have been downloaded successfully
	for _, image := range listRKE2Images {
		filePath := filepath.Join(ensureTrailingSlash(r.OutputDirTarball), image)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			logger.Printf("file does not exist: %s", filePath)
			return err
		}
		logger.Printf("Image verified successfully: %s", filePath)
	}
	return nil
}

func (r *RKE2) Upload() error {
	// Upload the tarball files to the registry
	// TODO: implement me if needed (prepared if we change the airgap with rke2-capi-provider to use registry instead of artifacts)
	return nil
}

func replaceVersionLink(version string) string {
	return strings.ReplaceAll(version, "+", "%2B")
}

func ensureTrailingSlash(dir string) string {
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return dir
}

func getFileFromURL(url, filename, filePath string) error {
	resp, err := http.Get(url)
	if err != nil {
		logger.Printf("failed to download the file %s, %v", filename, err)
		return err
	}

	if resp.StatusCode != http.StatusOK {
		logger.Printf("failed to download the file %s from %s: HTTP status %s", filename, url, resp.Status)
		return fmt.Errorf("failed to download the file %s from %s: HTTP status %s", filename, url, resp.Status)
	}

	// Create the file
	out, err := os.Create(filePath + filename)

	if err != nil {
		logger.Printf("failed to create file: %v", err)
		return err
	}
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		logger.Printf("failed to save file: %v", err)
		return err
	}
	logger.Printf("File %s downloaded successfully to %s", filename, filePath)

	defer resp.Body.Close()
	defer out.Close()
	return nil
}
