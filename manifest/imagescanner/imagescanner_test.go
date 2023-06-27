package imagescanner_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/testdata"
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

		scanner := NewImageScanner()

		g.Expect(scanner.Scan(tc.Manifests)).To(Succeed())

		images := scanner.GetImages()

		if tc.Expected != nil {
			g.Expect(images).To(Equal(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}
	}
}
