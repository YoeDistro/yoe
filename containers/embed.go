// Package containers provides the embedded Dockerfile for the build container.
package containers

import _ "embed"

//go:embed Dockerfile.build
var Dockerfile string
