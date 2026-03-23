# seactl (SUSE Edge Airgap Command Line Interface)

[![Go](https://github.com/alknopfler/seactl/actions/workflows/go.yml/badge.svg)](https://github.com/alknopfler/seactl/actions/workflows/go.yml)


SUSE Edge Airgap Tool created to make the airgap process easier for SUSE Edge for telco deployments.

## Features

- Read the info from the airgap manifest file.
- Create a tarball for rke2 release tarball files (required to be used in capi airgap scenarios).
- Upload the helm-charts oci images defined in the release manifest to the private registry .
- Upload the containers images defined in the release manifest to the private registry.

## Requirements

- Helm 3 installed on the machine. You can install it using:

```shell
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

## Usage

Clone the repository and build the tool using the following command:

```shell
make compile
```

1. If your private registry is auth based, create your own registry auth file with the following format:

```txt
<username_bas64encoded>:<password_base64encoded>
```

for example you can generate both using
```
echo -n "myuser" | base64
echo -n "mypassword" | base64
```

2. If your private registry is using a self-signed certificate, create a CA certificate file and provide the path to the tool.

3. Rancher Apps charts require authentication. Create a Rancher Apps auth file with the same base64 `user:pass` format described above. See [SUSE Storage installation docs](https://documentation.suse.com/suse-edge/3.5/html/edge/components-suse-storage.html#id-installing-suse-storage).

The following command can be used to mirror the airgap artifacts

```bash
Usage:
seactl mirror [flags]

Flags:
-h, --help                       help for mirror
-i, --input string               Release manifest file
-k, --insecure                   Skip TLS verification in registry
-o, --output string              Output directory to store the tarball files
-a, --registry-authfile string   Registry Auth file with username:password base64 encoded
    --rancher-apps-authfile string     Rancher Apps registry auth file with username:password base64 encoded
-c, --registry-cacert string     Registry CA Certificate file
-r, --registry-url string        Registry URL
-d, --dryrun                     Dry run mode, only print the actions without executing them
-m, --release-mode string        Release mode, can be 'factory' or 'production' (default "factory"). Only used if release-version is provided.
-v, --release-version string     Release version, e.g. 3.4.0 (X.Y.Z). Start Binary Mode if provided.
    --debug                      Debug mode with more logs verbosity
```

## Modes of Operation

### Binary Mode

In this mode, you provide the `release-version` and `release-mode` flags. The tool will download the necessary manifests from the remote release source.

Example:
```bash
seactl mirror -v 3.4.0 -m factory -o /tmp/airgap --rancher-apps-authfile rancher-auth.txt -a registry-auth.txt -c /opt/certs/ca.crt -r myregistry:5000
```

### Container Mode (Local Files inside Release Container)

In this mode, you **omit** the `release-version` flag. The tool expects to find `/release_manifest.yaml` and `/release_images.yaml` in the local filesystem root (intended for containerized usage where these files are present).

Example:
```bash
seactl mirror -o /tmp/airgap --rancher-apps-authfile rancher-auth.txt -a registry-auth.txt -c /opt/certs/ca.crt -r myregistry:5000
```

Check the installed version:

```bash
seactl --version
```

## Examples

### Binary Mode Examples

```bash
seactl mirror -v 3.4.0 -m production -o /tmp/airgap --rancher-apps-authfile rancher-auth.txt -a registry-auth.txt -r myregistry:5000 --insecure --debug
```

```bash 
./seactl mirror -v 3.4.0 -m production -o ./tmp/airgap -r localhost:3000 -d true
```

### Container Mode Example

```bash
# This mode assumes /release_manifest.yaml and /release_images.yaml exist locally (e.g. inside the container)
seactl mirror -o /tmp/airgap --rancher-apps-authfile rancher-auth.txt -a registry-auth.txt -r myregistry:5000 --insecure --debug
```

### Proxy Support

```bash
export HTTPS_PROXY=http://10.X.X.X:3128
./seactl mirror -v 3.4.0 -m factory -o /tmp/airgap --rancher-apps-authfile rancher-auth.txt -a registry-auth.txt -c /opt/certs/ca.crt -r myregistry:5000
```

## Developer

### Versioning

- Update the version in the Makefile variable `VERSION` only.
- Build with `make build` or `make compile` to inject the version into the binary.
- Create a git tag with `make tag` (uses `v$(VERSION)`).

Follow semantic versioning for every change:

- Patch: bug fixes, no breaking changes.
- Minor: new features, backward compatible.
- Major: breaking changes.