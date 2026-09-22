// Package workspace discovers bounded source candidates without interpreting them.
package workspace

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

var ErrLimit = errors.New("discovery resource limit")
var ErrChanged = errors.New("directory changed during discovery")
var ErrInvalid = errors.New("invalid discovery request")
var ErrUnavailable = errors.New("no-atime directory discovery unavailable")

type Options struct {
	Recursive  bool
	MaxEntries int
	MaxDepth   int
	PathBytes  int
}
type Entry struct {
	RelativePath string `json:"relative_path"`
	Kind         string `json:"kind"`
	State        string `json:"state"`
	Reason       string `json:"reason"`
}
type Result struct {
	Entries     []Entry `json:"entries"`
	Directories int     `json:"directories"`
	// Complete concerns directory enumeration, not content analysis or safety.
	Complete bool `json:"complete"`
}

// Discover performs no content audit and grants no source-read authority to its
// returned paths. The eventual acquisition must reopen safely and verify current
// identity. Symlinks and special files are explicit skips; no extension filters
// are applied. Limits/cancellation/change failures return no partial candidate
// set: callers must represent the entire discovery as failed or canceled.
func Discover(ctx context.Context, path string, options Options) (*Result, error) {
	if ctx == nil || options.MaxEntries <= 0 || options.MaxDepth < 0 || options.PathBytes <= 0 {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return DiscoverRoot(ctx, root, options)
}

// Open pins a real source directory. The caller owns and closes the handle.
func Open(path string) (*os.Root, error) {
	if path == "" || !utf8.ValidString(path) {
		return nil, ErrInvalid
	}
	if !available() {
		return nil, ErrUnavailable
	}
	return Pin(path)
}

// Pin verifies directory identity without enumeration or source-content reads.
// It does not establish that no-atime acquisition is available on this host.
func Pin(path string) (*os.Root, error) {
	if path == "" || !utf8.ValidString(path) {
		return nil, ErrInvalid
	}
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalid
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	actual, err := root.Stat(".")
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	if !os.SameFile(before, actual) {
		_ = root.Close()
		return nil, ErrChanged
	}
	return root, nil
}

// DiscoverRoot lets a corpus executor enumerate and acquire beneath the same
// pinned root even if the original directory pathname is later replaced.
func DiscoverRoot(ctx context.Context, root *os.Root, options Options) (*Result, error) {
	if ctx == nil || root == nil || options.MaxEntries <= 0 || options.MaxDepth < 0 || options.PathBytes <= 0 {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !available() {
		return nil, ErrUnavailable
	}
	options.MaxEntries = min(options.MaxEntries, 10000)
	options.MaxDepth = min(options.MaxDepth, 64)
	options.PathBytes = min(options.PathBytes, 4<<20)
	w := walker{ctx: ctx, options: options, result: Result{Entries: []Entry{}, Complete: true}}
	if err := w.walk(root, "", 0); err != nil {
		return nil, err
	}
	sort.Slice(w.result.Entries, func(i, j int) bool { return w.result.Entries[i].RelativePath < w.result.Entries[j].RelativePath })
	return &w.result, nil
}

type walker struct {
	ctx         context.Context
	options     Options
	result      Result
	seen, paths int
}

func (w *walker) add(path, kind, state, reason string) {
	w.result.Entries = append(w.result.Entries, Entry{path, kind, state, reason})
	if state == "failed" {
		w.result.Complete = false
	}
}
func (w *walker) walk(root *os.Root, prefix string, depth int) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	f, err := openDirectory(root)
	if err != nil {
		return err
	}
	before, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.result.Directories++
	names := []string{}
	for {
		if err := w.ctx.Err(); err != nil {
			_ = f.Close()
			return err
		}
		entries, readErr := f.ReadDir(128)
		for _, entry := range entries {
			name := entry.Name()
			relative := prefix + name
			if !utf8.ValidString(relative) || strings.ContainsRune(name, '/') || name == "." || name == ".." {
				_ = f.Close()
				return ErrInvalid
			}
			if w.seen >= w.options.MaxEntries || len(relative) > 4096 || len(relative) > w.options.PathBytes-w.paths {
				_ = f.Close()
				return ErrLimit
			}
			w.seen++
			w.paths += len(relative)
			names = append(names, name)
		}
		if endOfDirectory(readErr) {
			break
		}
		if readErr != nil {
			_ = f.Close()
			return readErr
		}
	}
	after, statErr := f.Stat()
	closeErr := f.Close()
	if statErr != nil || closeErr != nil {
		return errors.Join(statErr, closeErr)
	}
	if directoryChanged(before, after) {
		return ErrChanged
	}
	sort.Strings(names)
	for _, name := range names {
		if err := w.ctx.Err(); err != nil {
			return err
		}
		relative := prefix + name
		info, err := root.Lstat(name)
		if err != nil {
			w.add(relative, "unknown", "failed", "entry_unavailable")
			continue
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			w.add(relative, "symlink", "skipped", "symlink_not_followed")
		case info.IsDir():
			if !w.options.Recursive {
				w.add(relative, "directory", "skipped", "recursion_disabled")
				continue
			}
			if depth >= w.options.MaxDepth {
				return ErrLimit
			}
			// OpenRoot may follow an in-root link introduced by a concurrent rename.
			// Compare its pinned identity with the preceding non-following stat before
			// reading entries; openDirectory also uses O_NOFOLLOW on that pinned root.
			child, err := root.OpenRoot(name)
			if err != nil {
				w.add(relative, "directory", "failed", "directory_unavailable")
				continue
			}
			actual, statErr := child.Stat(".")
			if statErr != nil || !os.SameFile(info, actual) {
				_ = child.Close()
				return ErrChanged
			}
			walkErr := w.walk(child, relative+"/", depth+1)
			closeErr := child.Close()
			if errors.Is(walkErr, ErrLimit) || errors.Is(walkErr, ErrChanged) || errors.Is(walkErr, ErrInvalid) || errors.Is(walkErr, context.Canceled) || errors.Is(walkErr, context.DeadlineExceeded) {
				return walkErr
			}
			if walkErr != nil || closeErr != nil {
				w.add(relative, "directory", "failed", "directory_unavailable")
			}
		case info.Mode().IsRegular():
			w.add(relative, "file", "candidate", "")
		default:
			w.add(relative, "special", "skipped", "not_regular")
		}
	}
	// Catch directory edits during child traversal as well as initial enumeration.
	final, err := root.Stat(".")
	if err != nil {
		return err
	}
	if directoryChanged(before, final) {
		return ErrChanged
	}
	return nil
}
