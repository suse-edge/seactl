package rke2

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// --- Helpers: create temporary directories/files for testing ---
func createTempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "rke2_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	return dir
}

func removeTempDir(t *testing.T, dir string) {
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("failed to remove temp dir: %v", err)
	}
}

// --- Tests for getFileFromURL ---
func Test_getFileFromURL(t *testing.T) {
	tempDir := createTempDir(t)
	defer removeTempDir(t, tempDir)

	// Mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/success" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("file content"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tests := []struct {
		name     string
		url      string
		filename string
		wantErr  bool
	}{
		{
			name:     "Successful download",
			url:      server.URL + "/success",
			filename: "file.txt",
			wantErr:  false,
		},
		{
			name:     "HTTP error",
			url:      server.URL + "/fail",
			filename: "file.txt",
			wantErr:  true,
		},
		{
			name:     "Invalid directory",
			url:      server.URL + "/success",
			filename: "file.txt",
			wantErr:  true, // because invalid path will cause os.Create to fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tempDir
			if tt.name == "Invalid directory" {
				dir = "/invalid/dir/"
			}
			err := getFileFromURL(tt.url, tt.filename, ensureTrailingSlash(dir))
			if (err != nil) != tt.wantErr {
				t.Errorf("getFileFromURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// --- Tests for RKE2.Download() ---

func TestRKE2_Download(t *testing.T) {
	// Create a temporary directory for downloads
	tempDir, err := ioutil.TempDir("", "rke2_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock HTTP server to simulate all file downloads
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return dummy content for any file requested
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("dummy content"))
	}))
	defer server.Close()

	// Create RKE2 instance with mocked ReleaseURL
	r := New("v1.21.3+rke2r1", tempDir)
	r.ReleaseURL = server.URL + "/" // override for testing

	// Run Download()
	if err := r.Download(); err != nil {
		t.Fatalf("Download() failed: %v", err)
	}

	// Verify all expected files exist
	for _, image := range listRKE2Images {
		filePath := filepath.Join(tempDir, image)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist after download", filePath)
		}
	}

	// Verify install.sh exists
	installScriptPath := filepath.Join(tempDir, "install.sh")
	if _, err := os.Stat(installScriptPath); os.IsNotExist(err) {
		t.Errorf("Expected install.sh to exist after download")
	}
}

// --- Tests for RKE2.Verify() ---
func TestRKE2_Verify(t *testing.T) {
	tempDir := createTempDir(t)
	defer removeTempDir(t, tempDir)

	r := New("v1.21.3+rke2r1", tempDir)

	// Case: missing files
	if err := r.Verify(); err == nil {
		t.Errorf("Verify() should fail when files missing")
	}

	// Case: all files exist
	for _, f := range listRKE2Images {
		err := ioutil.WriteFile(filepath.Join(tempDir, f), []byte("dummy"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	if err := r.Verify(); err != nil {
		t.Errorf("Verify() error = %v", err)
	}
}

// --- Existing helper tests ---
func Test_replaceVersionLink(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"With plus", "v1.21.3+rke2r1", "v1.21.3%2Brke2r1"},
		{"Without plus", "v1.21.3", "v1.21.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replaceVersionLink(tt.version); got != tt.want {
				t.Errorf("replaceVersionLink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ensureTrailingSlash(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"No slash", "test", "test/"},
		{"With slash", "test/", "test/"},
		{"Relative path with slash", "./test/", "./test/"},
		{"Nested path with slash", "./aa/test/", "./aa/test/"},
		{"Nested path no slash", "./aa/test", "./aa/test/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ensureTrailingSlash(tt.dir); got != tt.want {
				t.Errorf("ensureTrailingSlash() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestRKE2_Upload(t *testing.T) {
	r := New("v1.2.3", "outdir")
	err := r.Upload()
	if err != nil { t.Errorf("unexpected error: %v", err) }
}

func TestRKE2_Download_FailMkdir(t *testing.T) {
	r := New("v1.2.3", "/root/readonly/cannot-create")
	r.Download()
}

func TestRKE2_Download_FailInstallScript(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)
	oldRKE2URL := RKE2URL
	RKE2URL = "http://invalid-url"
	defer func() { RKE2URL = oldRKE2URL }()
	r := New("v1.2.3", tempDir)
	r.Download()
}

func TestRKE2_Download_FailImageTarball(t *testing.T) {
	tempDir := createTempDir(t)
	defer os.RemoveAll(tempDir)
	oldRKE2URL := RKE2URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	RKE2URL = server.URL
	defer func() { RKE2URL = oldRKE2URL }()
	r := New("v1.2.3", tempDir)
	r.ReleaseURL = "http://invalid-url/"
	r.Download()
}