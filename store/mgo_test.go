// Copyright 2011, 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package store_test

import (
	"bytes"
	"os/exec"
	"time"

	"labix.org/v2/mgo"
	gc "launchpad.net/gocheck"

	"launchpad.net/juju-core/testing"
)

// ----------------------------------------------------------------------------
// The mgo test suite

type MgoSuite struct {
	Addr    string
	Session *mgo.Session
	output  bytes.Buffer
	server  *exec.Cmd
}

func (s *MgoSuite) SetUpSuite(c *gc.C) {
	mgo.SetDebug(true)
	mgo.SetStats(true)
	dbdir := c.MkDir()
	args := []string{
		"--dbpath", dbdir,
		"--bind_ip", "127.0.0.1",
		"--port", "50017",
		"--nssize", "1",
		"--noprealloc",
		"--smallfiles",
		"--nojournal",
	}
	// Look for a system mongod first, since the juju-specific mongodb
	// doesn't come with V8 so we have to skip some tests
	mongopath, err := exec.LookPath("mongod")
	if err == nil {
		c.Log("found mongodb at: %q", mongopath)
	} else {
		c.Log("failed to find mongod: %v", err)
		// We can fall back to /usr/lib/juju/bin/mongod but we should
		// disable the JS tests
		mongopath, err = exec.LookPath("/usr/lib/juju/bin/mongod")
		c.Log("Tests requiring MongoDB Javascript will be skipped")
		*noTestMongoJs = true
	}
	c.Assert(err, gc.IsNil)
	s.server = exec.Command(mongopath, args...)
	s.server.Stdout = &s.output
	s.server.Stderr = &s.output
	err = s.server.Start()
	c.Assert(err, gc.IsNil)
}

func (s *MgoSuite) TearDownSuite(c *gc.C) {
	if s.server != nil && s.server.Process != nil {
		s.server.Process.Kill()
		s.server.Process.Wait()
		s.server.Process = nil
	}
}

func (s *MgoSuite) SetUpTest(c *gc.C) {
	err := DropAll("localhost:50017")
	c.Assert(err, gc.IsNil)
	mgo.SetLogger(c)
	mgo.ResetStats()
	s.Addr = "127.0.0.1:50017"
	s.Session, err = mgo.Dial(s.Addr)
	c.Assert(err, gc.IsNil)
}

func (s *MgoSuite) TearDownTest(c *gc.C) {
	if s.Session != nil {
		s.Session.Close()
	}
	t0 := time.Now()
	for i := 0; ; i++ {
		stats := mgo.GetStats()
		if stats.SocketsInUse == 0 && stats.SocketsAlive == 0 {
			break
		}
		if time.Since(t0) > testing.LongWait {
			// We wait up to 10s for all workers to finish up
			c.Fatal("Test left sockets in a dirty state")
		}
		c.Logf("Waiting for sockets to die: %d in use, %d alive", stats.SocketsInUse, stats.SocketsAlive)
		time.Sleep(testing.ShortWait)
	}
}

func DropAll(mongourl string) (err error) {
	session, err := mgo.Dial(mongourl)
	if err != nil {
		return err
	}
	defer session.Close()

	names, err := session.DatabaseNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		switch name {
		case "admin", "local", "config":
		default:
			err = session.DB(name).DropDatabase()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
