package imagecopier_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"

	. "github.com/docker/labs-brown-tape/manifest/imagecopier"
	"github.com/docker/labs-brown-tape/manifest/imageresolver"
	"github.com/docker/labs-brown-tape/manifest/imagescanner"

	"github.com/docker/labs-brown-tape/manifest/types"
)

func newDestination(name string) string {
	return fmt.Sprintf("ttl.sh/%s/bpt-imagecopier-test-%s", uuid.New().String(), name)
}

type imageCopierTestCase struct {
	description string
	manifests   []string
	expected    []types.Image
}

func TestImageResover(t *testing.T) {

	cases := []imageCopierTestCase{
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
					Digest:         "sha256:6c87d0bc19fcec5219107d4e153ea019febd8e03c505276383f4ee1df1d592d6",
				},
				{
					Manifest:       "../testdata/contour/03-contour.yaml",
					ManifestDigest: "a9de49647bab938407cb76c29f6b9465690bedb0b99a10736136f982d349d928",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "ghcr.io/projectcontour/contour:v1.24.1",
					OriginalName:   "ghcr.io/projectcontour/contour",
					OriginalTag:    "v1.24.1",
					Digest:         "sha256:6c87d0bc19fcec5219107d4e153ea019febd8e03c505276383f4ee1df1d592d6",
				},
				{
					Manifest:       "../testdata/contour/03-envoy.yaml",
					ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "ghcr.io/projectcontour/contour:v1.24.1",
					OriginalName:   "ghcr.io/projectcontour/contour",
					OriginalTag:    "v1.24.1",
					Digest:         "sha256:6c87d0bc19fcec5219107d4e153ea019febd8e03c505276383f4ee1df1d592d6",
				},
				{
					Manifest:       "../testdata/contour/03-envoy.yaml",
					ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "docker.io/envoyproxy/envoy:v1.25.1",
					OriginalName:   "docker.io/envoyproxy/envoy",
					OriginalTag:    "v1.25.1",
					Digest:         "sha256:d988076dfe0c92d6c7b8dac20e6b278c8de6c2f374f0f2b90976b7886f9a2852",
				},
				{
					Manifest:       "../testdata/contour/03-envoy.yaml",
					ManifestDigest: "e83cd3f98ddbbd91374511c8ce1e437d938ffc8ea8d50bc6d4ccdbf224e53ed4",
					NodePath:       []string{"spec", "template", "spec", "initContainers", "image"},
					OriginalRef:    "ghcr.io/projectcontour/contour:v1.24.1",
					OriginalName:   "ghcr.io/projectcontour/contour",
					OriginalTag:    "v1.24.1",
					Digest:         "sha256:6c87d0bc19fcec5219107d4e153ea019febd8e03c505276383f4ee1df1d592d6",
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
					Digest:         "sha256:8c6952141b93764740c94aac02b21cc0630902176bdf07ab6b76970e3556a0d2",
				},
				{
					Manifest:       "../testdata/flux/flux.yaml",
					ManifestDigest: "39ad63101dbb2ead069ca6185bd44f99f52b8513682d6002109c9b0db23f73b5",
					NodePath:       []string{"spec", "template", "spec", "containers", "image"},
					OriginalRef:    "ghcr.io/fluxcd/source-controller:v0.31.0",
					OriginalName:   "ghcr.io/fluxcd/source-controller",
					OriginalTag:    "v0.31.0",
					Digest:         "sha256:1e0b062d5129a462250eb03c5e8bd09b4cc42e88b25e39e35eee81d7ed2d15c0",
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
		t.Run(tc.description, makeImageCopierTest(tc))
	}
}

func makeImageCopierTest(i imageCopierTestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		scanner := imagescanner.NewImageScanner()

		g.Expect(scanner.Scan(i.manifests)).To(Succeed())

		images := scanner.GetImages()

		// TODO: should this use fake resolver to avoid network traffic?
		g.Expect(imageresolver.NewRegistryResolver().ResolveDigests(images)).To(Succeed())

		if i.expected != nil {
			g.Expect(images).To(Equal(i.expected))
		} else {
			t.Logf("%#v\n", images)
		}

		g.Expect(NewRegistryCopier(newDestination(i.description)).CopyImages(images)).To(Succeed())
	}
}
