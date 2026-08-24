package canon

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type instructionException struct {
	path   string
	text   string
	reason string
}

var secondAction = regexp.MustCompile(`(?i)\b(?:and|or)\s+(?:also\s+)?(?:add|adopt|allow|apply|ask|avoid|begin|capture|change|check|classify|complete|confirm|continue|create|define|delete|detect|draft|edit|emit|end|ensure|fail|follow|format|infer|install|keep|leave|limit|make|mark|move|omit|pass|place|prefer|preserve|prevent|prohibit|record|refresh|reject|remove|render|replace|require|reserve|resolve|restore|return|reuse|review|route|run|scan|set|skip|split|start|state|stop|synchronize|treat|update|use|validate|verify|wait|write)\b|\bpre-\s+and\s+post-`)
var acDocument = regexp.MustCompile(`^ac[0-9]+-`)

var instructionExceptions = []instructionException{
	{"AGENTS.md", "Ask the Director to narrow the task or split the AC before proposing delegation when the task exceeds practical inline capacity.", "one ask action offers two exclusive request objects"},
	{"AGENTS.md", "Map every in-scope command entry point, provider/API fetch, normalized-table write, durable snapshot, stale fallback, freshness gate, and complete-snapshot reconciliation path in the closure audit.", "one map action applies to a path-category list"},
	{"AGENTS.md", "Install or replace `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test, routing outcome, and validation disposition passes.", "exclusive operations apply to one target state"},
	{"AGENTS.md", "Reach for `Read` only to fetch unseen content or check for recent changes.", "one reach action has two exclusive purposes"},
	{"govna/development-cycle.md", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/overlays/code/files/govna/development-cycle.md.tmpl", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/overlays/doc/files/govna/editing-cycle.md.tmpl", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Ask the Director to narrow the task or split the AC before proposing delegation when the task exceeds practical inline capacity.", "one ask action offers two exclusive request objects"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Map every in-scope command entry point, provider/API fetch, normalized-table write, durable snapshot, stale fallback, freshness gate, and complete-snapshot reconciliation path in the closure audit.", "one map action applies to a path-category list"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Install or replace `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test, routing outcome, and validation disposition passes.", "exclusive operations apply to one target state"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Reach for `Read` only to fetch unseen content or check for recent changes.", "one reach action has two exclusive purposes"},
	{"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl", "Ask the Director to narrow the task or split the AC before proposing delegation when the task exceeds practical inline capacity.", "one ask action offers two exclusive request objects"},
	{"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl", "Install or replace `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test, routing outcome, and validation disposition passes.", "exclusive operations apply to one target state"},
	{"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl", "Reach for `Read` only to fetch unseen content or check for recent changes.", "one reach action has two exclusive purposes"},
	{"internal/canon/assets/overlays/code/files/arch.md.tmpl", "storage, messaging, or state boundaries", "architecture-outline list item is not an instruction"},
}

func TestInstructionStyleAtomicityGate(t *testing.T) {
	exceptions := make(map[string]instructionException, len(instructionExceptions))
	for _, exception := range instructionExceptions {
		if exception.path == "" || exception.text == "" || exception.reason == "" {
			t.Fatalf("incomplete instruction exception: %+v", exception)
		}
		key := exception.path + "\x00" + exception.text
		if prior, exists := exceptions[key]; exists {
			t.Fatalf("duplicate instruction exception for %s: %q and %q", exception.path, prior.reason, exception.reason)
		}
		exceptions[key] = exception
	}

	used := map[string]bool{}
	for path, content := range governanceCorpus(t) {
		for line := range strings.SplitSeq(content, "\n") {
			instruction, ok := ruleInstruction(line)
			if !ok || !secondAction.MatchString(instruction) {
				continue
			}
			key := path + "\x00" + instruction
			if _, allowed := exceptions[key]; !allowed {
				t.Errorf("%s: compound-action candidate requires a split or exact exception: %s", path, instruction)
				continue
			}
			used[key] = true
		}
	}
	for key, exception := range exceptions {
		if !used[key] {
			t.Errorf("%s: stale instruction exception: %s", exception.path, exception.text)
		}
	}
}

func TestCompoundActionDetection(t *testing.T) {
	for _, text := range []string{
		"Place the count paragraph first and start it with the required text.",
		"Reserve validation evidence and refresh it for Rust prep.",
		"Run ordinary canonical pre- and post-change validation.",
	} {
		if !secondAction.MatchString(text) {
			t.Errorf("did not reject compound action: %s", text)
		}
	}
	for _, text := range []string{
		"Reject missing, malformed, duplicate, or unsafe targets.",
		"Use `--dry-run` or `-n` to inspect without writes.",
		"Start the sentences with A, B, and C in that order.",
	} {
		if secondAction.MatchString(text) {
			t.Errorf("rejected compound object, alias, or result: %s", text)
		}
	}
}

func TestAtomicInstructionCorrections(t *testing.T) {
	want := map[string][]string{
		"govna/audit.md": {
			"- Place the count paragraph first under `## Summary`.",
			"- Start the count paragraph with `This adoption covers`.",
			"- Recompute the protected-region digest after adoption.",
			"- Require the protected-region digest to match the emitted digest.",
		},
		"govna/build-release.md": {
			"- Run ordinary canonical pre-change validation for Go prep.",
			"- Run ordinary canonical post-change validation for Go prep.",
			"- Reserve validation-token evidence for Rust prep.",
			"- Refresh validation-token evidence for Rust prep.",
			"- Rebuild a release from its clean tagged commit before publication.",
			"- Install the rebuilt release before publication.",
		},
		"govna/code-stacks.md": {
			"- Bump the root package version during release prep.",
			"- Refresh `Cargo.lock` during release prep.",
			"- Build only selected executable products during scoped builds.",
			"- Install only selected executable products during scoped builds.",
		},
		"govna/development-guidelines.md": {
			"- Never assume old data fits new schemas.",
			"- Write migration logic for old data.",
			"- Fail explicitly when migration logic is unavailable.",
		},
		"govna/roles.md": {
			"- Run each acceptance test in the active AC when it can be exercised.",
			"- Report the result of each exercised acceptance test.",
			"- State explicitly why each unexercised acceptance test was only reasoned about.",
		},
		"AGENTS.md": {
			"- Name the exact sections to change during every update.",
			"- Keep edits local during every update.",
			"- Stop when a request lacks authorization, scope, or required context.",
			"- Ask for the missing authorization, scope, or context.",
		},
	}
	corpus := governanceCorpus(t)
	for path, instructions := range want {
		content, ok := corpus[path]
		if !ok {
			t.Fatalf("governance corpus omits %s", path)
		}
		for _, instruction := range instructions {
			if !strings.Contains(content, instruction+"\n") {
				t.Errorf("%s omits atomic instruction %q", path, instruction)
			}
		}
	}
}

func governanceCorpus(t *testing.T) map[string]string {
	t.Helper()
	corpus := map[string]string{}
	root := filepath.Join("..", "..")
	paths, err := filepath.Glob(filepath.Join(root, "govna", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		name := filepath.Base(path)
		if acDocument.MatchString(name) {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		corpus[filepath.ToSlash(filepath.Join("govna", name))] = string(content)
	}
	if err := fs.WalkDir(assets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md.tmpl") {
			return nil
		}
		content, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}
		corpus["internal/canon/"+path] = string(content)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	corpus["AGENTS.md"] = string(agents)
	return corpus
}

func ruleInstruction(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- ") {
		return "", false
	}
	return strings.TrimPrefix(trimmed, "- "), true
}

func TestGovernanceCorpusOrder(t *testing.T) {
	paths := make([]string, 0, len(governanceCorpus(t)))
	for path := range governanceCorpus(t) {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 || paths[0] != "AGENTS.md" {
		t.Fatalf("unexpected governance corpus: %v", paths)
	}
}
