package imagescanner_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/loader"
	"github.com/docker/labs-brown-tape/manifest/testdata"
	"github.com/docker/labs-brown-tape/manifest/types"
)

func TestImageScanner(t *testing.T) {

	cases := testdata.TestCases{}

	cases = append(cases, testdata.BasicJSONCases()...)
	cases = append(cases, testdata.BaseYAMLCases()...)

	cases.Run(t, makeImageScannerTest)
}

func makeImageScannerTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		loader := loader.NewRecursiveManifestDirectoryLoader(tc.Directory)
		g.Expect(loader.Load()).To(Succeed())

		expectedNumPaths := len(tc.Manifests)
		g.Expect(loader.Paths()).To(HaveLen(expectedNumPaths))

		scanner := NewDefaultImageScanner()

		g.Expect(scanner.Scan(loader.RelPaths())).To(Succeed())

		images := scanner.GetImages()

		for _, image := range images.Items() {
			g.Expect(image.Source).ToNot(BeNil())
			g.Expect(image.Sources).To(BeEmpty())
		}

		if tc.Expected != nil {
			g.Expect(images.Items()).To(ConsistOf(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}

		images.Dedup()
		if tc.Expected != nil {
			g.Expect(images.Len()).To(BeNumerically("<=", len(tc.Expected)))
			expected := types.NewImageList(tc.Directory)
			for _, image := range tc.Expected {
				expected.Append(image)
			}
			expected.Dedup()
			for _, expectedImage := range expected.Items() {
				g.Expect(images.Items()).To(ContainElement(expectedImage))
			}
		}
		for _, image := range images.Items() {
			g.Expect(image.Source).To(BeNil())
			g.Expect(image.Sources).ToNot(BeEmpty())
		}
	}
}
