// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package local_test

import (
	"fmt"
	"net"
	stdtesting "testing"

	gc "launchpad.net/gocheck"

	"github.com/juju/core/environs"
	"github.com/juju/core/provider"
	"github.com/juju/core/provider/local"
	coretesting "github.com/juju/core/testing"
)

func TestLocal(t *stdtesting.T) {
	gc.TestingT(t)
}

type localSuite struct {
	coretesting.BaseSuite
}

var _ = gc.Suite(&localSuite{})

func (*localSuite) TestProviderRegistered(c *gc.C) {
	provider, error := environs.Provider(provider.Local)
	c.Assert(error, gc.IsNil)
	c.Assert(provider, gc.DeepEquals, local.Provider)
}

func (*localSuite) TestCheckLocalPort(c *gc.C) {
	// Listen on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, gc.IsNil)
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	checkLocalPort := *local.CheckLocalPort
	err = checkLocalPort(port, "test port")
	c.Assert(err, gc.ErrorMatches, fmt.Sprintf("cannot use %d as test port, already in use", port))

	ln.Close()
	err = checkLocalPort(port, "test port, no longer in use")
	c.Assert(err, gc.IsNil)
}
