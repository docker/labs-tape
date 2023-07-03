package imagecopier_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"

	ociclient "github.com/fluxcd/pkg/oci/client"

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

		ctx := context.Background()
		client := ociclient.NewClient(nil)

		images := scanner.GetImages()

		// TODO: should this use fake resolver to avoid network traffic?
		g.Expect(imageresolver.NewRegistryResolver(client).ResolveDigests(ctx, images)).To(Succeed())

		g.Expect(NewRegistryCopier(client, newDestination(tc.Description)).CopyImages(ctx, images)).To(Succeed())

		SetNewImageRefs(newDestination(tc.Description), sha256.New(), tc.Expected)

		if tc.Expected != nil {
			g.Expect(images.Items()).To(ConsistOf(tc.Expected))
		} else {
			t.Logf("%#v\n", images)
		}
	}
}
