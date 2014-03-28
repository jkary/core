// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package apiserver_test

import (
	"net"
	"strconv"

	"github.com/juju/loggo"
	jc "github.com/juju/testing/checkers"
	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/instance"
	jujutesting "launchpad.net/juju-core/juju/testing"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/state/api"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/state/apiserver"
	coretesting "launchpad.net/juju-core/testing"
	"launchpad.net/juju-core/utils"
)

type loginSuite struct {
	jujutesting.JujuConnSuite
}

var _ = gc.Suite(&loginSuite{})

var badLoginTests = []struct {
	tag      string
	password string
	err      string
	code     string
}{{
	tag:      "user-admin",
	password: "wrong password",
	err:      "invalid entity name or password",
	code:     params.CodeUnauthorized,
}, {
	tag:      "user-foo",
	password: "password",
	err:      "invalid entity name or password",
	code:     params.CodeUnauthorized,
}, {
	tag:      "bar",
	password: "password",
	err:      `"bar" is not a valid tag`,
}}

func (s *loginSuite) setupServer(c *gc.C) (*api.Info, func()) {
	srv, err := apiserver.NewServer(
		s.State,
		"localhost:0",
		[]byte(coretesting.ServerCert),
		[]byte(coretesting.ServerKey),
		"",
	)
	c.Assert(err, gc.IsNil)
	info := &api.Info{
		Tag:      "",
		Password: "",
		Addrs:    []string{srv.Addr()},
		CACert:   []byte(coretesting.CACert),
	}
	return info, func() {
		err := srv.Stop()
		c.Assert(err, gc.IsNil)
	}
}

func (s *loginSuite) TestBadLogin(c *gc.C) {
	// Start our own server so we can control when the first login
	// happens. Otherwise in JujuConnSuite.SetUpTest api.Open is
	// called with user-admin permissions automatically.
	info, cleanup := s.setupServer(c)
	defer cleanup()

	for i, t := range badLoginTests {
		c.Logf("test %d; entity %q; password %q", i, t.tag, t.password)
		// Note that Open does not log in if the tag and password
		// are empty. This allows us to test operations on the connection
		// before calling Login, which we could not do if Open
		// always logged in.
		info.Tag = ""
		info.Password = ""
		func() {
			st, err := api.Open(info, fastDialOpts)
			c.Assert(err, gc.IsNil)
			defer st.Close()

			_, err = st.Machiner().Machine("0")
			c.Assert(err, gc.ErrorMatches, `unknown object type "Machiner"`)

			// Since these are user login tests, the nonce is empty.
			err = st.Login(t.tag, t.password, "")
			c.Assert(err, gc.ErrorMatches, t.err)
			c.Assert(params.ErrCode(err), gc.Equals, t.code)

			_, err = st.Machiner().Machine("0")
			c.Assert(err, gc.ErrorMatches, `unknown object type "Machiner"`)
		}()
	}
}

func (s *loginSuite) TestLoginAsDeactivatedUser(c *gc.C) {
	info, cleanup := s.setupServer(c)
	defer cleanup()

	info.Tag = ""
	info.Password = ""
	st, err := api.Open(info, fastDialOpts)
	c.Assert(err, gc.IsNil)
	defer st.Close()
	u, err := s.State.AddUser("inactive", "password")
	c.Assert(err, gc.IsNil)
	err = u.Deactivate()
	c.Assert(err, gc.IsNil)

	_, err = st.Client().Status([]string{})
	c.Assert(err, gc.ErrorMatches, `unknown object type "Client"`)

	// Since these are user login tests, the nonce is empty.
	err = st.Login("user-inactive", "password", "")
	c.Assert(err, gc.ErrorMatches, "invalid entity name or password")

	_, err = st.Client().Status([]string{})
	c.Assert(err, gc.ErrorMatches, `unknown object type "Client"`)
}

func (s *loginSuite) TestLoginSetsLogIdentifier(c *gc.C) {
	info, cleanup := s.setupServer(c)
	defer cleanup()

	machineInState, err := s.State.AddMachine("quantal", state.JobHostUnits)
	c.Assert(err, gc.IsNil)
	err = machineInState.SetProvisioned("foo", "fake_nonce", nil)
	c.Assert(err, gc.IsNil)
	password, err := utils.RandomPassword()
	c.Assert(err, gc.IsNil)
	err = machineInState.SetPassword(password)
	c.Assert(err, gc.IsNil)
	c.Assert(machineInState.Tag(), gc.Equals, "machine-0")

	tw := &loggo.TestWriter{}
	c.Assert(loggo.RegisterWriter("login-tester", tw, loggo.DEBUG), gc.IsNil)
	defer loggo.RemoveWriter("login-tester")

	info.Tag = machineInState.Tag()
	info.Password = password
	info.Nonce = "fake_nonce"

	apiConn, err := api.Open(info, fastDialOpts)
	c.Assert(err, gc.IsNil)
	apiMachine, err := apiConn.Machiner().Machine(machineInState.Tag())
	c.Assert(err, gc.IsNil)
	c.Assert(apiMachine.Tag(), gc.Equals, machineInState.Tag())
	apiConn.Close()

	c.Assert(tw.Log, jc.LogMatches, []string{
		`<- \[[0-9A-F]+\] <unknown> {"RequestId":1,"Type":"Admin","Request":"Login","Params":` +
			`{"AuthTag":"machine-0","Password":"[^"]*","Nonce":"fake_nonce"}` +
			`}`,
		// Now that we are logged in, we see the entity's tag
		// [0-9.umns] is to handle timestamps that are ns, us, ms, or s
		// long, though we expect it to be in the 'ms' range.
		`-> \[[0-9A-F]+\] machine-0 [0-9.]+[umn]?s {"RequestId":1,"Response":{"Servers":\[\]}} Admin\[""\].Login`,
		`<- \[[0-9A-F]+\] machine-0 {"RequestId":2,"Type":"Machiner","Request":"Life","Params":{"Entities":\[{"Tag":"machine-0"}\]}}`,
		`-> \[[0-9A-F]+\] machine-0 [0-9.umns]+ {"RequestId":2,"Response":{"Results":\[{"Life":"alive","Error":null}\]}} Machiner\[""\]\.Life`,
	})
}

func (s *loginSuite) TestLoginAddrs(c *gc.C) {
	machine, err := s.State.AddMachine("quantal", state.JobHostUnits)
	c.Assert(err, gc.IsNil)
	err = machine.SetProvisioned("foo", "fake_nonce", nil)
	c.Assert(err, gc.IsNil)
	password, err := utils.RandomPassword()
	c.Assert(err, gc.IsNil)
	err = machine.SetPassword(password)
	c.Assert(err, gc.IsNil)
	info, cleanup := s.setupServer(c)
	defer cleanup()
	info.Tag = machine.Tag()
	info.Password = password
	info.Nonce = "fake_nonce"

	// Initially just the address we connect with is returned,
	// despite there being no APIHostPorts in state.
	connectedAddr, hostPorts := s.loginHostPorts(c, info)
	connectedAddrHost, connectedAddrPortString, err := net.SplitHostPort(connectedAddr)
	c.Assert(err, gc.IsNil)
	connectedAddrPort, err := strconv.Atoi(connectedAddrPortString)
	c.Assert(err, gc.IsNil)
	connectedAddrHostPorts := [][]instance.HostPort{
		[]instance.HostPort{{
			instance.NewAddress(connectedAddrHost),
			connectedAddrPort,
		}},
	}
	c.Assert(hostPorts, gc.DeepEquals, connectedAddrHostPorts)

	// After storing APIHostPorts in state, Login should store
	// all of them and the address we connected with.
	server1Addresses := []instance.Address{{
		Value:        "server-1",
		Type:         instance.HostName,
		NetworkScope: instance.NetworkPublic,
	}, {
		Value:        "10.0.0.1",
		Type:         instance.Ipv4Address,
		NetworkName:  "internal",
		NetworkScope: instance.NetworkCloudLocal,
	}}
	server2Addresses := []instance.Address{{
		Value:        "::1",
		Type:         instance.Ipv6Address,
		NetworkName:  "loopback",
		NetworkScope: instance.NetworkMachineLocal,
	}}
	stateAPIHostPorts := [][]instance.HostPort{
		instance.AddressesWithPort(server1Addresses, 123),
		instance.AddressesWithPort(server2Addresses, 456),
	}
	err = s.State.SetAPIHostPorts(stateAPIHostPorts)
	c.Assert(err, gc.IsNil)
	connectedAddr, hostPorts = s.loginHostPorts(c, info)
	stateAPIHostPorts = append(stateAPIHostPorts, connectedAddrHostPorts...)
	c.Assert(hostPorts, gc.DeepEquals, stateAPIHostPorts)
}

func (s *loginSuite) loginHostPorts(c *gc.C, info *api.Info) (connectedAddr string, hostPorts [][]instance.HostPort) {
	st, err := api.Open(info, fastDialOpts)
	c.Assert(err, gc.IsNil)
	defer st.Close()
	return st.Addr(), st.HostPorts()
}
