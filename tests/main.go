// A generated module for HfToModelkit functions
//
// This module has been generated via dagger init and serves as a reference to
// basic module structure as you get started with Dagger.
//
// Two functions have been pre-created. You can modify, delete, or add to them,
// as needed. They demonstrate usage of arguments and return types using simple
// echo and grep commands. The functions can be called from the dagger CLI or
// from one of the SDKs.
//
// The first line in this comment block is a short description line and the
// rest is a long description with more detail on the module's purpose or usage,
// if appropriate. All modules should have a short description.

package main

import (
	"context"
	"github.com/sourcegraph/conc/pool"
)

type Tests struct{}

func (m *Tests) All(ctx context.Context) error {

	p := pool.New().WithErrors().WithContext(ctx)

	// Call tests to run in parallel
	p.Go(m.Pack)

	return p.Wait()

}

//Tests 
func (m *Tests) Pack(ctx context.Context) error {

	directory := dag.CurrentModule().Source().Directory("./testdata")
	_, err := dag.Kit().Pack(directory, "jozu.ml/packtest:latest").Container().Sync(ctx)
	return err
}

