package loader_test

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/errordeveloper/tape/manifest/loader"
	"github.com/errordeveloper/tape/manifest/testdata"
)

func TestLoader(t *testing.T) {
	cases := testdata.TestCases{}

	cases = append(cases, testdata.BasicJSONCases()...)
	cases = append(cases, testdata.BaseYAMLCases()...)

	cases.Run(t, ("../../"), makeLoaderTest)
}

func makeLoaderTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		loader := NewRecursiveManifestDirectoryLoader(tc.Directory)
		g.Expect(loader.Load()).To(Succeed())

		expectedNumPaths := len(tc.Manifests)

		g.Expect(loader.Paths()).To(HaveLen(expectedNumPaths))
		_, relPaths1 := loader.RelPaths()
		g.Expect(relPaths1).To(HaveLen(expectedNumPaths))

		for i := range tc.Manifests {
			g.Expect(loader.ContainsRelPath(tc.Manifests[i])).To(BeTrue())
		}

		mrmFile1, mrmTimestamp1 := loader.MostRecentlyModified()

		g.Expect(loader.Cleanup()).To(Succeed())
		g.Expect(loader.Load()).To(Succeed())

		_, relPaths2 := loader.RelPaths()
		g.Expect(relPaths2).To(ConsistOf(relPaths1))

		mrmFile2, mrmTimestamp2 := loader.MostRecentlyModified()

		g.Expect(filepath.Base(mrmFile1)).To(Equal(filepath.Base(mrmFile2)))
		g.Expect(mrmTimestamp1).To(Equal(mrmTimestamp2))
	}
}
