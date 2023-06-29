package loader_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/docker/labs-brown-tape/manifest/loader"
	"github.com/docker/labs-brown-tape/manifest/testdata"
)

func TestLoader(t *testing.T) {
	cases := testdata.TestCases{}

	cases = append(cases, testdata.BasicJSONCases()...)
	cases = append(cases, testdata.BaseYAMLCases()...)

	cases.Run(t, makeLoaderTest)
}

func makeLoaderTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		loader := NewRecursiveManifestDirectoryLoader(tc.Directory)
		g.Expect(loader.Load()).To(Succeed())

		expectedNumPaths := len(tc.Manifests)
		g.Expect(loader.Paths()).To(HaveLen(expectedNumPaths))

		for i := range tc.Manifests {
			g.Expect(loader.ContainsRelPath(tc.Manifests[i])).To(BeTrue())
		}
	}
}
