package imagecopier_test

import (
	"context"
	"crypto/sha256"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/errordeveloper/tape/manifest/imagecopier"
	"github.com/errordeveloper/tape/manifest/imageresolver"
	"github.com/errordeveloper/tape/manifest/imagescanner"
	"github.com/errordeveloper/tape/manifest/loader"
	"github.com/errordeveloper/tape/manifest/testdata"
	"github.com/errordeveloper/tape/oci"
	"github.com/errordeveloper/tape/trex"
	"github.com/google/go-containerregistry/pkg/crane"
)

var (
	craneOptions    []crane.Option
	makeDestination func(string) string
)

func TestImageCopier(t *testing.T) {
	trex.RunShared()
	craneOptions = trex.Shared.CraneOptions()
	makeDestination = trex.Shared.NewUniqueRepoNamer("bpt-updater-test")

	testdata.BaseYAMLCasesWithDigests(t).Run(t, ("../../"), makeImageCopierTest)
}

func makeImageCopierTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		loader := loader.NewRecursiveManifestDirectoryLoader(tc.Directory)
		g.Expect(loader.Load()).To(Succeed())

		expectedNumPaths := len(tc.Manifests)
		g.Expect(loader.Paths()).To(HaveLen(expectedNumPaths))

		scanner := imagescanner.NewDefaultImageScanner()

		g.Expect(scanner.Scan(loader.RelPaths())).To(Succeed())

		ctx := context.Background()
		client := oci.NewClient(craneOptions)

		images := scanner.GetImages()

		// TODO: should this use fake resolver to avoid network traffic or perhaps pre-cache images in trex?
		g.Expect(imageresolver.NewRegistryResolver(client).ResolveDigests(ctx, images)).To(Succeed())

		copied, err := NewRegistryCopier(client, makeDestination(tc.Description)).CopyImages(ctx, images)
		g.Expect(err).ToNot(HaveOccurred())
		expectToCopyRefs := []string{}
		for _, image := range images.Items() {
			expectToCopyRefs = append(expectToCopyRefs, image.Ref(false))
		}
		g.Expect(copied).To(ConsistOf(expectToCopyRefs))

		SetNewImageRefs(makeDestination(tc.Description), sha256.New(), tc.Expected)

		images.MakeAliases()

		if tc.Expected != nil {
			g.Expect(images.Items()).To(ConsistOf(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}
	}
}
