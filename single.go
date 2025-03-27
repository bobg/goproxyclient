package goproxyclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bobg/errors"
	"github.com/bobg/mid"
	"golang.org/x/mod/semver"
)

type single struct {
	baseURL string
	client  *http.Client
}

func newSingle(url string, hc *http.Client) single {
	url = strings.TrimRight(url, "/")
	if hc == nil {
		hc = &http.Client{}
	}
	return single{baseURL: url, client: hc}
}

func (s single) list(ctx context.Context, modpath string) ([]string, error) {
	q := fmt.Sprintf("%s/%s/@v/list", s.baseURL, modpath)

	req, err := http.NewRequestWithContext(ctx, "GET", q, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "creating GET %s request", q)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "in GET %s", q)
	}
	defer resp.Body.Close()

	if code := resp.StatusCode; code != http.StatusOK {
		return nil, mid.CodeErr{C: code, Err: fmt.Errorf("GET %s: %s", q, resp.Status)}
	}

	var (
		sc       = bufio.NewScanner(resp.Body)
		versions []string
	)
	for sc.Scan() {
		versions = append(versions, sc.Text())
	}
	semver.Sort(versions)
	return versions, errors.Wrapf(sc.Err(), "scanning response from GET %s", q)
}

func (s single) info(ctx context.Context, modpath, version string) (string, time.Time, map[string]json.RawMessage, error) {
	q := fmt.Sprintf("%s/%s/@v/%s.info", s.baseURL, modpath, version)
	return s.handleInfoRequest(ctx, q)
}

func (s single) mod(ctx context.Context, modpath, version string) (io.ReadCloser, error) {
	return s.getContent(ctx, modpath, version, "mod")
}

func (s single) zip(ctx context.Context, modpath, version string) (io.ReadCloser, error) {
	return s.getContent(ctx, modpath, version, "zip")
}

func (s single) getContent(ctx context.Context, modpath, version, suffix string) (io.ReadCloser, error) {
	q := fmt.Sprintf("%s/%s/@v/%s.%s", s.baseURL, modpath, version, suffix)

	req, err := http.NewRequestWithContext(ctx, "GET", q, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "creating GET %s request", q)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "in GET %s", q)
	}

	if code := resp.StatusCode; code != http.StatusOK {
		resp.Body.Close()
		return nil, mid.CodeErr{C: code, Err: fmt.Errorf("GET %s: %s", q, resp.Status)}
	}

	return resp.Body, nil
}

// Latest gets info about the latest version of a Go module.
// Its return values are the same as for [Info].
func (s single) latest(ctx context.Context, modpath string) (string, time.Time, map[string]json.RawMessage, error) {
	q := fmt.Sprintf("%s/%s/@latest", s.baseURL, modpath)
	return s.handleInfoRequest(ctx, q)
}

func (s single) handleInfoRequest(ctx context.Context, q string) (string, time.Time, map[string]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", q, nil)
	if err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "creating GET %s request", q)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "in GET %s", q)
	}
	defer resp.Body.Close()

	if code := resp.StatusCode; code != http.StatusOK {
		return "", time.Time{}, nil, mid.CodeErr{C: code, Err: fmt.Errorf("GET %s: %s", q, resp.Status)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "reading response body from GET %s", q)
	}

	var info struct {
		Version string
		Time    time.Time
	}

	if err := json.Unmarshal(body, &info); err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "unmarshaling response body from GET %s", q)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "unmarshaling response body from GET %s", q)
	}

	return info.Version, info.Time, m, nil
}
