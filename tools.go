package fluxgo

import (
	"context"
	"encoding/json"

	"github.com/invopop/jsonschema"
)

type Tools struct {
	apm *Apm

	tools     []ToolsInterface
	toolsJson ToolsJson
}

type ToolsJson map[string][]byte

type ToolsSchema *jsonschema.Schema

type ToolsInterface interface {
	Name() string
	Description() string
	Schema() ToolsSchema
	ExecuteTool(ctx context.Context, raw json.RawMessage) (any, error)
}
type ToolDefinition struct {
	Name        string
	Description string
	Schema      ToolsSchema
}

func (f *FluxGo) AddTools() *FluxGo {
	f.AddDependency(func(apm *Apm) *Tools {
		return &Tools{apm, make([]ToolsInterface, 0), make(ToolsJson)}
	})

	return f
}
func (f *Tools) AddTool(tool ToolsInterface) {
	f.tools = append(f.tools, tool)
}
func (f *Tools) GetOllamaTools() ([]byte, error) {
	provider := "ollama"

	found, ok := f.toolsJson[provider]
	if ok {
		return found, nil
	}

	defs := make([]map[string]any, 0, len(f.tools))

	for _, tool := range f.tools {
		defs = append(defs, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name(),
				"description": tool.Description(),
				"parameters":  tool.Schema(),
			},
		})
	}

	toolsJson, err := json.Marshal(defs)
	if err != nil {
		return nil, err
	}

	f.toolsJson[provider] = toolsJson

	return toolsJson, nil
}

func ToolParseSchema(i any) ToolsSchema {
	val := jsonschema.Reflect(i)

	for _, v := range val.Definitions {
		return v
	}

	return val
}
