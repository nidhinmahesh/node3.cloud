// Package nodeutil provides shared helpers for locating rubixgoplatform nodes.
package nodeutil

import (
	"fmt"
	"os"
)

const BasePort = 20000

// URL returns the HTTP base URL for the rubixgoplatform node at the given index.
// In Docker deployments set RUBIX_NODE_HOST to the container service name (e.g. "node0").
func URL(index int) string {
	host := os.Getenv("RUBIX_NODE_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d", host, BasePort+index)
}
