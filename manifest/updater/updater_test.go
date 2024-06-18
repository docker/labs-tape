package updater_test

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	. "github.com/onsi/gomega"

	"github.com/errordeveloper/tape/manifest/imagecopier"
	"github.com/errordeveloper/tape/manifest/imageresolver"
	"github.com/errordeveloper/tape/manifest/imagescanner"
	"github.com/errordeveloper/tape/manifest/loader"
	"github.com/errordeveloper/tape/manifest/testdata"
	"github.com/errordeveloper/tape/manifest/types"
	. "github.com/errordeveloper/tape/manifest/updater"
	"github.com/errordeveloper/tape/oci"
	"github.com/errordeveloper/tape/trex"
)

var (
	craneOptions    []crane.Option
	makeDestination func(string) string
)

func TestUpdater(t *testing.T) {
	trex.RunShared()
	craneOptions = trex.Shared.CraneOptions()
	makeDestination = trex.Shared.NewUniqueRepoNamer("bpt-updater-test")

	cases := testdata.BaseYAMLCasesWithDigests(t)
	cases.Run(t, ("../../"), makeUpdaterTest)
}

func makeUpdaterTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		loader := loader.NewRecursiveManifestDirectoryLoader(tc.Directory)
		g.Expect(loader.Load()).To(Succeed())

		scanner := imagescanner.NewDefaultImageScanner()

		expectedNumPaths := len(tc.Manifests)
		g.Expect(loader.Paths()).To(HaveLen(expectedNumPaths))

		for i := range tc.Manifests {
			g.Expect(loader.ContainsRelPath(tc.Manifests[i])).To(BeTrue())
		}

		g.Expect(scanner.Scan(loader.RelPaths())).To(Succeed())

		ctx := context.Background()
		client := oci.NewClient(craneOptions)

		images := scanner.GetImages()

		// TODO: should this use fake resolver to avoid network traffic or perhaps pre-cache images in trex?
		g.Expect(imageresolver.NewRegistryResolver(client).ResolveDigests(ctx, images)).To(Succeed())

		// TODO: fix this, it currently breaks as tc.Expected has a single source
		// g.Expect(images.Dedup()).To(Succeed())
		imagesCopied, err := imagecopier.NewRegistryCopier(client, makeDestination(tc.Description)).CopyImages(ctx, images)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(imagesCopied).To(HaveLen(images.Len()))

		imagecopier.SetNewImageRefs(makeDestination(tc.Description), sha256.New(), tc.Expected)

		g.Expect(NewFileUpdater().Update(images)).To(Succeed())

		images.MakeAliases()

		if tc.Expected != nil {
			g.Expect(images.Items()).To(ConsistOf(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}

		scanner.Reset()

		g.Expect(scanner.Scan(loader.RelPaths())).To(Succeed())

		if tc.Expected != nil {
			g.Expect(scanner.GetImages().Items()).To(HaveLen(len(tc.Expected)))

			images := &types.ImageList{}
			expected := &types.ImageList{}
			matched := &types.ImageList{}

			images.Append(scanner.GetImages().Items()...)

			expected.Append(tc.Expected...)

			imagesGroups := images.GroupByManifest()
			expectedGroups := expected.GroupByManifest()

			g.Expect(imagesGroups).To(HaveLen(len(expectedGroups)))
			for m := range expectedGroups {
				g.Expect(imagesGroups).To(HaveKey(m))
				g.Expect(imagesGroups[m].Len()).To(Equal(expectedGroups[m].Len()))
				for _, image := range imagesGroups[m].Items() {
					for _, expectedImage := range expectedGroups[m].Items() {
						// aliasing is possible, so cannot just match the digest,
						// need to check name and tag as well
						if expectedImage.Digest == image.Digest &&
							expectedImage.NewName == image.OriginalName &&
							expectedImage.NewTag == image.OriginalTag {
							g.Expect(image.ManifestDigest()).ToNot(Equal(expectedImage.ManifestDigest()))
							matched.Append(image)
						}
					}
				}
			}

			// the above loop can yield duplicate entries, it would be avoidable
			// if image.NodePath did represent the full path to the image, but
			// for some reason it drops array indices, so it's not unique
			g.Expect(matched.Len()).To(BeNumerically(">=", images.Len()))
			g.Expect(matched.Items()).To(ContainElements(images.Items()))
		}
	}
}
