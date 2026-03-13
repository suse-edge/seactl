package registry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Registry struct {
	RegistryAuthFile string
	RegistryURL      string
	RegistryCACert   string
	RegistryInsecure bool
}

var (
	execCommand   = exec.Command
	remoteCatalog = remote.Catalog
)

func New(registryAuthFile, registryURL, registryCACert string, insecure bool) *Registry {
	return &Registry{
		RegistryAuthFile: registryAuthFile,
		RegistryURL:      registryURL,
		RegistryCACert:   registryCACert,
		RegistryInsecure: insecure,
	}
}

func (r *Registry) RegistryHelmLogin() error {
	var args, auth []string
	args = append(args, "registry", "login", r.RegistryURL)

	auth, err := r.GetUserFromAuthFile()
	if err == nil {
		if auth[0] != "" && auth[1] != "" {
			args = append(args, "--username", auth[0], "--password", auth[1])
		}
	}

	if r.RegistryInsecure {
		args = append(args, "--insecure")
	} else if r.RegistryCACert != "" {
		args = append(args, "--ca-file", r.RegistryCACert)
	}
	cmd := execCommand("helm", args...)
	err = cmd.Run()

	if err != nil {
		log.Printf("failed to login to the registry: %s", err)
		return err
	}
	log.Printf("successfully logged in to the helm registry %s", r.RegistryURL)
	return nil
}

func (r *Registry) RegistryLogin() error {
	ctx := context.Background()
	tlsConfig := &tls.Config{}

	if r.RegistryInsecure {
		tlsConfig.InsecureSkipVerify = true
	} else if r.RegistryCACert != "" {
		// Load CA certificate
		caCert, err := os.ReadFile(r.RegistryCACert)
		if err != nil {
			return err
		}

		// Create a CA certificate pool
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	// Create an HTTP client with the custom transport
	client := &http.Client{Transport: transport}

	authFileInfo, err := r.GetUserFromAuthFile()
	if err != nil {
		return fmt.Errorf("failed to get user credentials from authFile: %w", err)
	}

	auth := &authn.Basic{
		Username: authFileInfo[0],
		Password: authFileInfo[1],
	}

	// Remote options with custom HTTP client and authenticator
	remoteOpts := remote.WithTransport(client.Transport)
	authOpts := remote.WithAuth(auth)

	ref, err := name.NewRegistry(r.RegistryURL)
	if err != nil {
		return fmt.Errorf("invalid registry %q: %v", r.RegistryURL, err)
	}

	_, err = remoteCatalog(ctx, ref, remoteOpts, authOpts)
	if err != nil {
		return fmt.Errorf("error pinging registry %q: %v", r.RegistryURL, err)
	}

	log.Printf("successfully authenticated to registry %q", r.RegistryURL)
	return nil
}

func (r *Registry) GetUserFromAuthFile() ([]string, error) {
	// Read the content of the file
	data, err := os.ReadFile(r.RegistryAuthFile)
	if err != nil {
		return []string{}, fmt.Errorf("failed to read auth file: %w", err)
	}

	// Split the decoded string by ":" to get user and pass
	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) != 2 {
		return []string{}, fmt.Errorf("decoded data does not contain user:pass format")
	}

	// Decode the base64 content
	user, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return []string{}, fmt.Errorf("failed to decode base64 user: %w", err)
	}
	pass, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return []string{}, fmt.Errorf("failed to decode base64 password: %w", err)
	}

	// Return the user part
	return []string{string(user), string(pass)}, nil
}
