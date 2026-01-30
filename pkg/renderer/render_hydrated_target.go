package renderer

import (
	"embed"
	"encoding/json"
)

//go:embed template/hydrated-target/*
var hydratedTargetFS embed.FS

var (
	hydratedTargetFiles []string = []string{
		"template/hydrated-target/Chart.yaml",
		"template/hydrated-target/values.yaml",
		"template/hydrated-target/.helmignore",
		"template/hydrated-target/templates/release.yaml",
	}
)

type HydratedTargetInput struct {
}

type HydratedTargetConfig struct {
	Chart  ChartConfig
	Input  HydratedTargetInput
	Values json.RawMessage
}

func RenderHydratedTarget(c HydratedTargetConfig) (*RenderResult, error) {
	return newRenderer().
		withTemplateData(c).
		withTemplateFS(hydratedTargetFS).
		withTemplateFiles(hydratedTargetFiles).
		withOutputName("solar-hydrated-target").
		render()
}
