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

