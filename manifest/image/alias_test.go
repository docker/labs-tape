package image_test

import (
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/docker/labs-brown-tape/manifest/image"
)

func TestMakeAliasesForNames(t *testing.T) {
	cases := [][]struct {
		image string
		alias string
	}{
		{
			{
				alias: "bar/foo",
				image: "example.com/a1/bar/foo",
			},
			{
				alias: "foo1",
				image: "example.com/a1/bar/foo1",
			},
			{
				alias: "baz/foo",
				image: "example.io/b1/baz/foo",
			},
			{
				alias: "baz/foo",
				image: "example.io/b1/baz/foo",
			},
			{
				alias: "f1/foo",
				image: "example.com/f1/foo",
			},
			{
				alias: "f2/foo",
				image: "example.io/f2/foo",
			},
			{
				alias: "foo",
				image: "foo",
			},
			{
				alias: "example.io/b2/barfoo",
				image: "example.io/b2/barfoo",
			},
			{
				alias: "example.sh/b2/barfoo",
				image: "example.sh/b2/barfoo",
			},
			{
				alias: "b1/barfoo",
				image: "example.io/b1/barfoo",
			},
		},
		{
			{
				alias: "b1/barfoo",
				image: "example.io/b1/barfoo",
			},
			{
				alias: "x/barfoo",
				image: "example.io/b1/x/barfoo",
			},
			{
				alias: "baz/foo",
				image: "example.io/b1/baz/foo",
			},
			{
				alias: "baz",
				image: "example.io/b1/foo/baz",
			},
			{
				alias: "b1/foo",
				image: "example.io/b1/foo",
			},
		},
		{
			{image: "foo", alias: "foo"},
			{image: "bar", alias: "bar"},
		},
		{
			{image: "example.io/foo", alias: "foo"},
			{image: "example.io/bar", alias: "bar"},
		},
		{
			{image: "example.io/foo", alias: "example.io/foo"},
			{image: "example.io/foo", alias: "example.io/foo"},
			{image: "example.org/foo", alias: "example.org/foo"},
		},
		{
			{image: "example.io/bar/foo", alias: "bar/foo"},
			{image: "example.io/foo", alias: "example.io/foo"},
		},
		{
			{image: "example.io/bar/foo", alias: "bar/foo"},
			{image: "example.io/baz/foo", alias: "baz/foo"},
		},
		{
			{image: "example.com/a1/bar/foo", alias: "bar/foo"},
			{image: "example.io/b1/baz/foo", alias: "baz/foo"},
		},
		{
			{image: "example.com/a1/bar/foo", alias: "bar/foo"},
			{image: "example.com/b1/baz/foo", alias: "baz/foo"},
		},
		{
			{image: "example.com/a1/bar/foo", alias: "foo"},
			{image: "example.io/b1/baz/bar", alias: "bar"},
		},
	}

	g := NewWithT(t)

	for c := range cases {
		images := make([]string, len(cases[c]))
		for i := range cases[c] {
			images[i] = cases[c][i].image
		}

		// TODO: shuffle images to ensure aliases are not biased based on order
		aliases := NewAliasCache(images).MakeAliasesForNames()
		for i := range cases[c] {
			g.Expect(aliases[i]).To(Equal(cases[c][i].alias))
		}
	}
}
