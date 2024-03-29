// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package testing

import (
	"io"

	gc "launchpad.net/gocheck"

	"github.com/juju/core/environs/filestorage"
	"github.com/juju/core/environs/httpstorage"
	"github.com/juju/core/environs/storage"
)

// CreateLocalTestStorage returns the listener, which needs to be closed, and
// the storage that is backed by a directory created in the running test's temp
// directory.
func CreateLocalTestStorage(c *gc.C) (closer io.Closer, stor storage.Storage, dataDir string) {
	dataDir = c.MkDir()
	underlying, err := filestorage.NewFileStorageWriter(dataDir)
	c.Assert(err, gc.IsNil)
	listener, err := httpstorage.Serve("localhost:0", underlying)
	c.Assert(err, gc.IsNil)
	stor = httpstorage.Client(listener.Addr().String())
	closer = listener
	return
}
