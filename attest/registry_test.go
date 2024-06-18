package attest_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	. "github.com/onsi/gomega"

	. "github.com/errordeveloper/tape/attest"
	"github.com/errordeveloper/tape/attest/manifest"
	"github.com/errordeveloper/tape/manifest/imageresolver"
	"github.com/errordeveloper/tape/manifest/imagescanner"
	"github.com/errordeveloper/tape/manifest/loader"
	"github.com/errordeveloper/tape/manifest/testdata"
	"github.com/errordeveloper/tape/oci"
	// "github.com/errordeveloper/tape/trex"
)

var (
	craneOptions []crane.Option
	//makeDestination func(string) string
)

const repeat = 3

func TestRegistry(t *testing.T) {
	// trex.RunShared()
	// craneOptions = trex.Shared.CraneOptions()
	// makeDestination = trex.Shared.NewUniqueRepoNamer("bpt-registry-test")

	cases := testdata.BaseYAMLCasesWithDigests(t)
	cases.Run(t, ("../"), makeRegistryTest)
}

func makeRegistryTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		checksums := make([]string, repeat)
		for i := range checksums {

			loader := loader.NewRecursiveManifestDirectoryLoader(tc.Directory)
			g.Expect(loader.Load()).To(Succeed())

			pathChecker, attreg, err := DetectVCS(tc.Directory)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(pathChecker).ToNot(BeNil())
			g.Expect(attreg).ToNot(BeNil())

			scanner := imagescanner.NewDefaultImageScanner()
			scanner.WithProvinanceAttestor(attreg)

			expectedNumPaths := len(tc.Manifests)
			g.Expect(loader.Paths()).To(HaveLen(expectedNumPaths))

			for i := range tc.Manifests {
				g.Expect(loader.ContainsRelPath(tc.Manifests[i])).To(BeTrue())
			}

			g.Expect(scanner.Scan(loader.RelPaths())).To(Succeed())

			collection, err := attreg.MakePathCheckSummarySummaryCollection()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(collection).ToNot(BeNil())
			g.Expect(collection.Providers).To(ConsistOf("git"))
			g.Expect(collection.EntryGroups).To(HaveLen(1))
			g.Expect(collection.EntryGroups[0]).To(HaveLen(expectedNumPaths + 1))

			g.Expect(attreg.AssociateCoreStatements()).To(Succeed())

			ctx := context.Background()
			client := oci.NewClient(craneOptions)

			images := scanner.GetImages()

			g.Expect(attreg.AssociateStatements(manifest.MakeOriginalImageRefStatements(images)...)).To(Succeed())

			// TODO: should this use fake resolver to avoid network traffic or perhaps pre-cache images in trex?
			g.Expect(imageresolver.NewRegistryResolver(client).ResolveDigests(ctx, images)).To(Succeed())

			g.Expect(images.Dedup()).To(Succeed())

			g.Expect(attreg.AssociateStatements(manifest.MakeResovedImageRefStatements(images)...)).To(Succeed())

			hash := sha256.New()
			buf := bytes.NewBuffer(nil)

			g.Expect(attreg.EncodeAllAttestations(io.MultiWriter(buf, hash))).To(Succeed())
			checksums[i] = hex.EncodeToString(hash.Sum(nil))
		}
		for i := range checksums {
			g.Expect(checksums[i]).To(Equal(checksums[(i+1)%repeat]))
		}
	}
}
