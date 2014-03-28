// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package api

import (
	"crypto/tls"
	"crypto/x509"
	"time"

	"code.google.com/p/go.net/websocket"

	"launchpad.net/juju-core/cert"
	"launchpad.net/juju-core/instance"
	"launchpad.net/juju-core/log"
	"launchpad.net/juju-core/rpc"
	"launchpad.net/juju-core/rpc/jsoncodec"
	"launchpad.net/juju-core/state/api/params"
	"launchpad.net/juju-core/utils"
)

// PingPeriod defines how often the internal connection health check
// will run. It's a variable so it can be changed in tests.
var PingPeriod = 1 * time.Minute

type State struct {
	client *rpc.Conn
	conn   *websocket.Conn

	// addr is the address used to connect to the API server.
	addr string

	// hostPorts is the API server addresses returned from Login,
	// which the client may cache and use for failover.
	hostPorts [][]instance.HostPort

	// authTag holds the authenticated entity's tag after login.
	authTag string

	// broken is a channel that gets closed when the connection is
	// broken.
	broken chan struct{}

	// tag and password hold the cached login credentials.
	tag      string
	password string
	// serverRoot holds the cached API server address and port we used
	// to login, with a https:// prefix.
	serverRoot string
}

// Info encapsulates information about a server holding juju state and
// can be used to make a connection to it.
type Info struct {
	// Addrs holds the addresses of the state servers.
	Addrs []string

	// CACert holds the CA certificate that will be used
	// to validate the state server's certificate, in PEM format.
	CACert []byte

	// Tag holds the name of the entity that is connecting.
	// If this and the password are empty, no login attempt will be made
	// (this is to allow tests to access the API to check that operations
	// fail when not logged in).
	Tag string

	// Password holds the password for the administrator or connecting entity.
	Password string

	// Nonce holds the nonce used when provisioning the machine. Used
	// only by the machine agent.
	Nonce string `yaml:",omitempty"`
}

// DialOpts holds configuration parameters that control the
// Dialing behavior when connecting to a state server.
type DialOpts struct {
	// Timeout is the amount of time to wait contacting
	// a state server.
	Timeout time.Duration

	// RetryDelay is the amount of time to wait between
	// unsucssful connection attempts.
	RetryDelay time.Duration
}

// DefaultDialOpts returns a DialOpts representing the default
// parameters for contacting a state server.
func DefaultDialOpts() DialOpts {
	return DialOpts{
		Timeout:    10 * time.Minute,
		RetryDelay: 2 * time.Second,
	}
}

func Open(info *Info, opts DialOpts) (*State, error) {
	// TODO Select a random address from info.Addrs
	// and only fail when we've tried all the addresses.
	// TODO what does "origin" really mean, and is localhost always ok?
	cfg, err := websocket.NewConfig("wss://"+info.Addrs[0]+"/", "http://localhost/")
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	xcert, err := cert.ParseCert(info.CACert)
	if err != nil {
		return nil, err
	}
	pool.AddCert(xcert)
	cfg.TlsConfig = &tls.Config{
		RootCAs:    pool,
		ServerName: "anything",
	}
	var conn *websocket.Conn
	openAttempt := utils.AttemptStrategy{
		Total: opts.Timeout,
		Delay: opts.RetryDelay,
	}
	for a := openAttempt.Start(); a.Next(); {
		log.Infof("state/api: dialing %q", cfg.Location)
		conn, err = websocket.DialConfig(cfg)
		if err == nil {
			break
		}
		log.Errorf("state/api: %v", err)
	}
	if err != nil {
		return nil, err
	}
	log.Infof("state/api: connection established")

	client := rpc.NewConn(jsoncodec.NewWebsocket(conn), nil)
	client.Start()
	st := &State{
		client:     client,
		conn:       conn,
		addr:       cfg.Location.Host,
		serverRoot: "https://" + cfg.Location.Host,
		tag:        info.Tag,
		password:   info.Password,
	}
	if info.Tag != "" || info.Password != "" {
		if err := st.Login(info.Tag, info.Password, info.Nonce); err != nil {
			conn.Close()
			return nil, err
		}
	}
	st.broken = make(chan struct{})
	go st.heartbeatMonitor()
	return st, nil
}

func (s *State) heartbeatMonitor() {
	for {
		if err := s.Ping(); err != nil {
			close(s.broken)
			return
		}
		time.Sleep(PingPeriod)
	}
}

func (s *State) Ping() error {
	return s.Call("Pinger", "", "Ping", nil, nil)
}

// Call invokes a low-level RPC method of the given objType, id, and
// request, passing the given parameters and filling in the response
// results. This should not be used directly by clients.
// TODO (dimitern) Add tests for all client-facing objects to verify
// we return the correct error when invoking Call("Object",
// "non-empty-id",...)
func (s *State) Call(objType, id, request string, args, response interface{}) error {
	err := s.client.Call(rpc.Request{
		Type:   objType,
		Id:     id,
		Action: request,
	}, args, response)
	return params.ClientError(err)
}

func (s *State) Close() error {
	return s.client.Close()
}

// Broken returns a channel that's closed when the connection is broken.
func (s *State) Broken() <-chan struct{} {
	return s.broken
}

// RPCClient returns the RPC client for the state, so that testing
// functions can tickle parts of the API that the conventional entry
// points don't reach. This is exported for testing purposes only.
func (s *State) RPCClient() *rpc.Conn {
	return s.client
}

// Addr returns the address used to connect to the API server.
func (s *State) Addr() string {
	return s.addr
}

// HostPorts returns addresses that may be used to connect to
// the API server, including the address used to connect.
//
// The addresses are scoped (public, cloud-internal, etc.), so
// the client may choose which addresses to attempt. For the
// Juju CLI, all addresses must be attempted, as the CLI may
// be invoked both within and outside the environment (think
// private clouds).
func (s *State) HostPorts() [][]instance.HostPort {
	return append([][]instance.HostPort{}, s.hostPorts...)
}
