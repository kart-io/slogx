package slogx

import "github.com/kart-io/version"

// resolveVersion returns the dynamic version string.
//
// Build-time injection goes through the kart-io/version package:
// -ldflags "-X 'github.com/kart-io/version.gitVersion=$(git describe --tags --always)'"
func resolveVersion(userVer string) string {
	if userVer != "" {
		return userVer
	}
	return version.Get().GitVersion
}
