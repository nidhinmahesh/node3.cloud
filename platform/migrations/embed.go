package migrations

import _ "embed"

// PlatformSchema is the idempotent DDL for the platform database.
// Applied automatically on every startup via db.New().
//
//go:embed 001_platform_schema.sql
var PlatformSchema string
