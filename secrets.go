package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/hashicorp/vault/api"
)

type secret struct {
	name  string
	path  string
	key   string
	value string
}

func (s *secret) String() string {
	displayVal := ""
	if s.value != "" {
		displayVal = "[...]"
	}

	return fmt.Sprintf("(%s): %s %s=%s", s.name, s.path, s.key, displayVal)
}

func fetchSecrets(paths string, client *api.Client) ([]*secret, error) {
	secrets, err := parsePaths(paths)
	if err != nil {
		return nil, err
	}

	for _, s := range secrets {
		glog.V(6).Infof("Fetching secret path=%s key=%s", s.path, s.key)
		if err := fetchSecret(s, client); err != nil {
			return nil, err
		}
	}

	return secrets, nil
}

func parsePaths(paths string) ([]*secret, error) {
	results := make([]*secret, 0)
	p := strings.Split(paths, ",")
	for _, e := range p {
		e = strings.TrimSpace(e)

		toks := strings.Split(e, "#")
		if len(toks) != 3 {
			return nil, fmt.Errorf("Invalid entry %s", e)
		}

		s := &secret{
			path: toks[0],
			key:  toks[1],
			name: toks[2],
		}

		results = append(results, s)
	}

	return results, nil
}

func fetchSecret(s *secret, client *api.Client) error {
	resp, err := client.Logical().Read(s.path)
	if err != nil {
		glog.V(6).Infof("Failed to fetch secret %s from %s", s.String(), client.Address())
		return err
	}

	if resp == nil {
		glog.V(3).Infof("No entry found at path %s", s.path)
		return fmt.Errorf("Secret (%s) not found", s.path)
	}

	// Vault v1 KV returns secrets in "data"
	// Vault v2 KV returns secrets in "data/data"
	data := resp.Data
	if resp.Data["data"] != nil {
		data = resp.Data["data"].(map[string]interface{})
	}

	glog.V(6).Infof("Found %d entries at path %s", len(data), s.path)

	val, ok := data[s.key]
	if !ok {
		glog.V(3).Infof("No entry found at path %s and key %s", s.path, s.key)
		return fmt.Errorf("Secret (%s#%s) not found", s.path, s.key)
	}

	s.value = val.(string)
	glog.Infof("Got secret: %s", s.String())
	return nil
}

func writeSecrets(secrets []*secret, location string) error {
	dir := filepath.Dir(location)
	os.MkdirAll(dir, 0755)

	f, err := os.Create(location)
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f)
	for _, s := range secrets {
		w.WriteString(fmt.Sprintf("%s=%s\n", s.name, s.value))
	}

	w.Flush()
	glog.Infof("Wrote %d secrets to %s", len(secrets), location)
	return nil
}
