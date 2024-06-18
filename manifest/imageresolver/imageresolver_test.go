package imageresolver_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/errordeveloper/tape/manifest/imageresolver"
	"github.com/errordeveloper/tape/manifest/imagescanner"
	"github.com/errordeveloper/tape/manifest/loader"
	"github.com/errordeveloper/tape/manifest/testdata"
	"github.com/errordeveloper/tape/manifest/types"
)

func TestImageResover(t *testing.T) {
	cases := testdata.BaseYAMLCasesWithDigests(t)

	cases.Run(t, ("../../"), makeImageResolverTest)
}

func makeImageResolverTest(tc testdata.TestCase) func(t *testing.T) {
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

		images := scanner.GetImages()
		// TODO: should this use fake resolver to avoid network traffic or perhaps pre-cache images in trex?
		resolver := NewRegistryResolver(nil)
		g.Expect(resolver.ResolveDigests(ctx, images)).To(Succeed())

		images.MakeAliases()

		if tc.Expected != nil {
			g.Expect(images.Items()).To(ConsistOf(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}

		g.Expect(images.Dedup()).To(Succeed())
		related, err := resolver.FindRelatedTags(ctx, images)
		g.Expect(err).ToNot(HaveOccurred())

		g.Expect(related.Items()).To(HaveLen(tc.NumRelatedTags))
		if tc.NumRelatedTags > 0 {
			for _, image := range images.Items() {
				// TODO: this is very naive, need more concerte assertions
				relatedTo := related.RelatedTo(image.Ref(true))
				g.Expect(relatedTo).ToNot(BeEmpty())
			}
		}
		if tc.Expected != nil {
			g.Expect(images.Len()).To(BeNumerically("<=", len(tc.Expected)))
			expected := types.NewImageList(tc.Directory)
			expected.Append(tc.Expected...)
			g.Expect(expected.Dedup()).To(Succeed())
			for _, expectedImage := range expected.Items() {
				g.Expect(images.Items()).To(ContainElement(expectedImage))
			}
		}
		for _, image := range images.Items() {
			g.Expect(image.Sources).ToNot(BeEmpty())
		}
	}
}
