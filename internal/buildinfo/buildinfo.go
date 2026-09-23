// Package buildinfo carries values stamped into the binary at link time.
package buildinfo

// Version is the release version, set via -ldflags at build time.
var Version = "dev"

// Commit is the source revision the binary was built from.
var Commit = "none"

// Date is the commit timestamp of that revision, so rebuilding a tag
// reproduces the same binary.
var Date = "unknown"
