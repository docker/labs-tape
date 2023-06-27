package updater_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"

	"github.com/docker/labs-brown-tape/manifest/imagecopier"
	"github.com/docker/labs-brown-tape/manifest/imageresolver"
	"github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/testdata"
	. "github.com/docker/labs-brown-tape/manifest/updater"
)

var destinationUUID = uuid.New().String()

func newDestination(name string) string {
	return fmt.Sprintf("ttl.sh/%s/bpt-updater-test-%s", destinationUUID, name)
}

func TestUpdater(t *testing.T) {
	cases := testdata.BaseYAMLCasesWithDigests(t)
	cases.RelocateFiles(t)
	cases.Run(t, makeUpdaterTest)
}

func makeUpdaterTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		scanner := imagescanner.NewImageScanner()

		g.Expect(scanner.Scan(tc.Manifests)).To(Succeed())

		images := scanner.GetImages()

		// TODO: should this use fake resolver to avoid network traffic?
		g.Expect(imageresolver.NewRegistryResolver().ResolveDigests(images)).To(Succeed())

		g.Expect(imagecopier.NewRegistryCopier(newDestination(tc.Description)).CopyImages(images)).To(Succeed())

		imagecopier.SetNewImageRefs(newDestination(tc.Description), sha256.New(), tc.Expected)

		g.Expect(NewFileUpdater().Update(images)).To(Succeed())

		if tc.Expected != nil {
			g.Expect(images).To(Equal(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}

		scanner.Reset()

		g.Expect(scanner.Scan(tc.Manifests)).To(Succeed())

		if tc.Expected != nil {
			images := scanner.GetImages()
			for i := range tc.Expected {
				tc.Expected[i].OriginalRef = tc.Expected[i].NewName + ":" + tc.Expected[i].NewTag + "@" + tc.Expected[i].Digest
				tc.Expected[i].OriginalName = tc.Expected[i].NewName
				tc.Expected[i].OriginalTag = tc.Expected[i].NewTag
				tc.Expected[i].NewName = ""
				tc.Expected[i].NewTag = ""
				tc.Expected[i].ManifestDigest = images[i].ManifestDigest
			}
			g.Expect(images).To(Equal(tc.Expected))
		}
	}
}
