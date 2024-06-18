package imagescanner_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/errordeveloper/tape/manifest/imagescanner"
	"github.com/errordeveloper/tape/manifest/loader"
	"github.com/errordeveloper/tape/manifest/testdata"
)

func TestImageScanner(t *testing.T) {

	cases := testdata.TestCases{}

	cases = append(cases, testdata.BasicJSONCases()...)
	cases = append(cases, testdata.BaseYAMLCases()...)

	cases.Run(t, ("../../"), makeImageScannerTest)
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
			g.Expect(image.Sources).To(HaveLen(1))
		}

		if tc.Expected != nil {
			g.Expect(images.Items()).To(ConsistOf(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}
	}
}
