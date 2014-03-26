// Copyright 2012-2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package charm_test

import (
	"io/ioutil"
	"path/filepath"

	gc "launchpad.net/gocheck"

	corecharm "launchpad.net/juju-core/charm"
	"launchpad.net/juju-core/testing/testbase"
	"launchpad.net/juju-core/worker/uniter/charm"
)

type ManifestDeployerSuite struct {
	testbase.LoggingSuite
	bundles    *bundleReader
	targetPath string
	deployer   charm.Deployer
}

var _ = gc.Suite(&ManifestDeployerSuite{})

func (s *ManifestDeployerSuite) SetUpTest(c *gc.C) {
	s.LoggingSuite.SetUpTest(c)
	s.bundles = &bundleReader{}
	s.targetPath = filepath.Join(c.MkDir(), "target")
	deployerPath := filepath.Join(c.MkDir(), "deployer")
	s.deployer = charm.NewManifestDeployer(s.targetPath, deployerPath, s.bundles)
}

func (s *ManifestDeployerSuite) charmURL(revision int) *corecharm.URL {
	baseURL := corecharm.MustParseURL("cs:s/c")
	return baseURL.WithRevision(revision)
}

func (s *ManifestDeployerSuite) addCharm(c *gc.C, revision int, customise func(path string)) charm.BundleInfo {
	return s.bundles.AddCustomBundle(c, s.charmURL(revision), customise)
}

func (s *ManifestDeployerSuite) addMockCharm(c *gc.C, revision int, bundle charm.Bundle) charm.BundleInfo {
	return s.bundles.AddBundle(c, s.charmURL(revision), bundle)
}

func (s *ManifestDeployerSuite) assertCharm(c *gc.C, revision int) {
	url, err := charm.ReadCharmURL(filepath.Join(s.targetPath, ".juju-charm"))
	c.Assert(err, gc.IsNil)
	c.Assert(url, gc.DeepEquals, s.charmURL(revision))
}

func (s *ManifestDeployerSuite) assertFile(c *gc.C, path, content string) {
	data, err := ioutil.ReadFile(filepath.Join(s.targetPath, path))
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, content)
}

func (s *ManifestDeployerSuite) TestAbortStageWhenClosed(c *gc.C) {
	info := s.addMockCharm(c, 1, mockBundle{})
	abort := make(chan struct{})
	errors := make(chan error)
	s.bundles.SetAbortWait()
	go func() {
		errors <- s.deployer.Stage(info, abort)
	}()
	close(abort)
	err := <-errors
	c.Assert(err, gc.ErrorMatches, "charm read aborted")
}

func (s *ManifestDeployerSuite) TestDontAbortStageWhenNotClosed(c *gc.C) {
	info := s.addMockCharm(c, 1, mockBundle{})
	abort := make(chan struct{})
	errors := make(chan error)
	wait := s.bundles.SetAbortWait()
	go func() {
		errors <- s.deployer.Stage(info, abort)
	}()
	close(wait)
	err := <-errors
	c.Assert(err, gc.IsNil)
}

func (s *ManifestDeployerSuite) TestDeployWithoutStage(c *gc.C) {
	err := s.deployer.Deploy()
	c.Assert(err, gc.ErrorMatches, "charm deployment failed: no charm set")
}

func (s *ManifestDeployerSuite) TestInstall(c *gc.C) {
	// Prepare.
	info := s.addCharm(c, 1, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "some-file"), []byte("hello"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err := s.deployer.Stage(info, nil)
	c.Assert(err, gc.IsNil)

	// Install.
	err = s.deployer.Deploy()
	c.Assert(err, gc.IsNil)
	s.assertCharm(c, 1)
	s.assertFile(c, "some-file", "hello")
}

func (s *ManifestDeployerSuite) TestSimpleUpgrade(c *gc.C) {
	// Install base.
	info1 := s.addCharm(c, 1, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "some-file"), []byte("hello"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err := s.deployer.Stage(info1, nil)
	c.Assert(err, gc.IsNil)
	err = s.deployer.Deploy()
	c.Assert(err, gc.IsNil)
	s.assertCharm(c, 1)

	// Upgrade.
	info2 := s.addCharm(c, 2, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "some-file"), []byte("goodbye"), 0644)
		c.Assert(err, gc.IsNil)
		err = ioutil.WriteFile(filepath.Join(path, "another-file"), []byte("ahoy-hoy"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err = s.deployer.Stage(info2, nil)
	c.Assert(err, gc.IsNil)
	err = s.deployer.Deploy()
	c.Assert(err, gc.IsNil)
	s.assertCharm(c, 2)

	// Check content.
	s.assertFile(c, "some-file", "goodbye")
	s.assertFile(c, "another-file", "ahoy-hoy")
}

func (s *ManifestDeployerSuite) TestComplexUpgrade(c *gc.C) {
	// Install base.
	info1 := s.addCharm(c, 1, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "charm-file"), []byte("x"), 0644)
		c.Assert(err, gc.IsNil)
		err := os.Symlink("some-file", filepath.Join(path, "charm-symlink"))
		c.Assert(err, gc.IsNil)
		err := os.MkdirAll(filepath.Join(path, "charm-remove-dir"), 0755)
		c.Assert(err, gc.IsNil)
		err := os.MkdirAll(filepath.Join(path, "charm-dir"), 0755)
		c.Assert(err, gc.IsNil)
		err := ioutil.WriteFile(filepath.Join(path, "charm-dir", "charm-file"), []byte("y"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err := s.deployer.Stage(info1, nil)
	c.Assert(err, gc.IsNil)
	err = s.deployer.Deploy()
	c.Assert(err, gc.IsNil)
	s.assertCharm(c, 1)

	// Add user files that will not be removed.
	err := ioutil.WriteFile(filepath.Join(path, "user-file"), []byte("z"), 0644)
	c.Assert(err, gc.IsNil)
	err := os.Symlink("user-file", filepath.Join(path, "user-symlink"))
	c.Assert(err, gc.IsNil)
	err := os.MkdirAll(filepath.Join(path, "user-dir"), 0755)
	c.Assert(err, gc.IsNil)
	err := ioutil.WriteFile(filepath.Join(path, "user-dir", "user-file"), []byte("a"), 0644)
	c.Assert(err, gc.IsNil)

	// Add user files that will be removed.
	err := ioutil.WriteFile(filepath.Join(path, "charm-dir", "user-file"), []byte("b"), 0644)
	c.Assert(err, gc.IsNil)
	err := ioutil.WriteFile(filepath.Join(path, "charm-remove-dir", "user-file"), []byte("c"), 0644)
	c.Assert(err, gc.IsNil)

	// Upgrade with all file types changed.
	info2 := s.addCharm(c, 2, func(path string) {
		err := ioutil.WriteFile(filepath.Join(path, "charm-dir"), []byte("d"), 0644)
		c.Assert(err, gc.IsNil)
		err := os.Symlink("charm-file", filepath.Join(path, "charm-dir"))
		c.Assert(err, gc.IsNil)
		err := os.MkdirAll(filepath.Join(path, "charm-file"), 0755)
		c.Assert(err, gc.IsNil)
		err := ioutil.WriteFile(filepath.Join(path, "charm-file", "charm-file"), []byte("e"), 0644)
		c.Assert(err, gc.IsNil)
	})
	err := s.deployer.Stage(info1, nil)
	c.Assert(err, gc.IsNil)
	err = s.deployer.Deploy()
	c.Assert(err, gc.IsNil)
	s.assertCharm(c, 1)

	// Check new files are all in place.

}

func (s *ManifestDeployerSuite) TestUpgradeConflictResolveRetrySameCharm(c *gc.C) {
	c.Fatalf("not finished")
}

func (s *ManifestDeployerSuite) TestUpgradeConflictRevertRetryDifferentCharm(c *gc.C) {
	c.Fatalf("not finished")
}
