package packager_test

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	. "github.com/onsi/gomega"

	"github.com/errordeveloper/tape/attest"
	"github.com/errordeveloper/tape/attest/manifest"
	"github.com/errordeveloper/tape/manifest/imagecopier"
	"github.com/errordeveloper/tape/manifest/imageresolver"
	"github.com/errordeveloper/tape/manifest/imagescanner"
	"github.com/errordeveloper/tape/manifest/loader"
	. "github.com/errordeveloper/tape/manifest/packager"
	"github.com/errordeveloper/tape/manifest/testdata"
	"github.com/errordeveloper/tape/manifest/updater"
	"github.com/errordeveloper/tape/oci"
	"github.com/errordeveloper/tape/trex"
)

var (
	craneOptions    []crane.Option
	makeDestination func(string) string
)

func TestPackager(t *testing.T) {
	trex.RunShared()
	craneOptions = trex.Shared.CraneOptions()
	makeDestination = trex.Shared.NewUniqueRepoNamer("bpt-updater-test")

	cases := testdata.BaseYAMLCasesWithDigests(t)
	cases.Run(t, ("../../"), makePackagerTest)
}

func makePackagerTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		loader := loader.NewRecursiveManifestDirectoryLoader(tc.Directory)
		g.Expect(loader.Load()).To(Succeed())

		pathChecker, attreg, err := attest.DetectVCS(tc.Directory)
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
		g.Expect(attreg.AssociateCoreStatements()).To(Succeed())

		ctx := context.Background()
		client := oci.NewClient(craneOptions)

		images := scanner.GetImages()

		g.Expect(attreg.AssociateStatements(manifest.MakeOriginalImageRefStatements(images)...)).To(Succeed())

		// TODO: should this use fake resolver to avoid network traffic or perhaps pre-cache images in trex?
		g.Expect(imageresolver.NewRegistryResolver(client).ResolveDigests(ctx, images)).To(Succeed())

		g.Expect(attreg.AssociateStatements(manifest.MakeResovedImageRefStatements(images)...)).To(Succeed())

		imagesCopied, err := imagecopier.NewRegistryCopier(client, makeDestination(tc.Description)).CopyImages(ctx, images)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(imagesCopied).To(HaveLen(images.Len()))

		imagecopier.SetNewImageRefs(makeDestination(tc.Description), sha256.New(), tc.Expected)

		g.Expect(updater.NewFileUpdater().Update(images)).To(Succeed())

		images.MakeAliases()

		if tc.Expected != nil {
			g.Expect(images.Items()).To(ConsistOf(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}

		// scanner.Reset()

		// g.Expect(scanner.Scan(loader.RelPaths())).To(Succeed())

		destinationRef := makeDestination(tc.Description)
		_, sorceEpochTimestamp := loader.MostRecentlyModified()

		// TODO: consider adding digest to tests fixtures to test exact value for a moree definite assertion of reproduciability
		artefactRef1, err := NewDefaultPackager(client, destinationRef, &sorceEpochTimestamp, attreg.GetStatements()...).Push(ctx, images.Dir())
		g.Expect(err).To(Succeed())

		artefactRef2, err := NewDefaultPackager(client, destinationRef, &sorceEpochTimestamp, attreg.GetStatements()...).Push(ctx, images.Dir())
		g.Expect(err).To(Succeed())

		g.Expect(artefactRef1).To(Equal(artefactRef2))

		// TODO: pull the contents from the registry and compare them to what is expected;
		// e.g. also as the means to test inspection logic (TBI)
	}
}
