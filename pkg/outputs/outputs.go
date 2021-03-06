package outputs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
)

var (
	ErrOutputsClientKeyEmpty      = errors.New("key is required but was empty")
	ErrOutputsClientValueEmpty    = errors.New("value is required but was empty")
	ErrOutputsClientTaskNameEmpty = errors.New("taskName is required but was empty")
	ErrOutputsClientEnvVarMissing = errors.New(MetadataAPIURLEnvName + " was expected but was empty")
	ErrOutputsClientNotFound      = errors.New("output was not found")
)

// OutputsClient is a client for storing task outputs in
// the nebula outputs storage.
type OutputsClient interface {
	SetOutput(ctx context.Context, key, value string) error
	GetOutput(ctx context.Context, taskName, key string) (string, error)
}

// DefaultOutputsClient uses the default net/http.Client to
// store task output values.
type DefaultOutputsClient struct {
	apiURL *url.URL
}

func (c DefaultOutputsClient) SetOutput(ctx context.Context, key, value string) error {
	if key == "" {
		return ErrOutputsClientKeyEmpty
	}

	if value == "" {
		return ErrOutputsClientValueEmpty
	}

	loc := *c.apiURL
	loc.Path = path.Join(loc.Path, key)

	buf := bytes.NewBufferString(value)

	req, err := http.NewRequest("PUT", loc.String(), buf)
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	return nil
}

func (c DefaultOutputsClient) GetOutput(ctx context.Context, taskName, key string) (string, error) {
	if key == "" {
		return "", ErrOutputsClientKeyEmpty
	}

	if taskName == "" {
		return "", ErrOutputsClientTaskNameEmpty
	}

	loc := *c.apiURL
	loc.Path = path.Join(loc.Path, taskName, key)

	req, err := http.NewRequest("GET", loc.String(), nil)
	if err != nil {
		return "", err
	}

	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return "", ErrOutputsClientNotFound
		}

		return "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	var output Output

	if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
		return "", err
	}

	v, err := output.Value.Decode()
	if err != nil {
		return "", err
	}

	return string(v), nil
}

func NewDefaultOutputsClient(location *url.URL) OutputsClient {
	return &DefaultOutputsClient{apiURL: location}
}

func NewDefaultOutputsClientFromNebulaEnv() (OutputsClient, error) {
	locStr := os.Getenv(MetadataAPIURLEnvName)

	if locStr == "" {
		return nil, ErrOutputsClientEnvVarMissing
	}

	loc, err := url.Parse(locStr)
	if err != nil {
		return nil, err
	}

	loc.Path = "/outputs"

	return NewDefaultOutputsClient(loc), nil
}
