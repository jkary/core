// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package testing

import (
	"github.com/juju/loggo"
	"github.com/juju/testing"

	"github.com/juju/core/environs"
	"github.com/juju/core/environs/cloudinit"
	"github.com/juju/core/instance"
	"github.com/juju/core/provider/common"
	"github.com/juju/core/utils/ssh"
)

var logger = loggo.GetLogger("juju.environs.testing")

// DisableFinishBootstrap disables common.FinishBootstrap so that tests
// do not attempt to SSH to non-existent machines. The result is a function
// that restores finishBootstrap.
func DisableFinishBootstrap() func() {
	f := func(environs.BootstrapContext, ssh.Client, instance.Instance, *cloudinit.MachineConfig) error {
		logger.Warningf("provider/common.FinishBootstrap is disabled")
		return nil
	}
	return testing.PatchValue(&common.FinishBootstrap, f)
}
