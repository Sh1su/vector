// Package api enthält den aus api/openapi.yaml generierten Server-Code (ADR-014).
package api

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config oapi-codegen.yaml ../../../api/openapi.yaml
//go:generate go run ./genstubs -in api.gen.go -out stubs.gen.go
