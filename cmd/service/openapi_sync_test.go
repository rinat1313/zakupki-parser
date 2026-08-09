package main

import (
	"os"
	"path/filepath"
	"testing"
)

// docs/openapi.yaml — канон; cmd/service/swagger/openapi.yaml вшивается в бинарь.
func TestOpenAPIEmbeddedMatchesDocs(t *testing.T) {
	root := filepath.Join("..", "..")
	canonical, err := os.ReadFile(filepath.Join(root, "docs", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := os.ReadFile(filepath.Join("swagger", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != string(embedded) {
		t.Fatal("docs/openapi.yaml и cmd/service/swagger/openapi.yaml разошлись — скопируйте канон в embed")
	}
}
