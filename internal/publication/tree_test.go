package publication

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/toddwbucy/Aletharsis/internal/evidence"
)

var treeLimits = TreeLimits{Sources: 100, Nodes: 1000, Bytes: 1 << 20}

func treeArtifacts() map[string][]byte {
	return map[string][]byte{"report": []byte("{\"report\":true}\n"), "revealed": []byte("a⟦U+200B ZERO WIDTH SPACE⟧b\r\n"), "mapping": []byte("{\"mapping\":true}\n"), "faithful": []byte("faithful\n"), "display": []byte("display\n")}
}
func TestTreeLayoutIdentityAndDeterminism(t *testing.T) {
	sources := []TreeSource{{"notes/a.md", true}, {"broken.txt", true}, {"link", false}, {"empty.txt", true}}
	var previous []byte
	for pass := 0; pass < 2; pass++ {
		r, dir := root(t)
		tree, err := NewTree(r, sources, treeLimits)
		if err != nil {
			t.Fatal(err)
		}
		if err := tree.Record("notes/a.md", "revealed", "", digest, treeArtifacts()); err != nil {
			t.Fatal(err)
		}
		if err := tree.Record("broken.txt", "failed", "text.decode_failed", digest, map[string][]byte{"report": []byte("{\"failed\":true}\n")}); err != nil {
			t.Fatal(err)
		}
		if err := tree.Record("link", "skipped", "symlink_not_followed", "", nil); err != nil {
			t.Fatal(err)
		}
		empty := treeArtifacts()
		empty["revealed"] = nil
		empty["faithful"] = nil
		empty["display"] = nil
		if err := tree.Record("empty.txt", "revealed", "", digest, empty); err != nil {
			t.Fatal(err)
		}
		stream := []byte("{\"type\":\"summary\"}\n")
		receipt, err := tree.Commit(stream)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		if pass > 0 && !bytes.Equal(previous, raw) {
			t.Fatal("nondeterministic manifest")
		}
		previous = raw
		if evidence.Hash(raw) != receipt.ManifestSHA256 {
			t.Fatal("manifest hash")
		}
		var manifest TreeManifest
		if err := json.Unmarshal(raw, &manifest); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(manifest, receipt.Manifest) || len(manifest.Sources) != 4 || manifest.Corpus.SHA256 != evidence.Hash(stream) {
			t.Fatal("manifest mismatch")
		}
		for _, source := range manifest.Sources {
			for _, artifact := range source.Artifacts {
				data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(artifact.Name)))
				if err != nil || len(data) != artifact.Size || evidence.Hash(data) != artifact.SHA256 {
					t.Fatal("artifact identity mismatch", err)
				}
			}
			if source.State == "revealed" && len(source.Artifacts) != 5 {
				t.Fatal("missing representation")
			}
			if source.RelativePath == "broken.txt" && (len(source.Artifacts) != 1 || source.Artifacts[0].Name != "reports/broken.txt.json") {
				t.Fatal("failed source lost its report")
			}
		}
		if _, err := os.Stat(filepath.Join(dir, "revealed", "notes", "a.md")); err != nil {
			t.Fatal("relative hierarchy lost", err)
		}
		if err := tree.Abort(); !errors.Is(err, ErrState) {
			t.Fatal("committed tree could be erased")
		}
	}
}
func TestTreePreflightCollisions(t *testing.T) {
	for _, paths := range [][]string{{"a", "a.json/b"}, {"A.txt", "a.txt"}, {"café.txt", "cafe\u0301.txt"}, {"A/one", "a/two"}, {"a", "a"}, {"../escape"}, {"a\\b"}, {"nul.txt"}, {"dir/con"}, {"trailing."}, {"bad\x1b.txt"}, {strings.Repeat("x", 252)}} {
		r, dir := root(t)
		sources := []TreeSource{}
		for _, name := range paths {
			sources = append(sources, TreeSource{name, true})
		}
		if tree, err := NewTree(r, sources, treeLimits); err == nil || tree != nil {
			t.Fatalf("unsafe plan accepted: %q", paths)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			t.Fatal("preflight wrote artifacts", err)
		}
	}
}
func TestTreeRequiresCompleteLedgerAndRollsBack(t *testing.T) {
	r, dir := root(t)
	tree, err := NewTree(r, []TreeSource{{"a", true}, {"b", true}}, treeLimits)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Record("a", "revealed", "", digest, treeArtifacts()); err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Commit([]byte("stream")); !errors.Is(err, ErrState) {
		t.Fatal("incomplete ledger committed")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("incomplete tree remained", err)
	}
	if err := tree.Record("b", "failed", "audit.failed", "", nil); !errors.Is(err, ErrState) {
		t.Fatal("failed transaction resumed")
	}
}
func TestTreeBoundsAndValidation(t *testing.T) {
	for _, limit := range []TreeLimits{{Sources: 1, Nodes: 100, Bytes: 1000}, {Sources: 10, Nodes: 1, Bytes: 1000}, {Sources: 10, Nodes: 5, Bytes: 1000}} {
		r, _ := root(t)
		if tree, err := NewTree(r, []TreeSource{{"a", true}, {"b", true}}, limit); !errors.Is(err, ErrLimit) || tree != nil {
			t.Fatal("plan bound ignored", err)
		}
	}
	for _, mode := range []string{"artifact_budget", "commit_budget", "unknown", "bad_hash", "bad_artifact", "bad_reason", "noncandidate"} {
		t.Run(mode, func(t *testing.T) {
			r, dir := root(t)
			limit := treeLimits
			if mode == "artifact_budget" {
				limit.Bytes = 1
			}
			if mode == "commit_budget" {
				limit.Bytes = 300
			}
			tree, err := NewTree(r, []TreeSource{{"a", mode != "noncandidate"}}, limit)
			if err != nil {
				t.Fatal(err)
			}
			name, state, reason, hash, artifacts := "a", "revealed", "", digest, treeArtifacts()
			switch mode {
			case "unknown":
				name = "unknown"
			case "bad_hash":
				hash = "invalid"
			case "bad_artifact":
				delete(artifacts, "mapping")
			case "bad_reason":
				state = "failed"
				reason = "unbounded prose is not a reason code"
				artifacts = nil
				hash = ""
			}
			err = tree.Record(name, state, reason, hash, artifacts)
			if mode == "commit_budget" && err == nil {
				_, err = tree.Commit([]byte("stream"))
			}
			if err == nil {
				t.Fatal("invalid record committed")
			}
			entries, e := os.ReadDir(dir)
			if e != nil || len(entries) != 0 {
				t.Fatal("failure retained partial tree", e)
			}
		})
	}
}
func TestTreeRejectsExistingAndProtectsReplacement(t *testing.T) {
	r, dir := root(t)
	sentinel := filepath.Join(dir, "existing")
	if err := os.WriteFile(sentinel, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTree(r, nil, treeLimits); !errors.Is(err, ErrCollision) {
		t.Fatal("nonempty output root accepted")
	}
	if err := os.Remove(sentinel); err != nil {
		t.Fatal(err)
	}
	tree, err := NewTree(r, []TreeSource{{"a", true}}, treeLimits)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Record("a", "revealed", "", digest, treeArtifacts()); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "revealed", "a")
	// Keep the old inode alive to prevent inode reuse in the replacement check.
	moved := filepath.Join(t.TempDir(), "original-derivative")
	if err := os.Rename(target, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("stranger"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := tree.Abort(); !errors.Is(err, ErrCollision) {
		t.Fatal("replacement identity not checked", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "stranger" {
		t.Fatal("rollback deleted a substituted file", err)
	}
}
func TestTreeEmptyWorkspaceAndSkippedDirectory(t *testing.T) {
	r, _ := root(t)
	tree, err := NewTree(r, []TreeSource{{"failed-dir", false}, {"failed-dir/file", true}}, treeLimits)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Record("failed-dir", "failed", "directory_unavailable", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := tree.Record("failed-dir/file", "revealed", "", digest, treeArtifacts()); err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Commit([]byte("stream")); err != nil {
		t.Fatal(err)
	}
	empty, _ := root(t)
	tree, err = NewTree(empty, nil, treeLimits)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := tree.Commit([]byte("stream"))
	if err != nil || len(receipt.Manifest.Sources) != 0 {
		t.Fatal("empty workspace failed", err)
	}
}

func TestTreeLateManifestCollisionPreservesExistingFile(t *testing.T) {
	r, dir := root(t)
	tree, err := NewTree(r, []TreeSource{{"a", true}}, treeLimits)
	if err != nil {
		t.Fatal(err)
	}
	if err := tree.Record("a", "revealed", "", digest, treeArtifacts()); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := tree.Commit([]byte("stream")); err == nil {
		t.Fatal("late collision overwritten")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "manifest.json" {
		t.Fatal("rollback left owned artifacts", entries, err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "keep" {
		t.Fatal("foreign manifest overwritten", err)
	}
}

func TestTreeLateReportLinkDoesNotTouchSource(t *testing.T) {
	for _, kind := range []string{"hard", "symbolic"} {
		t.Run(kind, func(t *testing.T) {
			r, dir := root(t)
			tree, err := NewTree(r, []TreeSource{{"a", true}, {"b", true}}, treeLimits)
			if err != nil {
				t.Fatal(err)
			}
			if err := tree.Record("a", "revealed", "", digest, treeArtifacts()); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(t.TempDir(), "original")
			data := []byte("original evidence")
			if err := os.WriteFile(source, data, 0600); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "reports", "b.json")
			if kind == "hard" {
				err = os.Link(source, target)
			} else {
				err = os.Symlink(source, target)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := tree.Record("b", "revealed", "", digest, treeArtifacts()); err == nil {
				t.Fatal("existing link accepted")
			}
			after, err := os.Stat(source)
			if err != nil || !reflect.DeepEqual(before.Sys(), after.Sys()) {
				t.Fatal("source metadata changed", err)
			}
			untouched, err := os.ReadFile(source)
			if err != nil || !bytes.Equal(untouched, data) {
				t.Fatal("source content changed", err)
			}
			if _, err := os.Lstat(target); err != nil {
				t.Fatal("foreign link removed", err)
			}
		})
	}
}
