package v4

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/identity"
)

// Bind every archived member, not only the final text part. A deliberately
// failed CRC fixture still binds its compressed bytes and must fail decompression.
func checkFixtureArchive(raw []byte, r Report) error {
	if r.File.SHA256 == nil || *r.File.SHA256 != evidence.Hash(raw) || r.File.Size == nil || int64(*r.File.Size) != int64(len(raw)) {
		return fmt.Errorf("source identity")
	}
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	pkg := r.Evidence.Office.Packages[0]
	if pkg.SourceSHA256 != evidence.Hash(raw) || pkg.SourceByteLength != int64(len(raw)) || len(pkg.Parts) != len(z.File) {
		return fmt.Errorf("package identity")
	}
	members := map[string]*zip.File{}
	for _, f := range z.File {
		if members[f.Name] != nil {
			return fmt.Errorf("duplicate member")
		}
		members[f.Name] = f
	}
	for _, p := range pkg.Parts {
		f := members[p.Name]
		if f == nil {
			return fmt.Errorf("missing member %s", p.Name)
		}
		offset, err := f.DataOffset()
		if err != nil {
			return err
		}
		end := offset + int64(f.CompressedSize64)
		if offset < 0 || end < offset || end > int64(len(raw)) || p.CompressedSpan != (Span{offset, end}) || p.Method != int64(f.Method) || p.CompressedSHA256 != evidence.Hash(raw[offset:end]) {
			return fmt.Errorf("compressed identity %s", p.Name)
		}
		reader, err := f.Open()
		if err != nil {
			return err
		}
		body, readErr := io.ReadAll(io.LimitReader(reader, 1<<20))
		closeErr := reader.Close()
		if closeErr != nil {
			return closeErr
		}
		if p.State == "completed" {
			if readErr != nil {
				return readErr
			}
			if len(body) >= 1<<20 || p.ByteLength == nil || *p.ByteLength != int64(len(body)) || p.SHA256 == nil || *p.SHA256 != evidence.Hash(body) {
				return fmt.Errorf("payload identity %s", p.Name)
			}
		} else if readErr == nil || p.ByteLength != nil || p.SHA256 != nil {
			return fmt.Errorf("expected failed payload %s", p.Name)
		}
	}
	return nil
}

func TestAllOfficeFixtureArchiveIdentities(t *testing.T) {
	paths, err := filepath.Glob("../../../tests/contracts_v4/fixtures/office-*.json")
	if err != nil || len(paths) < 4 {
		t.Fatal("at least four Office fixtures required", err)
	}
	paths = append(paths, "../../../tests/contracts_v4/fixtures/metadata-inventory.json")
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			encoded, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			r, err := DecodeReport(encoded, identity.Limits{InputBytes: 16 << 20, OutputBytes: 16 << 20, Nodes: 4194304, Depth: 64})
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), r.File.Filename))
			if err != nil {
				t.Fatal(err)
			}
			if err := checkFixtureArchive(raw, r); err != nil {
				t.Fatal(err)
			}
			for _, p := range r.Evidence.Office.Packages[0].Parts {
				if p.CompressedSpan.Start == p.CompressedSpan.End {
					continue
				}
				corrupted := bytes.Clone(raw)
				corrupted[p.CompressedSpan.Start] ^= 1
				if checkFixtureArchive(corrupted, r) == nil {
					t.Fatal("source corruption accepted", p.Name)
				}
				changed := r
				changed.File.SHA256 = str(evidence.Hash(corrupted))
				changed.Evidence.Office.Packages = append([]Package{}, r.Evidence.Office.Packages...)
				changed.Evidence.Office.Packages[0].SourceSHA256 = evidence.Hash(corrupted)
				if err := checkFixtureArchive(corrupted, changed); err == nil || err.Error() == "source identity" || err.Error() == "package identity" {
					t.Fatal("part binding not exercised", p.Name, err)
				}

			}
			for i := range r.Evidence.Office.Packages[0].Parts {
				p := &r.Evidence.Office.Packages[0].Parts[i]
				saved := p.CompressedSHA256
				p.CompressedSHA256 = "wrong"
				if checkFixtureArchive(raw, r) == nil {
					t.Fatal("compressed digest mismatch accepted", p.Name)
				}
				p.CompressedSHA256 = saved
				if p.SHA256 != nil {
					saved := *p.SHA256
					*p.SHA256 = "wrong"
					if checkFixtureArchive(raw, r) == nil {
						t.Fatal("payload digest mismatch accepted", p.Name)
					}
					*p.SHA256 = saved
				}
			}
		})
	}
}
