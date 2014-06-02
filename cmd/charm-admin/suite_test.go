// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	stdtesting "testing"

	"github.com/juju/core/testing"
)

func Test(t *stdtesting.T) {
	testing.MgoTestPackageSsl(t, false)
}
