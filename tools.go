package fluxgo

import (
	"context"
	"encoding/json"

	"github.com/invopop/jsonschema"
	"github.com/ollama/ollama/api"
)

type Tools struct {
	apm *Apm

	tools     map[string]ToolsInterface
	toolsJson ToolsJson
}

type ToolsJson map[string]any

type ToolsSchema *jsonschema.Schema

type ToolsInterface interface {
	Name() string
	Description() string
	Schema() ToolsSchema
	ExecuteTool(ctx context.Context, raw json.RawMessage) (json.RawMessage, error)
}
type ToolDefinition struct {
	Name        string
	Description string
	Schema      ToolsSchema
}

func ToolsStart(apm *Apm) *Tools {
	return &Tools{apm, make(map[string]ToolsInterface), make(ToolsJson)}
}

func (f *FluxGo) AddTools() *FluxGo {
	f.AddDependency(func(apm *Apm) *Tools {
		return ToolsStart(apm)
	})

	return f
}
func (f *Tools) AddTool(tool ToolsInterface) {
	f.tools[tool.Name()] = tool
}
func (f *Tools) GetTool(name string) ToolsInterface {
	tool, ok := f.tools[name]
	if !ok {
		return nil
	}
	return tool
}
func (f *Tools) GetOllamaTools() (api.Tools, error) {
	provider := "ollama"

	if found, ok := f.toolsJson[provider]; ok {
		return found.(api.Tools), nil
	}

	defs := make(api.Tools, 0, len(f.tools))

	for _, tool := range f.tools {
		jsonMarshal, err := json.Marshal(tool.Schema())
		if err != nil {
			return nil, err
		}

		parameters := api.ToolFunctionParameters{}
		if err := json.Unmarshal(jsonMarshal, &parameters); err != nil {
			return nil, err
		}

		defs = append(defs, api.Tool{
			Type: "function",
			Function: api.ToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  parameters,
			},
		})
	}

	f.toolsJson[provider] = defs

	return defs, nil
}

func ToolParseSchema(i any) ToolsSchema {
	val := jsonschema.Reflect(i)

	for _, v := range val.Definitions {
		return v
	}

	return val
}
