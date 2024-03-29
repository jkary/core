// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"os"

	"github.com/juju/core/cmd/plugins/local"

	// Import only the local provider.
	_ "github.com/juju/core/provider/local"
)

func main() {
	local.Main(os.Args)
}
