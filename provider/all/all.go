// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package all

// Register all the available providers.
import (
	_ "github.com/juju/core/provider/azure"
	_ "github.com/juju/core/provider/ec2"
	_ "github.com/juju/core/provider/joyent"
	_ "github.com/juju/core/provider/local"
	_ "github.com/juju/core/provider/maas"
	_ "github.com/juju/core/provider/manual"
	_ "github.com/juju/core/provider/openstack"
)
