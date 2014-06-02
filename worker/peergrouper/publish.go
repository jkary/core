// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package peergrouper

import (
	"github.com/juju/core/instance"
	"github.com/juju/core/state"
)

type publisher struct {
	st *state.State
}

func newPublisher(st *state.State) *publisher {
	return &publisher{
		st: st,
	}
}

func (pub *publisher) publishAPIServers(apiServers [][]instance.HostPort, instanceIds []instance.Id) error {
	// TODO(rog) publish instanceIds in environment storage.
	return pub.st.SetAPIHostPorts(apiServers)
}
