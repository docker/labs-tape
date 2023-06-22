package types

type Image struct {
	Manifest       string
	ManifestDigest string

	NodePath []string

	OriginalRef  string
	OriginalName string
	OriginalTag  string

	Digest string
	NewRef string
}
