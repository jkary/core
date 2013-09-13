// Copyright 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package simplestreams

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"launchpad.net/juju-core/errors"
)

// A DataSource retrieves simplestreams metadata.
type DataSource interface {
	// Fetch loads the data at the specified relative path. It returns a reader from which
	// the data can be retrieved as well as the full URL of the file. The full URL is typically
	// used in log messages to help diagnose issues accessing the data.
	Fetch(path string) (io.ReadCloser, string, error)
	// URL returns the full URL of the path, as applicable to this datasource.
	// This method is used primarily for logging purposes.
	URL(path string) (string, error)
}

// A urlDataSource retrieves data from an HTTP URL.
type urlDataSource struct {
	baseURL string
}

// NewURLDataSource returns a new datasource reading from the specified baseURL.
func NewURLDataSource(baseURL string) DataSource {
	return &urlDataSource{baseURL}
}

// urlJoin returns baseURL + relpath making sure to have a '/' inbetween them
// This doesn't try to do anything fancy with URL query or parameter bits
// It also doesn't use path.Join because that normalizes slashes, and you need
// to keep both slashes in 'http://'.
func urlJoin(baseURL, relpath string) string {
	if strings.HasSuffix(baseURL, "/") {
		return baseURL + relpath
	}
	return baseURL + "/" + relpath
}

// Fetch is defined in simplestreams.DataSource.
func (h *urlDataSource) Fetch(path string) (io.ReadCloser, string, error) {
	dataURL := urlJoin(h.baseURL, path)
	// dataURL can be http:// or file://
	resp, err := urlClient.Get(dataURL)
	if err != nil {
		return nil, dataURL, errors.NotFoundf("invalid URL %q", dataURL)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, dataURL, errors.NotFoundf("cannot find URL %q", dataURL)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, dataURL, errors.Unauthorizedf("unauthorised access to URL %q", dataURL)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, dataURL, fmt.Errorf("cannot access URL %q, %q", dataURL, resp.Status)
	}
	return resp.Body, dataURL, nil
}

// URL is defined in simplestreams.DataSource.
func (h *urlDataSource) URL(path string) (string, error) {
	return urlJoin(h.baseURL, path), nil
}