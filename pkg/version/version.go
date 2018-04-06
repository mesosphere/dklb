package version

// Version is a version string populated by the build using -ldflags "-X
// ${PKG}/pkg/version.Version=${VERSION}".
var Version = "UNKNOWN"