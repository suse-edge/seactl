package registry

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/alknopfler/seactl/pkg/logger"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	http_auth "github.com/josegomezr/go-http-auth-challenge"
)

type Registry struct {
	RegistryAuthFile string
	RegistryURL      string
	RegistryCACert   string
	RegistryInsecure bool
}

type authTransport struct {
	base     http.RoundTripper
	username string
	password string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("Www-Authenticate")
		if authHeader != "" {
			challenges, errParse := http_auth.ParseChallenges(authHeader)
			if errParse == nil && len(challenges) > 0 {
				challenge := challenges[0]
				if strings.EqualFold(challenge.Scheme, "Basic") {
					resp.Body.Close()
					reqCopy := req.Clone(req.Context())
					reqCopy.SetBasicAuth(t.username, t.password)
					return t.base.RoundTrip(reqCopy)
				}
			}
		}
	}
	return resp, nil
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
	var args []string
	args = append(args, "registry", "login", r.RegistryURL)

	auth, err := r.GetUserFromAuthFile()
	if err != nil {
		logger.Printf("failed to read auth file for %s: %s", r.RegistryURL, err)
	} else if auth[0] != "" && auth[1] != "" {
		args = append(args, "--username", auth[0], "--password", auth[1])
	}

	if r.RegistryInsecure {
		args = append(args, "--insecure")
	} else if r.RegistryCACert != "" {
		args = append(args, "--ca-file", r.RegistryCACert)
	}
	cmd := execCommand("helm", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if cmd.Env == nil {
		cmd.Env = append(os.Environ(),
			"HELM_CONFIG_HOME=/tmp/.helm-config",
			"HELM_CACHE_HOME=/tmp/.helm-cache",
			"HELM_DATA_HOME=/tmp/.helm-data",
		)
	} else {
		cmd.Env = append(cmd.Env,
			"HELM_CONFIG_HOME=/tmp/.helm-config",
			"HELM_CACHE_HOME=/tmp/.helm-cache",
			"HELM_DATA_HOME=/tmp/.helm-data",
		)
	}
	err = cmd.Run()
	if err != nil {
		logger.Printf("failed to login to the helm registry %s: %s — %s", r.RegistryURL, err, stderr.String())
		return fmt.Errorf("failed to login to helm registry %s: %w — %s", r.RegistryURL, err, stderr.String())
	}
	logger.Printf("successfully logged in to the helm registry %s", r.RegistryURL)
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
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}

	authFileInfo, err := r.GetUserFromAuthFile()
	if err != nil {
		return fmt.Errorf("failed to get user credentials from authFile: %w", err)
	}

	auth := &authn.Basic{
		Username: authFileInfo[0],
		Password: authFileInfo[1],
	}

	transportWithAuth := &authTransport{
		base:     transport,
		username: authFileInfo[0],
		password: authFileInfo[1],
	}

	// Create an HTTP client with the custom transport
	client := &http.Client{Transport: transportWithAuth}

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

	logger.Printf("successfully authenticated to registry %q", r.RegistryURL)
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

func (r *Registry) CreateHarborProject(projectName string) error {
	tlsConfig := &tls.Config{}

	if r.RegistryInsecure {
		tlsConfig.InsecureSkipVerify = true
	} else if r.RegistryCACert != "" {
		caCert, err := os.ReadFile(r.RegistryCACert)
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.RootCAs = caCertPool
	}

	transport := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: tlsConfig,
	}
	client := &http.Client{Transport: transport}

	payload := map[string]interface{}{
		"project_name": projectName,
		"public":       true,
	}
	body, _ := json.Marshal(payload)

	scheme := "https"
	if r.RegistryInsecure && !strings.Contains(r.RegistryURL, "443") {
		// Default to https, Harbor forces https but user might bypass
	}

	baseURL := r.RegistryURL
	if !strings.HasPrefix(baseURL, "http") {
		baseURL = fmt.Sprintf("%s://%s", scheme, baseURL)
	}
	url := fmt.Sprintf("%s/api/v2.0/projects", baseURL)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	authFileInfo, err := r.GetUserFromAuthFile()
	if err == nil {
		req.SetBasicAuth(authFileInfo[0], authFileInfo[1])
	}

	logger.Debugf("Attempting to project '%s' using Harbor API at %s", projectName, url)
	resp, err := client.Do(req)
	if err != nil {
		logger.Debugf("Failed to contact registry API to create project (likely not Harbor or network issue): %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		logger.Debugf("Registry API returned 404, assuming standard Docker Registry, skipping project creation.")
		return nil
	}

	if resp.StatusCode == http.StatusCreated {
		logger.Printf("Successfully created Harbor project: %s", projectName)
	} else if resp.StatusCode == http.StatusConflict {
		logger.Debugf("Harbor project '%s' already exists.", projectName)
	} else {
		logger.Debugf("Harbor project creation returned unexpected status: %s", resp.Status)
	}
	return nil
}
