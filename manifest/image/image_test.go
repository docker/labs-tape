package image

import (
	"testing"

	. "github.com/onsi/gomega"
	kimage "sigs.k8s.io/kustomize/api/image"
)

func TestDedup(t *testing.T) {
	g := NewWithT(t)

	cases := []struct {
		image string
		alias string
	}{
		{
			alias: "bar/foo",
			image: "example.com/a1/bar/foo:f@sha256:foo1",
		},
		{
			alias: "baz/foo",
			image: "example.io/b1/baz/foo:v1@sha256:foo1",
		},
		{
			alias: "baz/foo",
			image: "example.io/b1/baz/foo@sha256:foo1",
		},

		{
			alias: "example.com/f1/foo",
			image: "example.com/f1/foo@sha256:foo2",
		},
		{
			alias: "example.io/f1/foo",
			image: "example.io/f1/foo@sha256:foo3",
		},
		{
			alias: "foo",
			image: "foo@sha256:foo4",
		},

		// {
		// 	alias:  "example.io/b2/barfoo@sha256:barfoo1",
		// 	image:  "example.io/b2/barfoo",
		// 	digest: "barfoo1",
		// },
		// {
		// 	alias:  "example.io/b2/barfoo@sha256:barfoo2",
		// 	image:  "example.io/b2/barfoo",
		// 	digest: "barfoo2",
		// },
		// {
		// 	alias:  "example.io/b2/barfoo@sha256:barfoo1",
		// 	image:  "example.io/b2/barfoo",
		// 	digest: "barfoo1",
		// },
		{
			alias: "b1/barfoo",
			image: "example.io/b1/barfoo@sha256:barfoo3",
		},
		{
			alias: "x/barfoo",
			image: "example.io/b1/x/barfoo:latest@sha256:barfoo4",
		},
	}

	list := NewImageList("")

	for i := range cases {
		tc := cases[i]

		name, tag, digest := kimage.Split(tc.image)
		list.Append(Image{
			Sources: []Source{{
				OriginalRef: tc.image,
				ImageSourceLocation: ImageSourceLocation{
					Line:     i,
					Column:   0,
					Manifest: "test",
				},
			}},
			OriginalName: name,
			OriginalTag:  tag,
			Digest:       digest,
		})
	}

	g.Expect(list.Dedup()).To(Succeed())
	images := list.Items()

	for i := range images {
		image := images[i]
		g.Expect(image.Alias).ToNot(BeNil())
		g.Expect(*image.Alias).ToNot(BeEmpty())
		g.Expect(*image.Alias).To(Equal(cases[image.primarySource().Line].alias))
	}
}
