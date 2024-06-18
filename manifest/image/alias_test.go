package image_test

import (
	"math/rand"
	"testing"

	. "github.com/onsi/gomega"

	. "github.com/errordeveloper/tape/manifest/image"
)

func TestMakeAliasesForNames(t *testing.T) {
	cases := [][]struct {
		image string
		alias string
	}{
		{
			{image: "example.com/a1/bar/foo", alias: "bar/foo"},
			{image: "example.com/a1/bar/foo1", alias: "foo1"},
			{image: "example.io/b1/baz/foo", alias: "baz/foo"},
			{image: "example.io/b1/baz/foo", alias: "baz/foo"},
			{image: "example.com/f1/foo", alias: "f1/foo"},
			{image: "example.io/f2/foo", alias: "f2/foo"},
			{image: "foo", alias: "foo"},
			{image: "example.io/b2/barfoo", alias: "example.io/b2/barfoo"},
			{image: "example.sh/b2/barfoo", alias: "example.sh/b2/barfoo"},
			{image: "example.io/b1/barfoo", alias: "b1/barfoo"},
		},
		{
			{image: "example.io/b1/barfoo", alias: "b1/barfoo"},
			{image: "example.io/b1/x/barfoo", alias: "x/barfoo"},
			{image: "example.io/b1/baz/foo", alias: "baz/foo"},
			{image: "example.io/b1/foo/baz", alias: "baz"},
			{image: "example.io/b1/foo", alias: "b1/foo"},
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

	// shuffle cases to ensure that the alising is not dependent on the order
	rand.Shuffle(len(cases), func(i, j int) {
		cases[i], cases[j] = cases[j], cases[i]
	})

	for c := range cases {
		images := make([]string, len(cases[c]))
		for i := range cases[c] {
			images[i] = cases[c][i].image
		}

		aliases := NewAliasCache(images).MakeAliasesForNames()
		for i := range cases[c] {
			g.Expect(aliases[i]).To(Equal(cases[c][i].alias))
		}
	}
}

func TestSearchAliases(t *testing.T) {
	cases := [][]struct {
		image, alias string
	}{
		{
			{image: "example.com/a1/bar/foo", alias: "bar/foo"},            // 0
			{image: "example.com/a1/bar/foo1", alias: "foo1"},              // 1
			{image: "example.io/b1/baz/foo", alias: "baz/foo"},             // 2
			{image: "example.io/b1/baz/foo", alias: "baz/foo"},             // 3
			{image: "example.com/f1/foo", alias: "f1/foo"},                 // 4
			{image: "example.io/f2/foo", alias: "f2/foo"},                  // 5
			{image: "foo", alias: "foo"},                                   // 6
			{image: "example.io/b2/barfoo", alias: "example.io/b2/barfoo"}, // 7
			{image: "example.sh/b2/barfoo", alias: "example.sh/b2/barfoo"}, // 8
			{image: "example.io/b1/barfoo", alias: "b1/barfoo"},            // 9
		},
		{
			{image: "example.io/b1/barfoo", alias: "b1/barfoo"},  // 0
			{image: "example.io/b1/x/barfoo", alias: "x/barfoo"}, // 1
			{image: "example.io/b1/baz/foo", alias: "baz/foo"},   // 2
			{image: "example.io/b1/foo/baz", alias: "baz"},       // 3
			{image: "example.io/b1/foo", alias: "b1/foo"},        // 4
		},
	}

	searches := [][]struct {
		term    string
		results []int
	}{
		{
			{
				term:    "*/foo",
				results: nil,
			},
			{
				term:    "**/foo",
				results: nil,
			},
			{
				term:    ".*",
				results: nil,
			},
			{
				term:    "bar/foo",
				results: []int{0},
			},
			{
				term:    "foo",
				results: []int{6},
			},
			{
				term:    "/foo",
				results: nil,
			},
			{
				term:    "//foo",
				results: nil,
			},
			{
				term:    "///foo",
				results: nil,
			},
			{
				term:    "//foo/",
				results: nil,
			},
			{
				term:    "/foo/",
				results: nil,
			},
			{
				term:    "foo//",
				results: nil,
			},
			{
				term:    "a1/bar/foo",
				results: []int{0},
			},
			{
				term:    "example.com/a1/bar/foo",
				results: []int{0},
			},
			{
				term:    "example.com/foobar",
				results: nil,
			},
			{
				term:    "example.com",
				results: nil,
			},
			{
				term:    "foobar",
				results: nil,
			},

			{
				term:    "xample.com/a1/bar/foo",
				results: nil,
			},
			{
				term:    "example.com/a1/bar/fo",
				results: nil,
			},

			{
				term:    "example.com/a1/bar/foo1",
				results: []int{1},
			},
			{
				term:    "/a1/bar/foo1",
				results: nil,
			},
			{
				term:    "a1/bar/foo1",
				results: []int{1},
			},
			{
				term:    "bar/foo1",
				results: []int{1},
			},
			{
				term:    "foo1",
				results: []int{1},
			},
			{
				term:    "example.io/b1/baz/foo",
				results: []int{2, 3},
			},
			{
				term:    "b1/baz/foo",
				results: []int{2, 3},
			},
			{
				term:    "example.com/f1/foo",
				results: []int{4},
			},
			{
				term:    "example.io/f2/foo",
				results: []int{5},
			},
			{
				term:    "example.io/f2/f",
				results: nil,
			},
			{
				term:    "example.io/f2",
				results: nil,
			},
			{
				term:    "f2/foo",
				results: []int{5},
			},

			{
				term:    "example.io/b2/barfoo",
				results: []int{7},
			},
			{
				term:    "example.sh/b2/barfoo",
				results: []int{8},
			},
			{
				term:    "b2/barfoo",
				results: []int{7, 8},
			},
			{
				term:    "barfoo",
				results: []int{7, 8, 9},
			},
			{
				term:    "example.io/b1/barfoo",
				results: []int{9},
			},
		},
		{
			{
				term:    "example.io/b1/barfoo",
				results: []int{0},
			},
			{
				term:    "example.io/b1/x/barfoo",
				results: []int{1},
			},
			{
				term:    "x/barfoo",
				results: []int{1},
			},
			{
				term:    "barfoo",
				results: []int{0, 1},
			},
			{
				term:    "example.io/b1/baz/foo",
				results: []int{2},
			},
			{
				term:    "foo",
				results: []int{2, 4},
			},
			{
				term:    "example.io/b1/foo/baz",
				results: []int{3},
			},
			{
				term:    "foo/baz",
				results: []int{3},
			},
			{
				term:    "example.io/b1/foo",
				results: []int{4},
			},
		},
	}

	g := NewWithT(t)

	var cache AliasCache

	for c := range cases {
		images := make([]string, len(cases[c]))
		for i := range cases[c] {
			images[i] = cases[c][i].image
		}

		cache = NewAliasCache(images)
		aliases := cache.MakeAliasesForNames()
		for i := range cases[c] {
			g.Expect(aliases[i]).To(Equal(cases[c][i].alias))
		}

		for s := range searches[c] {
			search := searches[c][s]
			match, matches, ok := cache.Match(search.term)

			// t.Log(search, match, matches, ok)

			if search.results == nil {
				g.Expect(ok).To(BeFalse())
				continue
			}

			if len(search.results) == 1 {
				g.Expect(ok).To(BeTrue())
				g.Expect(match).To(Equal(cases[c][search.results[0]].image))
				continue
			}

			g.Expect(ok).To(BeFalse())
			g.Expect(matches).To(HaveLen(len(search.results)))
			expectedMatches := make([]string, len(search.results))
			for i, v := range search.results {
				expectedMatches[i] = cases[c][v].image
			}
			g.Expect(matches).To(ConsistOf(expectedMatches))
		}
	}
}
