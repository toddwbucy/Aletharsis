// Package publication writes new artifact bundles in a caller-owned directory.
// The caller must prevent concurrent namespace mutation in that directory.
package publication

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"github.com/toddwbucy/Aletharsis/internal/reporters"
)

var ErrInvalid = errors.New("invalid publication request")
var ErrLimit = errors.New("publication resource limit")
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}\.(txt|json|diff)$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Artifact struct {
	Name string
	Data []byte
}
type Entry struct {
	Name   string `json:"name"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Contract     string  `json:"contract"`
	SourceSHA256 string  `json:"source_sha256"`
	ReportSHA256 string  `json:"report_artifact_sha256"`
	Artifacts    []Entry `json:"artifacts"`
}
type Receipt struct {
	Manifest       Manifest
	ManifestSHA256 string
}
type Limits struct{ Files, Bytes int }

// Publish writes flat, exclusive 0600 files and publishes manifest.json last.
// root must be a private, caller-controlled output directory, separate from the
// source tree. It never follows an existing final-path symlink or overwrites a
// hard link. Source/report hashes identify caller-verified artifacts; this
// package does not acquire them or authenticate caller claims.
// On failure it removes only files created by this call; cleanup errors are
// returned as well. A crash may leave partial files without a manifest. This is
// not an atomic directory transaction or a power-loss durability guarantee.
func Publish(root *os.Root, sourceSHA256, reportSHA256 string, artifacts []Artifact, limits Limits) (*Receipt, error) {
	if root == nil {
		return nil, ErrInvalid
	}
	return publish(rootDirectory{root}, sourceSHA256, reportSHA256, artifacts, limits)
}

type destination interface {
	create(string) (io.WriteCloser, error)
	remove(string) error
}
type rootDirectory struct{ root *os.Root }

func (d rootDirectory) create(name string) (io.WriteCloser, error) {
	return d.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
}
func (d rootDirectory) remove(name string) error { return d.root.Remove(name) }

func publish(dir destination, sourceSHA256, reportSHA256 string, artifacts []Artifact, limits Limits) (*Receipt, error) {
	if !digestPattern.MatchString(sourceSHA256) || !digestPattern.MatchString(reportSHA256) {
		return nil, ErrInvalid
	}
	if limits.Files <= 0 || limits.Bytes <= 0 || len(artifacts) == 0 || len(artifacts) > min(limits.Files, 64) {
		return nil, ErrLimit
	}
	budget := min(limits.Bytes, 64<<20)
	items := append([]Artifact(nil), artifacts...)
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	m := Manifest{Contract: "aletharsis.artifact-bundle/1", SourceSHA256: sourceSHA256, ReportSHA256: reportSHA256, Artifacts: []Entry{}}
	for i, a := range items {
		if !namePattern.MatchString(a.Name) || a.Name == "manifest.json" || (i > 0 && items[i-1].Name == a.Name) {
			return nil, ErrInvalid
		}
		if len(a.Data) > budget {
			return nil, ErrLimit
		}
		budget -= len(a.Data)
		m.Artifacts = append(m.Artifacts, Entry{Name: a.Name, Size: len(a.Data), SHA256: evidence.Hash(a.Data)})
	}
	encoded, err := reporters.JSON(m, false)
	if err != nil {
		return nil, err
	}
	raw := []byte(encoded)
	if len(raw) > budget {
		return nil, ErrLimit
	}
	items = append(items, Artifact{Name: "manifest.json", Data: raw})
	created := []string{}
	rollback := func(cause error) (*Receipt, error) {
		for i := len(created) - 1; i >= 0; i-- {
			if err := dir.remove(created[i]); err != nil {
				cause = errors.Join(cause, fmt.Errorf("cleanup %s: %w", created[i], err))
			}
		}
		return nil, cause
	}
	for _, a := range items {
		f, err := dir.create(a.Name)
		if err != nil {
			return rollback(err)
		}
		created = append(created, a.Name)
		n, writeErr := f.Write(a.Data)
		if writeErr == nil && n != len(a.Data) {
			writeErr = io.ErrShortWrite
		}
		closeErr := f.Close()
		if err := errors.Join(writeErr, closeErr); err != nil {
			return rollback(err)
		}
	}
	return &Receipt{Manifest: m, ManifestSHA256: evidence.Hash(raw)}, nil
}
