package images

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/alknopfler/seactl/pkg/logger"
	"github.com/alknopfler/seactl/pkg/registry"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Images struct {
	Name           string
	Insecure       bool // If true, skip TLS verification
	reg            *registry.Registry
	rancherAppsReg *registry.Registry
	ImageRef       v1.Image
}

var (
	remoteImage = remote.Image
	remoteWrite = remote.Write
)

func New(name string, reg *registry.Registry, rancherAppsReg *registry.Registry) *Images {
	return &Images{
		Name:           name,
		reg:            reg,
		rancherAppsReg: rancherAppsReg,
	}
}

func (i *Images) Download() error {
	logger.Debugf("Starting to download image %s", i.Name)
	ref, err := name.ParseReference(i.Name)
	if err != nil {
		logger.Printf("failed to parse image reference %v", err)
		return err
	}

	logger.Debugf("Parsed reference: %v", ref)

	var remoteOpts []remote.Option
	if strings.HasPrefix(i.Name, "dp.apps.rancher.io") && i.rancherAppsReg != nil {
		authFile, err := i.rancherAppsReg.GetUserFromAuthFile()
		if err == nil {
			auth := &authn.Basic{
				Username: authFile[0],
				Password: authFile[1],
			}
			remoteOpts = append(remoteOpts, remote.WithAuth(auth))
			logger.Debugf("Using rancher apps authentication for %s", i.Name)
		} else {
			logger.Debugf("Failed to read rancher apps auth file: %v", err)
		}
	}

	img, err := remoteImage(ref, remoteOpts...)
	if err != nil {
		logger.Printf("pulling image %q: %v", img, err)
		return err
	}

	i.ImageRef = img
	logger.Printf("successfully pulled image %q", i.Name)
	return nil
}

func (i *Images) Verify() error {
	// Verify the image
	return nil
}

func (i *Images) Upload() error {
	srcRef, err := name.ParseReference(i.Name)
	if err != nil {
		return fmt.Errorf("parsing reference %q: %v", i.Name, err)
	}

	ref, err := i.buildTargetReference(srcRef)
	if err != nil {
		return fmt.Errorf("building target reference for %q: %v", i.Name, err)
	}

	opts, err := i.getRemoteOpts()
	if err != nil {
		return fmt.Errorf("getting remote options: %v", err)
	}

	logger.Printf("pushing image to %s", ref.String())
	err = remoteWrite(ref, i.ImageRef, opts...)
	if err != nil {
		return fmt.Errorf("pushing image %q: %v", i.ImageRef, err)
	}

	logger.Printf("successfully pushed image %q", i.Name)
	return nil
}

func (i *Images) buildTargetReference(src name.Reference) (name.Reference, error) {
	repoPath := src.Context().RepositoryStr()
	targetRepo := fmt.Sprintf("%s/mirror/%s", i.reg.RegistryURL, repoPath)

	switch ref := src.(type) {
	case name.Tag:
		return name.NewTag(fmt.Sprintf("%s:%s", targetRepo, ref.TagStr()), name.WeakValidation)
	case name.Digest:
		return name.NewDigest(fmt.Sprintf("%s@%s", targetRepo, ref.DigestStr()), name.WeakValidation)
	default:
		return name.ParseReference(targetRepo)
	}
}

func (i *Images) getRemoteOpts() ([]remote.Option, error) {
	// Create a custom HTTP transport
	tlsConfig := &tls.Config{}

	if i.Insecure {
		tlsConfig.InsecureSkipVerify = true
	} else if i.reg.RegistryCACert != "" {
		// Load CA certificate
		caCert, err := os.ReadFile(i.reg.RegistryCACert)
		if err != nil {
			return nil, fmt.Errorf("reading CA certificate: %v", err)
		}

		// Create a CA certificate pool
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	authFile, err := i.reg.GetUserFromAuthFile()
	if err != nil {
		return nil, fmt.Errorf("reading auth file: %v", err)
	}

	// Create a registry authenticator
	auth := &authn.Basic{
		Username: authFile[0],
		Password: authFile[1],
	}

	remoteOpts := []remote.Option{
		remote.WithTransport(transport),
		remote.WithAuth(auth),
	}

	// Remote options with custom HTTP client and authenticator
	return remoteOpts, nil
}
