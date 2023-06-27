package imageresolver_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/docker/labs-brown-tape/manifest/imageresolver"
	"github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/testdata"
)

func TestImageResover(t *testing.T) {
	cases := testdata.BaseYAMLCasesWithDigests(t)

	cases.Run(t, makeImageResolverTest)
}

func makeImageResolverTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		scanner := imagescanner.NewImageScanner()

		g.Expect(scanner.Scan(tc.Manifests)).To(Succeed())

		images := scanner.GetImages()

		// TODO: should this use fake resolver to avoid network traffic?
		g.Expect(NewRegistryResolver().ResolveDigests(images)).To(Succeed())

		if tc.Expected != nil {
			g.Expect(images).To(Equal(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}
	}
}
