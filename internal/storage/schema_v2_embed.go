package storage

import _ "embed"

//go:embed schema_v2.sql
var schemaV2Raw string

// schemaV2Content returns the embedded schema SQL.
func schemaV2Content() (string, error) {
	return schemaV2Raw, nil
}
