// Dagger functions for downloading Huggingface repos and files.
//
// DownloadRepo downloads a Huggingface repo and returns the Directory to the downloaded repo.
// DownloadFile downloads a single file from Huggingface repo and returns the File.
//
// Uses the Huggingface CLI to download the repos and files.

package main

import (
	"context"
	"dagger/huggingface/internal/dagger"
	"fmt"
)

const (
	pythonImageRef = "cgr.dev/chainguard/python:latest-dev"
	localRepoDir   = "./hfrepo"
	hfCliPath      = "./.local/bin/huggingface-cli"
)

type Huggingface struct{}

func (m *Huggingface) baseContainer() *dagger.Container {
	return dag.Container().From(pythonImageRef).
		WithoutEntrypoint().
		//Set it to $HOME
		WithWorkdir("/home/nonroot").
		WithExec([]string{"pip", "install", "-U", "huggingface_hub[cli]"}).
		WithExec([]string{"pip", "install", "-U", "huggingface_hub[hf_transfer]"}).
		WithEnvVariable("HF_HUB_ENABLE_HF_TRANSFER", "1")
}

// Downloads a Huggingface repo and returns the Directory to the downloaded repo
func (m *Huggingface) DownloadRepo(ctx context.Context,
	// the Huggingface repository to download.
	hfrepo string,
	// the Huggingface secret token for authentication
	secret *dagger.Secret) (*dagger.Directory, error) {
	c := m.baseContainer().
		WithSecretVariable("HF_TOKEN", secret).
		WithExec([]string{hfCliPath, "download", hfrepo, "--local-dir", localRepoDir, "--token", "$HF_TOKEN"})

	_, err := c.Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to download repo %s: %w", hfrepo, err)
	}
	return c.Directory(localRepoDir), nil
}

// Downloads a single file from Huggingface repo and returns the File
func (m *Huggingface) DownloadFile(ctx context.Context,
	// the Huggingface repository to download.
	hfrepo string,
	// The path of the file to download
	path string,
	// the Huggingface secret token for authentication
	secret *dagger.Secret) (*dagger.File, error) {
	c := m.baseContainer().
		WithSecretVariable("HF_TOKEN", secret).
		WithExec([]string{hfCliPath, "download", hfrepo, path, "--local-dir", localRepoDir, "--token", "$HF_TOKEN"})
	_, err := c.Stdout(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s from %s: %w", path, hfrepo, err)
	}

	return c.Directory(localRepoDir).File(path), nil
}
