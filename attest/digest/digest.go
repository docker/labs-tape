package digest

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"

	totoCommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

type SHA256 string

const algoSHA256 = "sha256"

func MakeSHA256(hash hash.Hash) SHA256 {
	return SHA256(hex.EncodeToString(hash.Sum(nil)))
}

func (d SHA256) DigestSet() totoCommon.DigestSet {
	digestSet := totoCommon.DigestSet{}
	if d != "" {
		digestSet[algoSHA256] = d.String()
	}
	return digestSet
}

func (d SHA256) MarshalJSON() ([]byte, error) { return json.Marshal(d.DigestSet()) }

func (d *SHA256) UnmarshalJSON(data []byte) error {
	digestSet := totoCommon.DigestSet{}

	if err := json.Unmarshal(data, &digestSet); err != nil {
		return err
	}
	v, ok := digestSet[algoSHA256]
	if !ok {
		return fmt.Errorf("SHA256 digest is missing")
	}
	*d = SHA256(v)
	return nil
}

func (d SHA256) IsEqual(checksum string) bool { return d.String() == checksum }

func (d SHA256) String() string { return string(d) }
