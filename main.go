package main

import (
	"github.com/hashicorp/terraform/builtin/providers/bigip"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: bigip.Provider,
	})
}
