package version

import "github.com/hashicorp/packer-plugin-sdk/version"

var (
	// Version is the semantic plugin version, overridden at release build time.
	Version = "0.1.0"
	// VersionPrerelease is the semantic version prerelease suffix.
	VersionPrerelease = ""
	// VersionMetadata is the semantic version build metadata suffix.
	VersionMetadata = ""
	// PluginVersion is the Packer SDK representation of this plugin version.
	PluginVersion = version.NewPluginVersion(Version, VersionPrerelease, VersionMetadata)
)

// UserAgent returns the plugin product token for Verda API calls.
func UserAgent() string {
	return "packer-plugin-verda/" + PluginVersion.FormattedVersion()
}
