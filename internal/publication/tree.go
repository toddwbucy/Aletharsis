package publication

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var ErrCollision = errors.New("artifact path collision")
var treeReasonPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,127}$`)
var ErrState = errors.New("publication transaction is not active or incomplete")

type TreeSource struct {
	RelativePath string
	Candidate    bool
}
type TreeLimits struct{ Sources, Nodes, Bytes int }
type SourceReceipt struct {
	RelativePath string  `json:"relative_path"`
	State        string  `json:"state"`
	Reason       string  `json:"reason"`
	SourceSHA256 string  `json:"source_sha256,omitempty"`
	ReportSHA256 string  `json:"report_artifact_sha256,omitempty"`
	Artifacts    []Entry `json:"artifacts"`
}
type TreeManifest struct {
	Contract string          `json:"contract"`
	Corpus   Entry           `json:"corpus"`
	Sources  []SourceReceipt `json:"sources"`
}
type TreeReceipt struct {
	Manifest       TreeManifest
	ManifestSHA256 string
}
type treePlan struct {
	candidate bool
	paths     map[string]string
}
type ownedNode struct {
	name string
	info os.FileInfo
}

// Tree is a single-owner transaction in a new private output root. Callers must
// exclude the source tree and prevent concurrent namespace/buffer mutation.
// It writes derivatives only, never acquires or edits source documents.
type Tree struct {
	root      *os.Root
	plan      map[string]treePlan
	records   map[string]SourceReceipt
	dirs      map[string]bool
	created   []ownedNode
	remaining int
	active    bool
}

// NewTree preflights all possible artifact paths before writing anything. Input
// names are preserved; NFC/full-case-fold keys only reject ambiguous layouts.
func NewTree(root *os.Root, sources []TreeSource, limits TreeLimits) (*Tree, error) {
	if root == nil || limits.Sources <= 0 || limits.Nodes <= 0 || limits.Bytes <= 0 {
		return nil, ErrInvalid
	}
	if min(limits.Nodes, 100000) < 2 || len(sources) > min(limits.Sources, 10000) {
		return nil, ErrLimit
	}
	t := &Tree{root: root, plan: map[string]treePlan{}, records: map[string]SourceReceipt{}, dirs: map[string]bool{}, remaining: min(limits.Bytes, 256<<20), active: true}
	files, dirs := map[string]string{}, map[string]string{}
	fold := cases.Fold()
	key := func(s string) string { return norm.NFC.String(fold.String(norm.NFC.String(s))) }
	total := 0
	for _, source := range sources {
		relative := source.RelativePath
		if !fs.ValidPath(relative) || relative == "." || !utf8.ValidString(relative) {
			return nil, ErrInvalid
		}
		if len(relative) > 4096 || strings.Count(relative, "/") > 64 || len(relative) > (4<<20)-total {
			return nil, ErrLimit
		}
		total += len(relative)
		if _, exists := t.plan[relative]; exists {
			return nil, ErrCollision
		}
		paths := map[string]string{}
		if source.Candidate {
			paths = map[string]string{"report": "reports/" + relative + ".json", "revealed": "revealed/" + relative, "mapping": "mappings/" + relative + ".json", "faithful": "diffs/" + relative + ".diff", "display": "display-diffs/" + relative + ".diff"}
		}
		for _, kind := range []string{"report", "revealed", "mapping", "faithful", "display"} {
			name, exists := paths[kind]
			if !exists {
				continue
			}
			if len(name) > 4096 {
				return nil, ErrLimit
			}
			for _, component := range strings.Split(name, "/") {
				if !portableComponent(component) {
					return nil, ErrInvalid
				}
			}
			k := key(name)
			if _, exists := files[k]; exists {
				return nil, ErrCollision
			}
			if _, exists := dirs[k]; exists {
				return nil, ErrCollision
			}
			files[k] = name
			for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
				k := key(parent)
				if _, exists := files[k]; exists {
					return nil, ErrCollision
				}
				if prior, exists := dirs[k]; exists && prior != parent {
					return nil, ErrCollision
				}
				dirs[k] = parent
			}
			if len(files)+len(dirs)+2 > min(limits.Nodes, 100000) {
				return nil, ErrLimit
			}
		}
		t.plan[relative] = treePlan{source.Candidate, paths}
	}
	// Read only the caller-designated output root, never a source directory.
	f, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	entries, readErr := f.ReadDir(1)
	closeErr := f.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, errors.Join(readErr, closeErr)
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(entries) != 0 {
		return nil, ErrCollision
	}
	return t, nil
}
func portableComponent(s string) bool {
	if len(s) == 0 || len(s) > 255 || strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") || strings.ContainsAny(s, `\:*?"<>|`) {
		return false
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return false
		}
	}
	stem := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return false
	}
	for _, prefix := range []string{"COM", "LPT"} {
		if strings.HasPrefix(stem, prefix) {
			suffix := strings.TrimPrefix(stem, prefix)
			if len([]rune(suffix)) == 1 && strings.Contains("123456789¹²³", suffix) {
				return false
			}
		}
	}
	return true
}

// Record accepts either all five named artifacts for revealed text, only a
// report for failed/unsupported/canceled candidates, or no artifacts for a skip.
// Artifact contents and source hashes must already be verified by the caller.
func (t *Tree) Record(relative, state, reason, sourceHash string, artifacts map[string][]byte) error {
	if !t.active {
		return ErrState
	}
	plan, ok := t.plan[relative]
	if !ok {
		return t.fail(ErrInvalid)
	}
	if _, done := t.records[relative]; done {
		return t.fail(ErrState)
	}
	if sourceHash != "" && !digestPattern.MatchString(sourceHash) {
		return t.fail(ErrInvalid)
	}
	if state == "revealed" {
		if !plan.candidate || sourceHash == "" || reason != "" || len(artifacts) != 5 {
			return t.fail(ErrInvalid)
		}
		for _, kind := range []string{"report", "revealed", "mapping", "faithful", "display"} {
			if _, ok := artifacts[kind]; !ok {
				return t.fail(ErrInvalid)
			}
		}
	} else {
		if state != "failed" && state != "unsupported" && state != "skipped" && state != "canceled" {
			return t.fail(ErrInvalid)
		}
		if !treeReasonPattern.MatchString(reason) || len(artifacts) > 1 {
			return t.fail(ErrInvalid)
		}
		for kind := range artifacts {
			if kind != "report" || !plan.candidate || state == "skipped" {
				return t.fail(ErrInvalid)
			}
		}
	}
	if report, ok := artifacts["report"]; ok {
		if len(report) == 0 {
			return t.fail(ErrInvalid)
		}
	} else if sourceHash != "" {
		return t.fail(ErrInvalid)
	}
	keys := make([]string, 0, len(artifacts))
	bytes := 0
	for kind, data := range artifacts {
		if len(data) > t.remaining-bytes {
			return t.fail(ErrLimit)
		}
		bytes += len(data)
		keys = append(keys, kind)
	}
	sort.Slice(keys, func(i, j int) bool { return plan.paths[keys[i]] < plan.paths[keys[j]] })
	receipt := SourceReceipt{RelativePath: relative, State: state, Reason: reason, SourceSHA256: sourceHash, Artifacts: []Entry{}}
	for _, kind := range keys {
		data, name := artifacts[kind], plan.paths[kind]
		if err := t.write(name, data); err != nil {
			return t.fail(err)
		}
		receipt.Artifacts = append(receipt.Artifacts, Entry{Name: name, Size: len(data), SHA256: evidence.Hash(data)})
		if kind == "report" {
			receipt.ReportSHA256 = evidence.Hash(data)
		}
	}
	t.records[relative] = receipt
	return nil
}

// Commit writes the exact corpus stream and manifest last. It requires a terminal
// record for every planned source. A crash is not an atomic directory transaction;
// consumers must validate the manifest and all artifact identities.
func (t *Tree) Commit(corpus []byte) (*TreeReceipt, error) {
	if !t.active {
		return nil, ErrState
	}
	if len(t.records) != len(t.plan) || len(corpus) == 0 {
		return nil, t.fail(ErrState)
	}
	manifest := TreeManifest{Contract: "aletharsis.reveal-tree/1", Corpus: Entry{Name: "corpus.jsonl", Size: len(corpus), SHA256: evidence.Hash(corpus)}, Sources: []SourceReceipt{}}
	for _, record := range t.records {
		manifest.Sources = append(manifest.Sources, record)
	}
	sort.Slice(manifest.Sources, func(i, j int) bool { return manifest.Sources[i].RelativePath < manifest.Sources[j].RelativePath })
	raw, err := json.Marshal(manifest)
	if err != nil {
		return nil, t.fail(err)
	}
	raw = append(raw, '\n')
	if len(corpus) > t.remaining || len(raw) > t.remaining-len(corpus) {
		return nil, t.fail(ErrLimit)
	}
	if err := t.write("corpus.jsonl", corpus); err != nil {
		return nil, t.fail(err)
	}
	if err := t.write("manifest.json", raw); err != nil {
		return nil, t.fail(err)
	}
	t.active = false
	return &TreeReceipt{Manifest: manifest, ManifestSHA256: evidence.Hash(raw)}, nil
}
func (t *Tree) write(name string, data []byte) error {
	parents := []string{}
	for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
		parents = append(parents, parent)
	}
	for i := len(parents) - 1; i >= 0; i-- {
		parent := parents[i]
		if t.dirs[parent] {
			continue
		}
		if err := t.root.Mkdir(parent, 0700); err != nil {
			return err
		}
		info, err := t.root.Lstat(parent)
		t.created = append(t.created, ownedNode{parent, info})
		if err != nil {
			return err
		}
		t.dirs[parent] = true
	}
	f, err := t.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	info, statErr := f.Stat()
	// Retain the created path even if stat fails; rollback refuses to guess its
	// identity and reports the incomplete cleanup rather than deleting a stranger.
	t.created = append(t.created, ownedNode{name, info})
	if statErr != nil {
		return errors.Join(statErr, f.Close())
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return err
	}
	t.remaining -= len(data)
	return nil
}

// Abort removes only still-identical nodes created by this transaction, never a
// pre-existing or substituted path. A committed tree cannot be aborted.
func (t *Tree) Abort() error {
	if !t.active {
		return ErrState
	}
	return t.fail(nil)
}
func (t *Tree) fail(cause error) error {
	t.active = false
	for i := len(t.created) - 1; i >= 0; i-- {
		node := t.created[i]
		info, err := t.root.Lstat(node.name)
		if err == nil && (node.info == nil || !os.SameFile(info, node.info)) {
			err = ErrCollision
		}
		if err == nil {
			err = t.root.Remove(node.name)
		}
		if err != nil {
			cause = errors.Join(cause, fmt.Errorf("cleanup %s: %w", node.name, err))
		}
	}
	return cause
}
