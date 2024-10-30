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
	fplainHttp    = "--plain-http"
	baseImageRef  = "cgr.dev/chainguard/wolfi-base:latest"
	kitCommand    = "/app/kit"
	kitOpsHomeDir = "./kitops"
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

func (m *Kit) downloadKit() (*dagger.File, error) {

	var release Release
	var err error
	if m.Version == "latest" {
		// Fetch the latest release information
		release, err = fetchLatestRelease()
		if err != nil {
			return nil, fmt.Errorf("error fetching latest release: %w", err)
		}
	} else {
		// Fetch the specific version release information
		release, err = fetchRelease(m.Version)
		if err != nil {
			return nil, fmt.Errorf("error fetching release %s: %w", m.Version, err)

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
		return nil, fmt.Errorf("file or checksum not found in the release assets")

	}

	// Destination paths
	filePath := filepath.Join(".", "kitops-linux-x86_64.tar.gz")
	checksumPath := filepath.Join(".", "checksums.txt")

	// Download the file and checksum
	if err := downloadFile(fileURL, filePath); err != nil {
		return nil, fmt.Errorf("error downloading file from %s: %w", fileURL, err)
	}

	if err := downloadFile(checksumURL, checksumPath); err != nil {
		return nil, fmt.Errorf("error downloading checksum: %w", err)
	}

	// Verify the checksum
	valid, err := verifyChecksum(filePath, checksumPath)
	if err != nil {
		return nil, fmt.Errorf("error verifying checksum: %w", err)
	}

	if !valid {
		return nil, fmt.Errorf("checksum verification failed")
	}
	fmt.Println("Checksum verified successfully.")

	// Unpack the tar.gz file
	if err := unpackTarGz(filePath, "."); err != nil {
		return nil, fmt.Errorf("error unpacking tar.gz file: %w", err)
	}
	fmt.Println("File unpacked successfully.")

	return dag.CurrentModule().WorkdirFile("kit"), nil
}

func (m *Kit) baseContainer(_ context.Context) (*dagger.Container, error) {

	cacheOpts := &dagger.ContainerWithMountedCacheOpts{
		Sharing: dagger.Private,
	}
	kitBinary, err := m.downloadKit()
	if err != nil {
		return nil, err
	}

	if m.Container == nil {
		m.Container = dag.Container().
			From(baseImageRef).
			WithFile(kitCommand, kitBinary).
			WithMountedCache(kitOpsHomeDir, dag.CacheVolume("kitops"), *cacheOpts).
			WithEnvVariable("KITOPS_HOME", kitOpsHomeDir)

	}
	return m.Container, nil
}

func (m *Kit) WithAuth(ctx context.Context, username string, password *dagger.Secret) (*Kit, error) {
	cmd := []string{kitCommand, "login", "-v", m.Registry, "-u", username, "-p", "$KIT_PASSWORD"}
	if m.PlainHTTP {
		cmd = append(cmd, fplainHttp)
	}
	var err error
	m.Container, err = m.baseContainer(ctx)
	if err != nil {
		return nil, err
	}

	_, err = m.Container.WithSecretVariable("KIT_PASSWORD", password).
		WithExec([]string{"/bin/sh", "-c", strings.Join(cmd, " ")}).
		Stdout(ctx)
	var e *dagger.ExecError
	if err != nil {
		if errors.As(err, &e) {
			return nil, fmt.Errorf("authentication failed: %s", e.Stderr)
		}
		return nil, fmt.Errorf("error authenticating: %s", e.Stderr)
	}
	return m, nil
}

func (m *Kit) Pack(ctx context.Context,
	// directory to pack
	directory *dagger.Directory,
	// tag reference
	reference string,
	// the kitfile
	// +optional
	kitfile *dagger.File) (*Kit, error) {
	cmd := []string{kitCommand, "pack", "/mnt", "-t", reference}
	c, err := m.baseContainer(ctx)
	if err != nil {
		return nil, err
	}
	c = c.WithMountedDirectory("/mnt", directory).
		WithWorkdir("/mnt")

	if kitfile != nil {
		c = c.WithFile("kitfile.yml", kitfile)
		cmd = append(cmd, "-f", "kitfile.yml")
	}
	_, err = c.WithExec(cmd).
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

	c, err := m.baseContainer(ctx)
	if err != nil {
		return nil, err
	}

	return c.WithExec(cmd).
		Directory("/unpack"), nil
}

func (m *Kit) Pull(ctx context.Context, reference string) (*Kit, error) {
	cmd := []string{kitCommand, "pull", reference}
	if m.PlainHTTP {
		cmd = append(cmd, fplainHttp)
	}

	c, err := m.baseContainer(ctx)
	if err != nil {
		return nil, err
	}
	_, err = c.WithExec(cmd).
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
	c, err := m.baseContainer(ctx)
	if err != nil {
		return err
	}

	_, err = c.WithExec(cmd).
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

	c, err := m.baseContainer(ctx)
	if err != nil {
		return nil, err
	}
	_, err = c.WithExec(cmd).
		Stdout(ctx)
	if err != nil {
		return nil, err
	}
	return m, nil
}
