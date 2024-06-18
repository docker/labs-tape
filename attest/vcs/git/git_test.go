package git_test

import (
	"io"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	. "github.com/errordeveloper/tape/attest/vcs/git"
)

type gitTestCases struct {
	desc, path               string
	err, checked, unmodified types.GomegaMatcher
	setup, cleanup           func(*testing.T, *gitTestCases)
}

var dontCare = Or(BeTrue(), BeFalse())

func TestGitProvider(t *testing.T) {
	cases := []gitTestCases{
		{
			desc:       "non-existent-file",
			path:       "testdata/non-existent",
			err:        Not(HaveOccurred()),
			checked:    BeFalse(),
			unmodified: dontCare,
		},
		{
			desc:       "non-existent-file-another-dir",
			path:       "../../non-existent",
			err:        Not(HaveOccurred()),
			checked:    BeFalse(),
			unmodified: dontCare,
		},
		{
			desc:       "toplevel-dir-checked-file",
			path:       "../../../go.mod",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: dontCare,
		},
		{
			desc:       "another-dir-checked-file",
			path:       "../../../manifest/testdata/contour",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: dontCare,
		},
		{
			desc:       "another-dir-checked-dir",
			path:       "../../../manifest/testdata/contour",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: dontCare,
		},
		{
			desc:       "empty-file",
			path:       "testdata/dir1/empty",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: BeTrue(),
		},
		{
			desc:       "modify-checked-file",
			path:       "testdata/dir1/modify",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: BeFalse(),
			setup:      makeModifyFileSetupFunc("testdata/dir1/modify"),
		},
		{
			desc:       "unchecked-file",
			path:       "testdata/dir1/unchecked",
			err:        Not(HaveOccurred()),
			checked:    BeFalse(),
			unmodified: BeFalse(),
			setup:      makeCreateFileSetupFunc("testdata/dir1/unchecked"),
		},
		{
			desc:       "testdata-clean-dir",
			path:       "testdata/dir1",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: BeTrue(),
		},
		{
			desc:       "unchecked-dir",
			path:       "testdata/dir1/unchecked-dir",
			err:        Not(HaveOccurred()),
			checked:    BeFalse(),
			unmodified: BeFalse(),
			setup: func(t *testing.T, tc *gitTestCases) {
				g := NewWithT(t)

				g.Expect(os.Mkdir(tc.path, 0755)).To(Succeed())

				tc.cleanup = func(t *testing.T, tc *gitTestCases) {
					g := NewWithT(t)

					g.Expect(os.RemoveAll(tc.path)).To(Succeed())
				}
			},
		},
		{
			desc:       "testdata-dir-modify-checked-file",
			path:       "testdata/dir2",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: BeFalse(),
			setup:      makeModifyFileSetupFunc("testdata/dir2/modify"),
		},
		{
			desc:       "testdata-dir-modify-checked-file-outside",
			path:       "testdata/dir2",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: BeTrue(),
			setup:      makeModifyFileSetupFunc("testdata/dir1/modify"),
		},
		{
			desc:       "testdata-dir-new-file",
			path:       "testdata/dir2",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: BeFalse(),
			setup:      makeCreateFileSetupFunc("testdata/dir2/new-file"),
		},
		{
			desc:       "testdata-dir-new-file-outside",
			path:       "testdata/dir2",
			err:        Not(HaveOccurred()),
			checked:    BeTrue(),
			unmodified: BeTrue(),
			setup:      makeCreateFileSetupFunc("testdata/dir1/new-file"),
		},
	}

	for _, path := range []string{
		"testdata/dir1/new-file",
		"testdata/dir1/unchecked-dir",
		"testdata/dir2/new-file",
		"testdata/dir1/unchecked",
	} {
		_ = os.RemoveAll(path)
	}

	for i := range cases {
		tc := cases[i]
		t.Run(tc.desc, func(t *testing.T) {
			// cannot use t.Parallel() here since same directory is used for all test cases
			if tc.setup != nil {
				tc.setup(t, &tc)
			}

			g := NewWithT(t)

			checked, unmodified, err := NewPathChecker(tc.path, "").Check()
			g.Expect(err).To(tc.err)
			g.Expect(checked).To(tc.checked)
			g.Expect(unmodified).To(tc.unmodified)

			if tc.cleanup != nil {
				tc.cleanup(t, &tc)
			}
		})
	}
}

func makeModifyFileSetupFunc(path string) func(*testing.T, *gitTestCases) {
	return func(t *testing.T, tc *gitTestCases) {
		g := NewWithT(t)

		file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0)
		g.Expect(err).NotTo(HaveOccurred())

		originalContents, err := io.ReadAll(file)
		g.Expect(err).NotTo(HaveOccurred())

		_, err = file.WriteString("\n\n\n")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(file.Close()).To(Succeed())

		tc.cleanup = func(t *testing.T, tc *gitTestCases) {
			g := NewWithT(t)

			file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
			g.Expect(err).NotTo(HaveOccurred())

			_, err = file.Write(originalContents)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(file.Close()).To(Succeed())
		}
	}
}

func makeCreateFileSetupFunc(path string) func(*testing.T, *gitTestCases) {
	return func(t *testing.T, tc *gitTestCases) {
		g := NewWithT(t)

		file, err := os.Create(path)
		g.Expect(err).NotTo(HaveOccurred())
		_, err = file.WriteString("something")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(file.Close()).To(Succeed())

		tc.cleanup = func(t *testing.T, tc *gitTestCases) {
			g := NewWithT(t)

			g.Expect(os.Remove(path)).To(Succeed())
		}
	}
}
