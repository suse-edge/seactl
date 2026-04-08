# seactl (SUSE Edge Airgap Command Line Interface)

[![Go](https://github.com/alknopfler/seactl/actions/workflows/go.yml/badge.svg)](https://github.com/alknopfler/seactl/actions/workflows/go.yml)


SUSE Edge Airgap Tool created to make the mirroring process to populate a registry easier for SUSE Edge for disconnected deployments.

## Features

- Read the info from the release manifest file (including all versions, helm charts and images).
- Create a tarball for rke2 release tarball files (required to be used in capi airgap scenarios).
- Upload the helm-charts oci images defined in the release manifest file to the private registry.
- Upload the containers images defined in the release images file to the private registry.
- Optionally authenticate and populate to SUSE Private Registry for charts and images.

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

for example you can generate both using the following command and concatenate both

```
echo -n "$(echo -n 'myusername' | base64 -w 0):$(echo -n 'mypassword' | base64 -w 0)" > encoded-registry-auth
```

2. If your private registry is using a self-signed certificate, create a CA certificate file and provide also the path to the tool.

3. Rancher Apps charts require authentication. Create a Rancher Apps auth file with the same base64 `user|base64:pass|base64` format described above. See [SUSE Storage installation docs](https://documentation.suse.com/suse-edge/3.5/html/edge/components-suse-storage.html#id-installing-suse-storage):

```
echo -n "$(echo -n 'myusername@apps.rancher.io' | base64 -w 0):$(echo -n 'mypassword' | base64 -w 0)" > rancher-apps-auth
```

4. SUSE Private Registry artifacts are optional. If you want to mirror `oci://registry.suse.com/private-registry/private-registry-helm` or `registry.suse.com/private-registry/harbor*` images, create a SUSE Private Registry auth file using your `SCC mirroring credentials` in the same base64 `user|base64:pass|base64` format and pass it with `--suse-private-registry-authfile`:

```
echo -n "$(echo -n 'SUSE_REGISTRY_USERNAME' | base64 -w 0):$(echo -n 'SUSE_REGISTRY_PASSWORD' | base64 -w 0)" > suse-private-registry-auth
```

If you omit this flag, those SUSE Private Registry artifacts are skipped so mirroring can continue when you are not using from SUSE Private Registry.

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
    --suse-private-registry-authfile string     SUSE Private Registry auth file with username:password base64 encoded
-c, --registry-cacert string     Registry CA Certificate file
-r, --registry-url string        Registry URL (e.g. '192.168.1.100:5000')
-d, --dry-run                    Dry run mode, only print the actions without executing them
-m, --release-mode string        Release mode, can be 'factory' or 'production' (default "factory"). Only used if release-version is provided.
-v, --release-version string     Release version, e.g. 3.4.0 (X.Y.Z). Start Binary Mode if provided.
    --debug                      Debug mode with more logs verbosity
```

## Modes of Operation

### Binary Mode

In this mode, you provide the `release-version` and `release-mode` flags. The tool will download the necessary manifests from the remote release source.

Example:
```bash
seactl mirror \
  -v 3.4.0 -m factory \
  -o /tmp/airgap \
  -a registry-auth.txt \
  -c /opt/certs/ca.crt \
  -r myregistry:5000 \
  --rancher-apps-authfile rancher-auth.txt
```

### Container Mode (Local Files inside Release Container)

In this mode, you have to **omit** the `release-version` flag. The tool expects to find `/release_manifest.yaml` and `/release_images.yaml` in the local filesystem root (intended for containerized usage where these files are present).

**To use this mode you will need to consume the release container image provided by SUSE Telco Cloud, which include this tool inside.**

Example:

```bash
podman run <release-image-id> \
  mirror \
  -o /tmp/airgap \
  -a registry-auth.txt \
  -c /opt/certs/ca.crt \
  -r myregistry:5000 \
  --rancher-apps-authfile rancher-auth.txt
```

where `<release-image-id>` is something like `registry.suse.com/edge/${VERSION}/release-manifest:${Z_VERSION}`.

## Examples

### Binary Mode Examples

Without SUSE Private Registry (Harbor charts/images will be skipped):

```bash
seactl mirror \
  -v 3.4.0 -m production \
  -o /tmp/airgap \
  -a registry-auth.txt \
  -r myregistry:5000 \
  --rancher-apps-authfile rancher-auth.txt \
  --insecure --debug
```

With CA certificate and SUSE Private Registry:

```bash
seactl mirror \
  -v 3.4.0 -m production \
  -o /tmp/airgap \
  -a registry-auth.txt \
  -c /opt/certs/ca.crt \
  -r myregistry:5000 \
  --rancher-apps-authfile rancher-auth.txt \
  --suse-private-registry-authfile suse-private-registry-auth \
  --debug
```

### Container Mode Example

Without SUSE Private Registry (Harbor charts/images will be skipped):

```bash
# /release_manifest.yaml and /release_images.yaml are expected inside the container
podman run --rm \
  -v ./:/opt:z \
  <release-image-id> \
  mirror \
  -o /opt/output \
  -a /opt/registry-auth.txt \
  -c /opt/cert.pem \
  -r myregistry:5000 \
  --rancher-apps-authfile /opt/rancher-apps-auth \
  --insecure --debug
```

With SUSE Private Registry:

```bash
podman run --rm \
  -v ./:/opt:z \
  <release-image-id> \
  mirror \
  -o /opt/output \
  -a /opt/registry-auth.txt \
  -c /opt/cert.pem \
  -r myregistry:5000 \
  --rancher-apps-authfile /opt/rancher-apps-auth \
  --suse-private-registry-authfile /opt/suse-private-registry-auth \
  --debug
```

### Proxy Support

```bash
export HTTPS_PROXY=http://10.X.X.X:3128

seactl mirror \
  -v 3.4.0 -m factory \
  -o /tmp/airgap \
  -a registry-auth.txt \
  -c /opt/certs/ca.crt \
  -r myregistry:5000 \
  --rancher-apps-authfile rancher-auth.txt
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