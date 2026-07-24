package fluxgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/invopop/jsonschema"
)

// SwaggerOptions configures the OpenAPI 3.0 documentation endpoints.
type SwaggerOptions struct {
	// Path is the base URL for the docs. Default: "/swagger"
	Path string
	// Title overrides the app name in the spec info.
	Title string
	// Description sets the API description in the spec info.
	Description string
}

// routeDoc holds the metadata collected for each registered HTTP route.
type routeDoc struct {
	method     string
	path       string // full path: group + route, e.g. /public/user/:id
	tags       []string
	doc        *RouteDoc
	entity     any
	fromBody   bool
	fromQuery  bool
	fromParam  bool
	fromHeader bool
}

var fiberParamRe = regexp.MustCompile(`:(\w+)`)

// fiberPathToOpenAPI converts Fiber param syntax (:id) to OpenAPI syntax ({id}).
func fiberPathToOpenAPI(path string) string {
	return fiberParamRe.ReplaceAllString(path, "{$1}")
}

// extractPathParams returns param names from a Fiber path (e.g. [:id, :slug]).
func extractPathParams(path string) []string {
	matches := fiberParamRe.FindAllStringSubmatch(path, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		result = append(result, m[1])
	}
	return result
}

// addRouteDoc registers metadata for a route so it appears in the generated spec.
func (h *Http) addRouteDoc(doc routeDoc) {
	h.docs = append(h.docs, doc)
}

// schemaRef reflects body into an OpenAPI-compatible schema object.
// All nested type definitions are registered in components (shared map).
// Internal $defs refs are rewritten to #/components/schemas/.
func schemaRef(body any, components map[string]any) any {
	r := jsonschema.Reflector{ExpandedStruct: true}
	schema := r.Reflect(body)

	// Register all nested definitions in the shared components map.
	for name, def := range schema.Definitions {
		if _, exists := components[name]; exists {
			continue
		}
		b, err := json.Marshal(def)
		if err != nil {
			continue
		}
		b = bytes.ReplaceAll(b, []byte(`"#/$defs/`), []byte(`"#/components/schemas/`))
		var v any
		if err := json.Unmarshal(b, &v); err == nil {
			components[name] = v
		}
	}

	// Marshal the root schema, rewrite refs, strip JSON Schema meta-fields.
	b, err := json.Marshal(schema)
	if err != nil {
		return map[string]any{"type": "object"}
	}
	b = bytes.ReplaceAll(b, []byte(`"#/$defs/`), []byte(`"#/components/schemas/`))

	var root map[string]any
	if err := json.Unmarshal(b, &root); err != nil {
		return map[string]any{"type": "object"}
	}
	delete(root, "$schema")
	delete(root, "$id")
	delete(root, "$defs") // moved to components

	return root
}

// responseObject builds an OpenAPI response object with optional JSON schema.
func responseObject(description string, d *RouteDoc, bodyFn func(*RouteDoc) any, components map[string]any) map[string]any {
	obj := map[string]any{"description": description}
	if d == nil {
		return obj
	}
	body := bodyFn(d)
	if body == nil {
		return obj
	}
	obj["content"] = map[string]any{
		"application/json": map[string]any{
			"schema": schemaRef(body, components),
		},
	}
	return obj
}

// buildOpenAPISpec generates an OpenAPI 3.0 spec from all collected route docs.
// Spec generation is lazy: called on first request to /swagger/openapi.json,
// ensuring all routes are already registered.
func (h *Http) buildOpenAPISpec(title, version, description string) map[string]any {
	paths := map[string]any{}
	components := map[string]any{} // shared schemas accumulator

	for _, doc := range h.docs {
		oaPath := fiberPathToOpenAPI(doc.path)

		if _, ok := paths[oaPath]; !ok {
			paths[oaPath] = map[string]any{}
		}
		pathItem := paths[oaPath].(map[string]any)

		tags := doc.tags
		if doc.doc != nil && len(doc.doc.Tags) > 0 {
			tags = doc.doc.Tags
		}

		responses := map[string]any{
			"200": responseObject("Success", doc.doc, func(d *RouteDoc) any { return d.OkResponse }, components),
			"400": responseObject("Bad Request", doc.doc, func(d *RouteDoc) any { return d.BadRequestResponse }, components),
			"422": map[string]any{"description": "Validation Error"},
			"500": map[string]any{"description": "Internal Server Error"},
		}
		if doc.doc != nil && doc.doc.CreatedResponse != nil {
			responses["201"] = responseObject("Created", doc.doc, func(d *RouteDoc) any { return d.CreatedResponse }, components)
		}

		operation := map[string]any{
			"tags":      tags,
			"responses": responses,
		}

		if doc.doc != nil {
			if doc.doc.Summary != "" {
				operation["summary"] = doc.doc.Summary
			}
			if doc.doc.Description != "" {
				operation["description"] = doc.doc.Description
			}
			if doc.doc.OperationId != "" {
				operation["operationId"] = doc.doc.OperationId
			}
			if doc.doc.Deprecated {
				operation["deprecated"] = true
			}
		}

		// Index entity fields by name (json/query/header tag or lowercase) for lookup.
		entityFields := map[string]reflect.StructField{}
		if doc.entity != nil {
			t := reflect.TypeOf(doc.entity)
			if t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			if t.Kind() == reflect.Struct {
				for i := range t.NumField() {
					f := t.Field(i)
					for _, key := range []string{"json", "query", "header", "params"} {
						if name := tagValue(f, key); name != "" {
							entityFields[name] = f
						}
					}
					entityFields[strings.ToLower(f.Name)] = f
				}
			}
		}
		// Path parameters — from URL pattern, enriched with entity field metadata if matched.
		pathParamSet := map[string]bool{}
		params := []any{}
		pathsParams := extractPathParams(doc.path)
		for _, p := range pathsParams {
			pathParamSet[p] = true
			if field, ok := entityFields[p]; ok {
				params = append(params, buildParam(p, "path", true, field))
			} else {
				params = append(params, map[string]any{
					"name": p, "in": "path", "required": true,
					"schema": map[string]any{"type": "string"},
				})
			}
		}
		if doc.entity != nil {
			entityType := reflect.TypeOf(doc.entity)
			if entityType.Kind() == reflect.Ptr {
				entityType = entityType.Elem()
			}

			// Walk entity fields: query → in:query | header → in:header | json → body (if not a path param).
			var hasBodyFields bool
			if entityType.Kind() == reflect.Struct {
				for i := range entityType.NumField() {
					field := entityType.Field(i)
					required := strings.Contains(field.Tag.Get("validate"), "required")

					if name := tagValue(field, "query"); name != "" {
						params = append(params, buildParam(name, "query", required, field))
						continue
					}
					if name := tagValue(field, "header"); name != "" {
						params = append(params, buildParam(name, "header", required, field))
						continue
					}
					if name := tagValue(field, "json"); name != "" && !pathParamSet[name] {
						hasBodyFields = true
					}
				}
			}

			// json: fields not matched to path params → requestBody.
			if hasBodyFields {
				operation["requestBody"] = map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": schemaRef(doc.entity, components),
						},
					},
				}
			}
		}

		if len(params) > 0 {
			operation["parameters"] = params
		}

		pathItem[strings.ToLower(doc.method)] = operation
	}

	info := map[string]any{
		"title":   title,
		"version": version,
	}
	if description != "" {
		info["description"] = description
	}

	spec := map[string]any{
		"openapi": "3.0.3",
		"info":    info,
		"paths":   paths,
	}
	if len(components) > 0 {
		spec["components"] = map[string]any{"schemas": components}
	}
	return spec
}

// buildParam builds an OpenAPI parameter object.
// description is hoisted to the parameter level (not inside schema).
func buildParam(name, in string, required bool, f reflect.StructField) map[string]any {
	schema := fieldSchema(f)
	param := map[string]any{
		"name":     name,
		"in":       in,
		"required": required,
		"schema":   schema,
	}
	// Only description is a valid top-level parameter property in OpenAPI 3.0.
	// title and other jsonschema props stay inside schema.
	if desc, ok := schema["description"]; ok {
		param["description"] = desc
		delete(schema, "description")
	}
	return param
}

// fieldSchema builds an OpenAPI schema object for a single struct field,
// reading type from reflection and enriching with jsonschema: tag metadata.
func fieldSchema(f reflect.StructField) map[string]any {
	schema := map[string]any{"type": goTypeToOAType(f.Type)}
	for k, v := range extractJsonSchemaMeta(f) {
		schema[k] = v
	}
	return schema
}

// extractJsonSchemaMeta parses the jsonschema: struct tag and returns
// recognised OpenAPI schema properties (description, example, minimum, etc.).
func extractJsonSchemaMeta(f reflect.StructField) map[string]any {
	props := map[string]any{}
	raw := f.Tag.Get("jsonschema")
	if raw == "" {
		return props
	}
	for _, part := range strings.Split(raw, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		switch key, val := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]); key {
		case "description", "title", "pattern", "format", "example", "default":
			props[key] = val
		case "minimum", "maximum", "minLength", "maxLength", "multipleOf", "exclusiveMinimum", "exclusiveMaximum":
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				props[key] = n
			}
		}
	}
	return props
}

// tagValue returns the first segment of the given struct tag (before any comma),
// or empty string if the tag is absent or set to "-".
func tagValue(f reflect.StructField, key string) string {
	raw := f.Tag.Get(key)
	if raw == "" {
		return ""
	}
	name := strings.Split(raw, ",")[0]
	if name == "-" {
		return ""
	}
	return name
}

// goTypeToOAType maps a Go reflect.Kind to an OpenAPI primitive type string.
func goTypeToOAType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	default:
		return "string"
	}
}

// swaggerUIHTML returns an HTML page that loads Swagger UI from unpkg CDN.
func swaggerUIHTML(specURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
  <head>
    <title>API Docs</title>
    <meta charset="utf-8"/>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      SwaggerUIBundle({
        url: %q,
        dom_id: '#swagger-ui',
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
        layout: "BaseLayout"
      })
    </script>
  </body>
</html>`, specURL)
}
