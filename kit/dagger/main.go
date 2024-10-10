package main

import (
	"context"
	"dagger/kit/internal/dagger"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	fplainHttp   = "--plain-http"
	baseImageRef = "cgr.dev/chainguard/wolfi-base:latest"
	kitCommand   = "/app/kit"
)

type Kit struct {
	Registry  string
	PlainHTTP bool
	Version   string
	Container *dagger.Container
}

func New(
	// OCI registry
	// +optional
	registry string,
	// use plainHttp
	// +optional
	plainHttp bool,
	// Kit version
	// +optional
	// +default="latest"
	version string,
) *Kit {
	return &Kit{
		Registry:  registry,
		PlainHTTP: plainHttp,
		Version:   version,
	}
}

// Release represents the structure of a GitHub release
type Release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

var allowedFilters = map[string]bool{
	"--docs":     true,
	"--code":     true,
	"--model":    true,
	"--datasets": true,
	"--kitfile":  true,
}

func isValidFilter(filter string) bool {
	return allowedFilters[filter]
}

func (m *Kit) downloadKit() *dagger.File {

	var release Release
	var err error
	if m.Version == "latest" {
		// Fetch the latest release information
		release, err = fetchLatestRelease()
		if err != nil {
			fmt.Printf("Error fetching latest release: %v\n", err)
			return nil
		}
	} else {
		// Fetch the specific version release information
		release, err = fetchRelease(m.Version)
		if err != nil {
			fmt.Printf("Error fetching release %s: %v\n", m.Version, err)
			return nil
		}
	}

	var fileURL, checksumURL string
	for _, asset := range release.Assets {
		if asset.Name == "kitops-linux-x86_64.tar.gz" {
			fileURL = asset.BrowserDownloadURL
		} else if strings.HasSuffix(asset.Name, "checksums.txt") {
			checksumURL = asset.BrowserDownloadURL
		}
	}

	if fileURL == "" || checksumURL == "" {
		fmt.Println("File or checksum not found in the release assets.")
		return nil
	}

	// Destination paths
	filePath := filepath.Join(".", "kitops-linux-x86_64.tar.gz")
	checksumPath := filepath.Join(".", "checksums.txt")

	// Download the file and checksum
	if err := downloadFile(fileURL, filePath); err != nil {
		fmt.Printf("Error downloading file: %v\n", err)
		return nil
	}

	if err := downloadFile(checksumURL, checksumPath); err != nil {
		fmt.Printf("Error downloading checksum: %v\n", err)
		return nil
	}

	// Verify the checksum
	valid, err := verifyChecksum(filePath, checksumPath)
	if err != nil {
		fmt.Printf("Error verifying checksum: %v\n", err)
		return nil
	}

	if !valid {
		fmt.Println("Checksum verification failed!")
		return nil
	}

	fmt.Println("Checksum verified successfully.")

	// Unpack the tar.gz file
	if err := unpackTarGz(filePath, "."); err != nil {
		fmt.Printf("Error unpacking tar.gz file: %v\n", err)
		return nil
	}

	fmt.Println("File unpacked successfully.")
	return dag.CurrentModule().WorkdirFile("kit")
}

func (m *Kit) baseContainer() *dagger.Container {

	cacheOpts := &dagger.ContainerWithMountedCacheOpts{
		Sharing: dagger.Private,
	}
	if m.Container == nil {
		m.Container = dag.Container().
			From(baseImageRef).
			WithFile(kitCommand, m.downloadKit()).
			WithMountedCache("/.kitops", dag.CacheVolume("kitops"), *cacheOpts).
			WithEnvVariable("KITOPS_HOME", "/.kitops")

	}
	return m.Container
}

func (m *Kit) WithAuth(username string, password *dagger.Secret) *Kit {
	cmd := []string{kitCommand, "login", "-v", m.Registry, "-u", username, "-p", "$KIT_PASSWORD"}
	if m.PlainHTTP {
		cmd = append(cmd, fplainHttp)
	}
	m.Container = m.baseContainer().
		WithSecretVariable("KIT_PASSWORD", password).
		WithExec([]string{"/bin/sh", "-c", strings.Join(cmd, " ")})
	return m
}

func (m *Kit) Pack(ctx context.Context, 
	// directory to pack
	directory *dagger.Directory, 
	// tag reference
	reference string,
	// the kitfile
	// +optional
	kitfile *dagger.File ) (*Kit, error) {
	cmd := []string{kitCommand, "pack", "/mnt", "-t", reference}
	c := m.baseContainer().
	WithMountedDirectory("/mnt", directory).
	WithWorkdir("/mnt")
	
	if kitfile != nil {
		c = c.WithFile("kitfile.yml", kitfile)
		cmd = append(cmd, "-f", "kitfile.yml")
	}
	_, err := c.WithExec(cmd).
		Stdout(ctx)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Kit) Unpack(ctx context.Context,
	// reference to the ModelKit
	reference string,
	// the artifacts to unpack
	// +optional
	filter []string) (*dagger.Directory, error) {
	cmd := []string{kitCommand, "unpack", reference, "-d", "/unpack"}
	for _, f := range filter {
		if !isValidFilter(f) {
			return nil, errors.New(f + " is not a valid filter")
		}
		cmd = append(cmd, f)
	}
	if m.PlainHTTP {
		cmd = append(cmd, fplainHttp)
	}
	return m.baseContainer().
		WithExec(cmd).
		Directory("/unpack"), nil
}

func (m *Kit) Pull(ctx context.Context, reference string) (*Kit, error) {
	cmd := []string{kitCommand, "pull", reference}
	if m.PlainHTTP {
		cmd = append(cmd, fplainHttp)
	}
	_, err := m.baseContainer().
		WithExec(cmd).
		Stdout(ctx)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Kit) Push(ctx context.Context, reference string) error {
	cmd := []string{kitCommand, "push", reference}
	if m.PlainHTTP {
		cmd = append(cmd, fplainHttp)
	}
	_, err := m.baseContainer().
		WithExec(cmd).
		Stdout(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (m *Kit) Tag(ctx context.Context, currentRef string, newRef string) (*Kit, error) {
	cmd := []string{kitCommand, "tag", currentRef, newRef}
	if m.PlainHTTP {
		cmd = append(cmd, fplainHttp)
	}
	_, err := m.baseContainer().
		WithExec(cmd).
		Stdout(ctx)
	if err != nil {
		return nil, err
	}
	return m, nil
}
