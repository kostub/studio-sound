package buildinfo

// Version is the sidecar version string. It defaults to "dev" and is
// overridden at build time via:
//
//	go build -ldflags "-X github.com/studio-sound/studio/sidecar/internal/buildinfo.Version=<ver>"
var Version = "dev"
