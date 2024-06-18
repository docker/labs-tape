package testdata

import (
	"path/filepath"
	"testing"

	"github.com/errordeveloper/tape/manifest/types"
)

type TestCase struct {
	Description    string
	Directory      string
	Manifests      []string
	NumRelatedTags int
	Expected       []types.Image
}

type TestCases []TestCase

func (tcs TestCases) Run(t *testing.T, pathToRootDir string, doTest func(tc TestCase) func(t *testing.T)) {
	t.Helper()
	tcs.makeRelativeTo(pathToRootDir)
	for i := range tcs {
		t.Run(tcs[i].Description, doTest(tcs[i]))
	}
}
func (tcs TestCases) makeRelativeTo(dir string) {
	for i := range tcs {
		tcs[i].Directory = filepath.Join(dir, tcs[i].Directory)
	}
}

func BasicJSONCases() TestCases {
	return []TestCase{{
		Description: "basic",
		Directory:   "manifest/testdata/basic",
		Manifests: []string{
			"list.json",
			"deployment.json",
		},
		Expected: []types.Image{
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "list.json",
						ManifestDigest: "577caeee80cfa690caf25bcdd4b1919b99d2860eb351c48e81b46b9e4b52aea5",
						NodePath:       []string{"spec", "containers", "image"},
						Line:           15,
						Column:         34,
					},
					OriginalRef: "nginx",
				}},
				OriginalName: "nginx",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "list.json",
						ManifestDigest: "577caeee80cfa690caf25bcdd4b1919b99d2860eb351c48e81b46b9e4b52aea5",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           63,
						Column:         42,
					},
					OriginalRef: "redis",
				}},
				OriginalName: "redis",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "list.json",
						ManifestDigest: "577caeee80cfa690caf25bcdd4b1919b99d2860eb351c48e81b46b9e4b52aea5",
						NodePath:       []string{"items", "spec", "containers", "image"},
						Line:           106,
						Column:         42,
					},
					OriginalRef: "redis",
				}},
				OriginalName: "redis",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "deployment.json",
						ManifestDigest: "8d85ce5a5de4085bb841cee0402022fcd03f86606a67d572e62012ce4420668c",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           11,
						Column:         16,
					},
					OriginalRef: "nginx:1.16.1",
				}},
				OriginalName: "nginx",
				OriginalTag:  "1.16.1",
			},
		},
	}}
}

var baseYAMLCases = []TestCase{
	{
		Description: "contour",
		Directory:   "manifest/testdata/contour",
		Manifests: []string{
			"00-common.yaml",
			"00-crds.yaml",
			"01-contour-config.yaml",
			"01-crds.yaml",
			"02-job-certgen.yaml",
			"02-rbac.yaml",
			"02-role-contour.yaml",
			"02-service-contour.yaml",
			"02-service-envoy.yaml",
			"03-contour.yaml",
			"03-envoy.yaml",
			"04-gatewayclass.yaml",
			"kustomization.yaml",
		},
		NumRelatedTags: 0,
		Expected: []types.Image{
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "02-job-certgen.yaml",
						ManifestDigest: "ba03dc02890e0ca080f12f03fd06a1d4f6b76ff75be0346ee27c9aa73c6d1d31",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           14,
						Column:         16,
					},
					OriginalRef: "ghcr.io/projectcontour/contour:v1.24.1",
				}},
				OriginalName: "ghcr.io/projectcontour/contour",
				OriginalTag:  "v1.24.1",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "03-contour.yaml",
						ManifestDigest: "a9de49647bab938407cb76c29f6b9465690bedb0b99a10736136f982d349d928",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           48,
						Column:         16,
					},
					OriginalRef: "ghcr.io/projectcontour/contour:v1.24.1",
				}},
				OriginalName: "ghcr.io/projectcontour/contour",
				OriginalTag:  "v1.24.1",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "03-envoy.yaml",
						ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           32,
						Column:         16,
					},
					OriginalRef: "ghcr.io/projectcontour/contour:v1.24.1",
				}},
				OriginalName: "ghcr.io/projectcontour/contour",
				OriginalTag:  "v1.24.1",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "03-envoy.yaml",
						ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           53,
						Column:         16,
					},
					OriginalRef: "docker.io/envoyproxy/envoy:v1.25.1",
				}},
				OriginalName: "docker.io/envoyproxy/envoy",
				OriginalTag:  "v1.25.1",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "03-envoy.yaml",
						ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
						NodePath:       []string{"spec", "template", "spec", "initContainers", "image"},
						Line:           110,
						Column:         16,
					},
					OriginalRef: "ghcr.io/projectcontour/contour:v1.24.1",
				}},
				OriginalName: "ghcr.io/projectcontour/contour",
				OriginalTag:  "v1.24.1",
			},
		},
	},
	{
		Description: "flux",
		Directory:   "manifest/testdata/flux",
		Manifests: []string{
			"flux.yaml",
			"kustomization.yaml",
		},
		NumRelatedTags: 2,
		Expected: []types.Image{
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "flux.yaml",
						ManifestDigest: "39ad63101dbb2ead069ca6185bd44f99f52b8513682d6002109c9b0db23f73b5",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           37,
						Column:         16,
					},
					OriginalRef: "ghcr.io/fluxcd/kustomize-controller:v0.30.0",
				}},
				OriginalName: "ghcr.io/fluxcd/kustomize-controller",
				OriginalTag:  "v0.30.0",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "flux.yaml",
						ManifestDigest: "39ad63101dbb2ead069ca6185bd44f99f52b8513682d6002109c9b0db23f73b5",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           43,
						Column:         16,
					},
					OriginalRef: "ghcr.io/fluxcd/source-controller:v0.31.0",
				}},
				OriginalName: "ghcr.io/fluxcd/source-controller",
				OriginalTag:  "v0.31.0",
			},
		},
	},
	{
		Description: "tekton",
		Directory:   "manifest/testdata/tekton",
		Manifests: []string{
			"base/feature-flags.yaml",
			"base/kustomization.yaml",
			"base/tekton-base.yaml",
			"webhooks/kustomization.yaml",
			"webhooks/tekton-mutating-webhooks.yaml",
			"webhooks/tekton-validating-webhooks.yaml",
		},
		NumRelatedTags: 6,
		Expected: []types.Image{
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "base/tekton-base.yaml",
						ManifestDigest: "c2cbc6d7a3c30f99e2e504d5758d8e0ce140a8f444c4d944d85c3f29800bf8c5",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           64,
						Column:         16,
					},
					OriginalRef: "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller:v0.40.2@sha256:dc7bc7d6607466b502d8dc22ba0598461d7477f608ab68aaff1ff4dedaa04f81",
				}},
				OriginalName: "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller",
				OriginalTag:  "v0.40.2",
				Digest:       "sha256:dc7bc7d6607466b502d8dc22ba0598461d7477f608ab68aaff1ff4dedaa04f81",
			},
			{
				Sources: []types.Source{{
					ImageSourceLocation: types.ImageSourceLocation{
						Manifest:       "base/tekton-base.yaml",
						ManifestDigest: "c2cbc6d7a3c30f99e2e504d5758d8e0ce140a8f444c4d944d85c3f29800bf8c5",
						NodePath:       []string{"spec", "template", "spec", "containers", "image"},
						Line:           79,
						Column:         16,
					},
					OriginalRef: "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook:v0.40.2@sha256:6b8aadbdcede63969ecb719e910b55b7681d87110fc0bf92ca4ee943042f620b",
				}},
				OriginalName: "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook",
				OriginalTag:  "v0.40.2",
				Digest:       "sha256:6b8aadbdcede63969ecb719e910b55b7681d87110fc0bf92ca4ee943042f620b",
			},
		},
	},
}

var baseCasesDigests = map[string]string{
	"ghcr.io/projectcontour/contour:v1.24.1":      "sha256:6c87d0bc19fcec5219107d4e153ea019febd8e03c505276383f4ee1df1d592d6",
	"docker.io/envoyproxy/envoy:v1.25.1":          "sha256:d988076dfe0c92d6c7b8dac20e6b278c8de6c2f374f0f2b90976b7886f9a2852",
	"ghcr.io/fluxcd/kustomize-controller:v0.30.0": "sha256:8c6952141b93764740c94aac02b21cc0630902176bdf07ab6b76970e3556a0d2",
	"ghcr.io/fluxcd/source-controller:v0.31.0":    "sha256:1e0b062d5129a462250eb03c5e8bd09b4cc42e88b25e39e35eee81d7ed2d15c0",
}

var baseCasesAliases = map[string]string{
	"ghcr.io/projectcontour/contour:v1.24.1":      "contour",
	"docker.io/envoyproxy/envoy:v1.25.1":          "envoy",
	"ghcr.io/fluxcd/kustomize-controller:v0.30.0": "kustomize-controller",
	"ghcr.io/fluxcd/source-controller:v0.31.0":    "source-controller",
	"gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook:v0.40.2@sha256:6b8aadbdcede63969ecb719e910b55b7681d87110fc0bf92ca4ee943042f620b":    "webhook",
	"gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller:v0.40.2@sha256:dc7bc7d6607466b502d8dc22ba0598461d7477f608ab68aaff1ff4dedaa04f81": "controller",
}

func BaseYAMLCases() TestCases {
	baseCases := make([]TestCase, len(baseYAMLCases))
	copy(baseCases, baseYAMLCases)
	return baseCases
}

func BaseYAMLCasesWithDigests(t *testing.T) TestCases {
	baseCases := make([]TestCase, len(baseYAMLCases))
	for i, c := range baseYAMLCases {
		for e := range c.Expected {
			if digest, ok := baseCasesDigests[c.Expected[e].OriginalRef()]; ok {
				c.Expected[e].Digest = digest
			} else if c.Expected[e].Digest == "" {
				t.Logf("digest not found for %s", c.Expected[e].OriginalRef())
				t.FailNow()
			}
			if alias, ok := baseCasesAliases[c.Expected[e].OriginalRef()]; ok {
				c.Expected[e].Alias = &alias
			} else if c.Expected[e].Alias == nil {
				t.Logf("alias not found for %s", c.Expected[e].OriginalRef())
				t.FailNow()
			}
		}
		baseCases[i] = c
	}
	return baseCases
}
