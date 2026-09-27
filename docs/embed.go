package docs

import _ "embed"

//go:embed openapi.yaml
var OpenAPI []byte

//go:embed redoc.html
var ReDocHTML []byte
