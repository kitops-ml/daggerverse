package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	latestReleaseAPIURL = "https://api.github.com/repos/jozu-ai/kitops/releases/latest"
	releaseAPIURL       = "https://api.github.com/repos/jozu-ai/kitops/releases/tags/%s"
)

// Function to fetch the latest release information
func fetchLatestRelease() (Release, error) {
	var release Release
	resp, err := http.Get(latestReleaseAPIURL)
	if err != nil {
		return release, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return release, fmt.Errorf("bad status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return release, err
	}

	return release, nil
}

// Function to fetch the release information
func fetchRelease(tag string) (Release, error) {
	var release Release
	url := fmt.Sprintf(releaseAPIURL, tag)
	resp, err := http.Get(url)
	if err != nil {
		return release, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return release, fmt.Errorf("bad status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return release, err
	}

	return release, nil
}

// Function to download a file from a URL
func downloadFile(url, dest string) error {
	// Create the file
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check server response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

// Function to verify the checksum of a file
func verifyChecksum(filePath, checksumPath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}

	calculatedChecksum := fmt.Sprintf("%x", hash.Sum(nil))

	checksumFile, err := os.ReadFile(checksumPath)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(checksumFile), "\n")
	expectedChecksum := ""
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == filepath.Base(filePath) {
			expectedChecksum = parts[0]
			break
		}
	}

	if expectedChecksum == "" {
		return false, fmt.Errorf("checksum for file %s not found", filepath.Base(filePath))
	}

	return calculatedChecksum == expectedChecksum, nil
}

// Function to unpack a tar.gz file
func unpackTarGz(src, dest string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	defer file.Close()

	uncompressedStream, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer uncompressedStream.Close()

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		target := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tarReader); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	return nil
}
