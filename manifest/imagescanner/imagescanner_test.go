package imagescanner_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/types"
)

type imageScannerTestCase struct {
	description string
	manifests   []string
	expected    []types.Image
}

func TestImageScanner(t *testing.T) {

	cases := []imageScannerTestCase{
		{
			description: "basic",
			manifests: []string{
				"../testdata/basic/list.json",
				"../testdata/basic/deployment.json",
			},
			expected: []types.Image{
				{
					Manifest:       "../testdata/basic/list.json",
					ManifestDigest: "577caeee80cfa690caf25bcdd4b1919b99d2860eb351c48e81b46b9e4b52aea5",
					NodePath:       []string{"spec", "containers", "image"},
					OriginalRef:    "nginx",
					OriginalName:   "nginx",
				},
				{
					Manifest:       "../testdata/basic/list.json",
					ManifestDigest: "577caeee80cfa690caf25bcdd4b1919b99d2860eb351c48e81b46b9e4b52aea5",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "redis",
					OriginalName:   "redis",
				},
				{
					Manifest:       "../testdata/basic/list.json",
					ManifestDigest: "577caeee80cfa690caf25bcdd4b1919b99d2860eb351c48e81b46b9e4b52aea5",
					NodePath:       []string{"items", "spec", "containers", "image"},
					OriginalRef:    "redis",
					OriginalName:   "redis",
				},
				{
					Manifest:       "../testdata/basic/deployment.json",
					ManifestDigest: "8d85ce5a5de4085bb841cee0402022fcd03f86606a67d572e62012ce4420668c",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "nginx:1.16.1",
					OriginalName:   "nginx",
					OriginalTag:    "1.16.1",
				},
			},
		},
		{
			description: "contour",
			manifests: []string{
				"../testdata/contour/00-common.yaml",
				"../testdata/contour/00-crds.yaml",
				"../testdata/contour/01-contour-config.yaml",
				"../testdata/contour/01-crds.yaml",
				"../testdata/contour/02-job-certgen.yaml",
				"../testdata/contour/02-rbac.yaml",
				"../testdata/contour/02-role-contour.yaml",
				"../testdata/contour/02-service-contour.yaml",
				"../testdata/contour/02-service-envoy.yaml",
				"../testdata/contour/03-contour.yaml",
				"../testdata/contour/03-envoy.yaml",
				"../testdata/contour/04-gatewayclass.yaml",
				"../testdata/contour/kustomization.yaml",
			},
			expected: []types.Image{
				{
					Manifest:       "../testdata/contour/02-job-certgen.yaml",
					ManifestDigest: "ba03dc02890e0ca080f12f03fd06a1d4f6b76ff75be0346ee27c9aa73c6d1d31",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "ghcr.io/projectcontour/contour:v1.24.1",
					OriginalName:   "ghcr.io/projectcontour/contour",
					OriginalTag:    "v1.24.1",
				},
				{
					Manifest:       "../testdata/contour/03-contour.yaml",
					ManifestDigest: "a9de49647bab938407cb76c29f6b9465690bedb0b99a10736136f982d349d928",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "ghcr.io/projectcontour/contour:v1.24.1",
					OriginalName:   "ghcr.io/projectcontour/contour",
					OriginalTag:    "v1.24.1",
				},
				{
					Manifest:       "../testdata/contour/03-envoy.yaml",
					ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "ghcr.io/projectcontour/contour:v1.24.1",
					OriginalName:   "ghcr.io/projectcontour/contour",
					OriginalTag:    "v1.24.1",
				},
				{
					Manifest:       "../testdata/contour/03-envoy.yaml",
					ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "docker.io/envoyproxy/envoy:v1.25.1",
					OriginalName:   "docker.io/envoyproxy/envoy",
					OriginalTag:    "v1.25.1",
				},
				{
					Manifest:       "../testdata/contour/03-envoy.yaml",
					ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
					NodePath:       []string{"spec", "template", "spec", "initContainers", "image"},
					OriginalRef:    "ghcr.io/projectcontour/contour:v1.24.1",
					OriginalName:   "ghcr.io/projectcontour/contour",
					OriginalTag:    "v1.24.1",
				},
			},
		},
		{
			description: "flux",
			manifests: []string{
				"../testdata/flux/flux.yaml",
				"../testdata/flux/kustomization.yaml",
			},
			expected: []types.Image{
				{
					Manifest:       "../testdata/flux/flux.yaml",
					ManifestDigest: "39ad63101dbb2ead069ca6185bd44f99f52b8513682d6002109c9b0db23f73b5",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "ghcr.io/fluxcd/kustomize-controller:v0.30.0",
					OriginalName:   "ghcr.io/fluxcd/kustomize-controller",
					OriginalTag:    "v0.30.0",
				},
				{
					Manifest:       "../testdata/flux/flux.yaml",
					ManifestDigest: "39ad63101dbb2ead069ca6185bd44f99f52b8513682d6002109c9b0db23f73b5",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "ghcr.io/fluxcd/source-controller:v0.31.0",
					OriginalName:   "ghcr.io/fluxcd/source-controller",
					OriginalTag:    "v0.31.0",
				},
			},
		},
		{
			description: "tekton",
			manifests: []string{
				"../testdata/tekton/base/feature-flags.yaml",
				"../testdata/tekton/base/kustomization.yaml",
				"../testdata/tekton/base/tekton-base.yaml",
				"../testdata/tekton/webhooks/kustomization.yaml",
				"../testdata/tekton/webhooks/tekton-mutating-webhooks.yaml",
				"../testdata/tekton/webhooks/tekton-validating-webhooks.yaml",
			},
			expected: []types.Image{
				{
					Manifest:       "../testdata/tekton/base/tekton-base.yaml",
					ManifestDigest: "c2cbc6d7a3c30f99e2e504d5758d8e0ce140a8f444c4d944d85c3f29800bf8c5",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller:v0.40.2@sha256:dc7bc7d6607466b502d8dc22ba0598461d7477f608ab68aaff1ff4dedaa04f81",
					OriginalName:   "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/controller",
					OriginalTag:    "v0.40.2",
					Digest:         "sha256:dc7bc7d6607466b502d8dc22ba0598461d7477f608ab68aaff1ff4dedaa04f81",
				},
				{
					Manifest:       "../testdata/tekton/base/tekton-base.yaml",
					ManifestDigest: "c2cbc6d7a3c30f99e2e504d5758d8e0ce140a8f444c4d944d85c3f29800bf8c5",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook:v0.40.2@sha256:6b8aadbdcede63969ecb719e910b55b7681d87110fc0bf92ca4ee943042f620b",
					OriginalName:   "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/webhook",
					OriginalTag:    "v0.40.2",
					Digest:         "sha256:6b8aadbdcede63969ecb719e910b55b7681d87110fc0bf92ca4ee943042f620b",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, makeImageScannerTest(tc))
	}
}

func makeImageScannerTest(i imageScannerTestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		scanner := NewImageScanner()

		g.Expect(scanner.Scan(i.manifests)).To(Succeed())

		images := scanner.GetImages()

		if i.expected != nil {
			g.Expect(images).To(Equal(i.expected))
		} else {
			t.Logf("%#v\n", images)
		}
	}
}
