/*
Copyright 2022 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/fluxcd/pkg/oci"
)

// ParseArtifactURL validates the OCI URL and returns the address of the artifact.
func ParseArtifactURL(ociURL string) (string, error) {
	if !strings.HasPrefix(ociURL, oci.OCIRepositoryPrefix) {
		return "", fmt.Errorf("URL must be in format 'oci://<domain>/<org>/<repo>'")
	}

	url := strings.TrimPrefix(ociURL, oci.OCIRepositoryPrefix)
	if _, err := name.ParseReference(url); err != nil {
		return "", fmt.Errorf("'%s' invalid URL: %w", ociURL, err)
	}

	return url, nil
}

// ParseRepositoryURL validates the OCI URL and returns the address of the artifact repository.
func ParseRepositoryURL(ociURL string) (string, error) {
	if !strings.HasPrefix(ociURL, oci.OCIRepositoryPrefix) {
		return "", fmt.Errorf("URL must be in format 'oci://<domain>/<org>/<repo>'")
	}

	url := strings.TrimPrefix(ociURL, oci.OCIRepositoryPrefix)
	ref, err := name.ParseReference(url)
	if err != nil {
		return "", fmt.Errorf("'%s' invalid URL: %w", ociURL, err)
	}

	return ref.Context().Name(), nil
}
