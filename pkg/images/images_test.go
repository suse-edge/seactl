package images

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/alknopfler/seactl/pkg/registry"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ------------------------
// Helpers
// ------------------------

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "authfile")
	err := os.WriteFile(tmp, []byte(content), 0600)
	require.NoError(t, err)
	return tmp
}

// save originals to restore later
var (
	origRemoteImage = remoteImage
	origRemoteWrite = remoteWrite
)

func setupTest(t *testing.T) {
	remoteImage = origRemoteImage
	remoteWrite = origRemoteWrite
}

// ------------------------
// New tests
// ------------------------

func TestNew(t *testing.T) {
	setupTest(t)

	reg := registry.New("auth.json", "registry.io", "", false)
	img := New("nginx:latest", reg, nil)

	assert.Equal(t, "nginx:latest", img.Name)
	assert.Equal(t, reg, img.reg)
	assert.False(t, img.Insecure)
}

// ------------------------
// fakeImage implements v1.Image
// ------------------------

type fakeImage struct{}

func (f *fakeImage) MediaType() (types.MediaType, error)     { return types.OCIManifestSchema1, nil }
func (f *fakeImage) Digest() (v1.Hash, error)                { return v1.Hash{Algorithm: "sha256", Hex: "fake"}, nil }
func (f *fakeImage) ConfigName() (v1.Hash, error)            { return v1.Hash{}, nil }
func (f *fakeImage) RawConfigFile() ([]byte, error)          { return []byte{}, nil }
func (f *fakeImage) ConfigFile() (*v1.ConfigFile, error)     { return &v1.ConfigFile{}, nil }
func (f *fakeImage) Layers() ([]v1.Layer, error)             { return []v1.Layer{}, nil }
func (f *fakeImage) Layer(v1.Hash) (v1.Layer, error)         { return nil, nil }
func (f *fakeImage) LayerByDiffID(v1.Hash) (v1.Layer, error) { return nil, nil }
func (f *fakeImage) LayerByDigest(v1.Hash) (v1.Layer, error) { return nil, nil }
func (f *fakeImage) Manifest() (*v1.Manifest, error)         { return &v1.Manifest{}, nil }
func (f *fakeImage) RawManifest() ([]byte, error)            { return []byte{}, nil }
func (f *fakeImage) Size() (int64, error)                    { return 0, nil }
func (f *fakeImage) ConfigLayer() (v1.Layer, error)          { return nil, nil }

// ------------------------
// Download tests
// ------------------------

func TestDownload_Success(t *testing.T) {
	setupTest(t)

	reg := registry.New("auth.json", "registry.io", "", false)
	img := New("nginx:latest", reg, nil)

	remoteImage = func(ref name.Reference, opts ...remote.Option) (v1.Image, error) {
		return &fakeImage{}, nil
	}

	err := img.Download()
	assert.NoError(t, err)
	assert.NotNil(t, img.ImageRef)
}

func TestDownload_InvalidRef(t *testing.T) {
	setupTest(t)

	reg := registry.New("auth.json", "registry.io", "", false)
	img := New("!invalid-ref", reg, nil)

	err := img.Download()
	assert.Error(t, err)
	assert.Nil(t, img.ImageRef)
}

func TestDownload_FailRemote(t *testing.T) {
	setupTest(t)

	reg := registry.New("auth.json", "registry.io", "", false)
	img := New("nginx:latest", reg, nil)

	remoteImage = func(ref name.Reference, opts ...remote.Option) (v1.Image, error) {
		return nil, errors.New("fake error")
	}

	err := img.Download()
	assert.Error(t, err)
	assert.Nil(t, img.ImageRef)
}

// ------------------------
// Upload tests
// ------------------------

func TestUpload_Success(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "dXNlcg==:cGFzc3dvcmQ=") // user:password
	reg := registry.New(authFile, "registry.io", "", false)
	img := New("nginx:latest", reg, nil)
	img.ImageRef = &fakeImage{}

	remoteWrite = func(ref name.Reference, img v1.Image, opts ...remote.Option) error {
		return nil
	}

	err := img.Upload()
	assert.NoError(t, err)
}

func TestUpload_InvalidRef(t *testing.T) {
	setupTest(t)

	reg := registry.New("auth.json", "registry.io", "", false)
	img := New("!bad-ref", reg, nil)
	img.ImageRef = &fakeImage{}

	err := img.Upload()
	assert.Error(t, err)
}

func TestUpload_FailRemoteWrite(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "dXNlcg==:cGFzc3dvcmQ=")
	reg := registry.New(authFile, "registry.io", "", false)
	img := New("nginx:latest", reg, nil)
	img.ImageRef = &fakeImage{}

	remoteWrite = func(ref name.Reference, img v1.Image, opts ...remote.Option) error {
		return errors.New("push failed")
	}

	err := img.Upload()
	assert.Error(t, err)
}

// ------------------------
// getRemoteOpts tests
// ------------------------

func TestGetRemoteOpts_Success(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "dXNlcg==:cGFzc3dvcmQ=") // user:password
	reg := registry.New(authFile, "registry.io", "", false)
	img := New("nginx:latest", reg, nil)

	opts, err := img.getRemoteOpts()
	assert.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestGetRemoteOpts_InvalidCA(t *testing.T) {
	setupTest(t)

	authFile := writeTempFile(t, "dXNlcg==:cGFzc3dvcmQ=")
	reg := registry.New(authFile, "registry.io", "missing-ca.crt", false)
	img := New("nginx:latest", reg, nil)

	opts, err := img.getRemoteOpts()
	assert.Error(t, err)
	assert.Nil(t, opts)
}

func TestGetRemoteOpts_InvalidAuthFile(t *testing.T) {
	setupTest(t)

	reg := registry.New("not-found.json", "registry.io", "", false)
	img := New("nginx:latest", reg, nil)

	opts, err := img.getRemoteOpts()
	assert.Error(t, err)
	assert.Nil(t, opts)
}
