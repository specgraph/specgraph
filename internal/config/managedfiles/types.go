// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// Strategy selects how a ManagedFile is read, classified, and written.
type Strategy int

// Strategy values. Order is fixed; do not reorder (callers may compare
// by value via the iota positions).
const (
	StrategyJSONKeyMerge Strategy = iota
	StrategyMarkdownBlock
	StrategyWholeFile
)

// State is the framework's drift classification for a single managed file.
type State int

// State values. Synced is the only "no-op needed" state; the others all
// imply some action (write, refresh, or surface to the user).
const (
	StateMissing State = iota
	StateSynced
	StateStale   // sentinel hash matches disk content but disk doesn't match canonical
	StateDrifted // disk content does not match the recorded sentinel hash (user-edited)
)

// Harness is the agent-harness a ManagedFile belongs to.
type Harness int

// Harness values.
const (
	HarnessClaude Harness = iota
	HarnessCursor
	HarnessOpenCode
)

// CommentSyntax describes the comment style used to embed a sentinel
// line in a file. Each value maps to a (open, close) pair — close is
// empty for line comments.
type CommentSyntax int

// CommentSyntax values cover the file types managed by the framework.
const (
	CommentNone   CommentSyntax = iota // JSON files: no sentinel possible
	CommentSlash                       // // ...    (TypeScript, Go)
	CommentHash                        // # ...     (shell, YAML)
	CommentHTML                        // <!-- ... --> (markdown, mdc)
)

// ManagedFile describes a single file specgraph manages in a project.
// Construct via the Manifest function; do not build literals at call sites.
type ManagedFile struct {
	// Path is the file location relative to the project root.
	Path string

	// Strategy selects how this file is read, classified, written.
	Strategy Strategy

	// Source is the path within the package's embedded source tree to
	// read the canonical content from. Empty for JSON-key-merge files
	// where the canonical is built programmatically from project config.
	Source string

	// Comment is the comment syntax used for sentinel lines in this file.
	Comment CommentSyntax

	// Harness is which agent-harness this file belongs to. Used to filter
	// the manifest by the user's enabled harnesses.
	Harness Harness

	// SupersedesPath is the project-relative path of an older file that
	// is replaced by this one (e.g. a `.md` cursor rule renamed to `.mdc`).
	// Empty when the file has no predecessor. Init deletes this path
	// after a successful guarded write — see supersedesGuardedDelete.
	SupersedesPath string

	// HasFrontmatter, when true, instructs the WholeFile strategy to position
	// the sentinel on the first body line *after* a leading YAML frontmatter
	// block (`---\n...\n---\n`) instead of on line 1. Required for Cursor's
	// .mdc rule format where the frontmatter must occupy line 1.
	//
	// Invariants (enforced by validateManifestEntry):
	//   - HasFrontmatter==true requires Strategy==StrategyWholeFile.
	//   - HasFrontmatter==true requires Comment != CommentNone.
	HasFrontmatter bool

	// Build is a closure that returns the canonical content for this
	// file given a ProjectParams. Mutually exclusive with Source and
	// JSONKeys: each manifest entry uses exactly one. MarkdownBlock
	// requires Build (canonical depends on per-project params);
	// WholeFile uses Source instead (canonical is a static embedded
	// asset); JSONKeyMerge uses the declarative JSONKeys form.
	//
	// Build MUST be a pure function of ProjectParams: same input →
	// byte-identical output, no FS reads, no clock, no env, no
	// randomness. TestManifestShape asserts this for every registered
	// entry. Without purity, Inspect and Sync can disagree on state.
	Build func(ProjectParams) ([]byte, error)

	// JSONKeys is the declarative form of managed JSON keys for
	// JSONKeyMerge entries. Mutually exclusive with Build (validator
	// enforces XOR). Only meaningful for StrategyJSONKeyMerge.
	JSONKeys []JSONManagedKey
}

// FileState is the result of Inspect for a single ManagedFile.
type FileState struct {
	Path         string
	Strategy     Strategy
	Harness      Harness // which harness owns this entry; populated by InspectAll
	State        State
	DiskHash     string // sha256 of current disk content (empty if Missing)
	SentinelHash string // hash recorded in disk sentinel (empty if no sentinel)
	EmbeddedHash string // sha256 of canonical source content
	Detail       string // human-readable explanation, used in doctor output
}

// Action is the outcome of a write attempt for a single ManagedFile.
type Action int

// Action values.
const (
	ActionNoOp      Action = iota // file already Synced; nothing written
	ActionCreated                 // file was Missing; canonical written
	ActionRefreshed               // file was Stale; canonical rewritten with fresh sentinel
	ActionSkipped                 // file was Drifted; init skipped without --force
	ActionForced                  // file was Drifted; --force overwrote
	ActionError                   // some error occurred; see Err
)

// SyncResult reports what Sync did for a single ManagedFile.
type SyncResult struct {
	Path   string
	Action Action
	Err    error

	// Detail is a human-readable explanation populated by strategies
	// for non-trivial cases (legacy-block purge counts, supersedes-path
	// drifted, --force --keep-edits semantics). Exact strings pinned
	// in testdata/detail-grammar.txt. Empty for the common case.
	Detail string
}

// SyncOptions controls Sync behaviour.
type SyncOptions struct {
	// Force overwrites Drifted files with canonical content. Default false.
	Force bool

	// KeepEdits, when used with Force, preserves drifted on-disk content
	// but updates the sentinel hash to match. Default false.
	KeepEdits bool
}
