package imagecopier_test

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"

	. "github.com/docker/labs-brown-tape/manifest/imagecopier"
	"github.com/docker/labs-brown-tape/manifest/imageresolver"
	"github.com/docker/labs-brown-tape/manifest/imagescanner"
	"github.com/docker/labs-brown-tape/manifest/loader"
	"github.com/docker/labs-brown-tape/manifest/testdata"
)

var destinationUUID = uuid.New().String()

func newDestination(name string) string {
	return fmt.Sprintf("ttl.sh/%s/bpt-imagecopier-test-%s", destinationUUID, name)
}

func TestImageResover(t *testing.T) {
	testdata.BaseYAMLCasesWithDigests(t).Run(t, makeImageCopierTest)
}

func makeImageCopierTest(tc testdata.TestCase) func(t *testing.T) {
	return func(t *testing.T) {
		g := NewWithT(t)
		t.Parallel()

		loader := loader.NewRecursiveManifestDirectoryLoader(tc.Directory)
		g.Expect(loader.Load()).To(Succeed())

		expectedNumPaths := len(tc.Manifests)
		g.Expect(loader.Paths()).To(HaveLen(expectedNumPaths))

		scanner := imagescanner.NewDefaultImageScanner()

		g.Expect(scanner.Scan(loader.RelPaths())).To(Succeed())

		images := scanner.GetImages()

		// TODO: should this use fake resolver to avoid network traffic?
		g.Expect(imageresolver.NewRegistryResolver().ResolveDigests(images)).To(Succeed())

		g.Expect(NewRegistryCopier(newDestination(tc.Description)).CopyImages(images)).To(Succeed())

		SetNewImageRefs(newDestination(tc.Description), sha256.New(), tc.Expected)

		if tc.Expected != nil {
			g.Expect(images.Items()).To(ConsistOf(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}
	}
}
