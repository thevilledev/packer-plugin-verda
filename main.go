// packer-plugin-verda registers Verda builders with Packer.
package main

import (
	"fmt"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/thevilledev/packer-plugin-verda/builder/instance"
	verdaVersion "github.com/thevilledev/packer-plugin-verda/version"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("instance", new(instance.Builder))
	pps.SetVersion(verdaVersion.PluginVersion)

	if err := pps.Run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
