package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const manifestFileName = ".bogo-manifest.json"

// genOptions are the command line switches that control how files are written
type genOptions struct {
	DryRun bool // report what would happen without touching the disk
	Force  bool // overwrite files even if they were modified after generation
}

type manifestFile struct {
	Version int `json:"version"`
	// Files holds the hash each file had on disk when the generator last left it
	Files map[string]string `json:"files"`
	// Generated holds the hash of the content the generator produced, before goimports ran.
	// It lets a rerun see that nothing changed even though goimports rewrote the file.
	Generated map[string]string `json:"generated,omitempty"`
}

// writeTracker remembers what the generator did during one run. The manifest maps
// each generated file to the hash it had when the generator last left it, so a
// later run can tell "untouched since generation" from "edited by hand".
type writeTracker struct {
	opts     genOptions
	root     string
	manifest  map[string]string
	generated map[string]string

	created   []string
	updated   []string
	unchanged []string
	skipped   []string

	written []string // paths actually written to disk during this run
}

var tracker = &writeTracker{manifest: map[string]string{}, generated: map[string]string{}}

// startTracking resets the tracker for a new run and loads the previous manifest
func startTracking(root string, opts genOptions) {
	tracker = &writeTracker{opts: opts, root: root, manifest: map[string]string{}, generated: map[string]string{}}

	data, err := os.ReadFile(filepath.Join(root, manifestFileName))
	if err != nil {
		return
	}
	var m manifestFile
	if json.Unmarshal(data, &m) == nil && m.Files != nil {
		tracker.manifest = m.Files
		if m.Generated != nil {
			tracker.generated = m.Generated
		}
	}
}

func hashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func (t *writeTracker) relPath(path string) string {
	if rel, err := filepath.Rel(t.root, path); err == nil && t.root != "" {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(path)
}

// writeManagedFile writes generated content unless doing so would destroy manual edits
func writeManagedFile(filePath, content string, perm os.FileMode) error {
	t := tracker
	data := []byte(content)
	if strings.HasSuffix(filePath, ".go") {
		// Compare and write in gofmt form so reruns do not report formatting noise
		if formatted, err := format.Source(data); err == nil {
			data = formatted
		}
	}
	newHash := hashContent(data)
	rel := t.relPath(filePath)

	existing, readErr := os.ReadFile(filePath)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		// Only a missing file may be created. A file that exists but cannot be read (permissions,
		// a directory in the way) must not be treated as absent and overwritten.
		return fmt.Errorf("failed to read %s: %v", filePath, readErr)
	}
	exists := readErr == nil

	switch {
	case !exists:
		t.created = append(t.created, rel)
		t.logAction("create", rel)
	case hashContent(existing) == newHash,
		t.generated[rel] == newHash && t.manifest[rel] == hashContent(existing):
		t.unchanged = append(t.unchanged, rel)
		t.logAction("unchanged", rel)
		if !t.opts.DryRun {
			// adopt files that match but were not tracked yet
			t.manifest[rel] = hashContent(existing)
			t.generated[rel] = newHash
		}
		return nil
	case t.manifest[rel] == hashContent(existing):
		t.updated = append(t.updated, rel)
		t.logAction("update", rel)
	case t.opts.Force:
		t.updated = append(t.updated, rel)
		t.logAction("overwrite", rel)
	default:
		t.skipped = append(t.skipped, rel)
		t.logAction("skip (modified)", rel)
		return nil
	}

	if t.opts.DryRun {
		return nil
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(filePath, data, perm); err != nil {
		return fmt.Errorf("failed to write file %s: %v", filePath, err)
	}

	t.manifest[rel] = newHash
	t.generated[rel] = newHash
	t.written = append(t.written, filePath)
	return nil
}

func (t *writeTracker) logAction(action, rel string) {
	if t.opts.DryRun {
		fmt.Printf("[dry-run] %-16s %s\n", action, rel)
		return
	}
	if action != "unchanged" {
		fmt.Printf("%-16s %s\n", action, rel)
	}
}

// writtenGoFiles returns the Go files written in this run, relative to the module root
func (t *writeTracker) writtenGoFiles() []string {
	var files []string
	for _, path := range t.written {
		if strings.HasSuffix(path, ".go") {
			files = append(files, t.relPath(path))
		}
	}
	return files
}

// refreshHashes re-hashes written files, picking up changes made by goimports
func (t *writeTracker) refreshHashes() {
	for _, path := range t.written {
		if data, err := os.ReadFile(path); err == nil {
			t.manifest[t.relPath(path)] = hashContent(data)
		}
	}
}

// saveManifest persists the manifest; it is a no-op for dry runs
func (t *writeTracker) saveManifest() error {
	if t.opts.DryRun || t.root == "" {
		return nil
	}
	data, err := json.MarshalIndent(manifestFile{Version: 1, Files: t.manifest, Generated: t.generated}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(t.root, manifestFileName), append(data, '\n'), 0644)
}

// printSummary reports what happened and how to resolve skipped files
func (t *writeTracker) printSummary() {
	prefix := ""
	if t.opts.DryRun {
		prefix = "[dry-run] "
	}
	fmt.Printf("\n%s%d created, %d updated, %d unchanged, %d skipped\n",
		prefix, len(t.created), len(t.updated), len(t.unchanged), len(t.skipped))

	if len(t.skipped) > 0 {
		sort.Strings(t.skipped)
		fmt.Printf("%sSkipped files were modified after generation (or not generated by boGO):\n", prefix)
		for _, rel := range t.skipped {
			fmt.Printf("  - %s\n", rel)
		}
		fmt.Printf("%sRe-run with --force to overwrite them.\n", prefix)
	}
}

// rebaseline records the current on-disk hash of a file written in this run as its generated
// state, so changes made by tools run right after generation (go mod tidy) are not seen as
// user edits. Files that were skipped keep their old baseline: their content is the user's.
func (t *writeTracker) rebaseline(path string) {
	if t.opts.DryRun || !t.wasWritten(path) {
		return
	}
	if data, err := os.ReadFile(path); err == nil {
		t.manifest[t.relPath(path)] = hashContent(data)
	}
}

// wasWritten reports whether the generator wrote this file during the current run
func (t *writeTracker) wasWritten(path string) bool {
	for _, written := range t.written {
		if written == path {
			return true
		}
	}
	return false
}
