package main

import (
	"context"
	"dagger/gguf/internal/dagger"
)

const (
	ggufConvertScript = "/app/convert_hf_to_gguf.py"
	llamacppImageRef  = "ghcr.io/ggerganov/llama.cpp:full"
	convertedFileName = "converted.gguf"
	quantizedFileName = "quantized.gguf"
)

type Gguf struct{}

func (m *Gguf) BaseContainer() *dagger.Container {
	return dag.Container().
		From(llamacppImageRef).
		WithoutEntrypoint()
}

// Converts a model to GGUF format.
// outfile: the output file name for the converted model.
// Returns the resulting file
func (m *Gguf) ConvertToGGuf(

	// the directory containing the source model.
	source *dagger.Directory,
	// additional parameters to pass to the conversion script.
	// +optional
	parameters ...string) *dagger.File {

	execWord := []string{"python3", ggufConvertScript, "/src", "--outfile", convertedFileName}
	execWord = append(execWord, parameters...)
	return m.BaseContainer().
		WithMountedDirectory("/src", source).
		WithExec(execWord).
		File(convertedFileName)
}

// Quantize applies quantization to a given model file.
func (m *Gguf) Quantize(ctx context.Context,
	// the source model file to be quantized.
	source *dagger.File,
	// the quantization parameter to apply.
	quantization string) *dagger.File {

	modelname, err := source.Name(ctx)
	if err != nil {
		return nil
	}

	execWord := []string{"/app/llama-quantize", modelname, quantizedFileName, quantization}
	return m.BaseContainer().
		WithMountedFile(modelname, source).
		WithExec(execWord).
		File(quantizedFileName)
}
