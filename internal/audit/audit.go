package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/emission"
	"github.com/queone/govna/internal/repository"
	"github.com/queone/govna/internal/usererr"
)

const baselinePath = "govna/canon-baseline.txt"
const preservePath = "govna/preserve.txt"

type Config struct {
	Flavor, Stack, RepoName string
	JSON                    bool
	DiffLines               int
	invocation              string
}

type FileResult struct {
	Path                    string   `json:"relpath"`
	Classification          string   `json:"classification"`
	EffectiveClassification string   `json:"effective_classification,omitempty"`
	Diff                    string   `json:"diff,omitempty"`
	PriorCommits            []string `json:"commits,omitempty"`
	PreserveEntries         []string `json:"preserve_entries,omitempty"`
	LegacyPreserveMarkers   []string `json:"legacy_preserve_markers,omitempty"`
	CanonReference          string   `json:"canon_ref,omitempty"`
	CompareCommand          string   `json:"compare_command,omitempty"`
	Boundary                string   `json:"boundary,omitempty"`
	protectedHash           string
	forceSync               bool
	legacyOnly              bool
	targetHash              string
	targetPresent           bool
}

type Header struct {
	Invocation   string `json:"invocation"`
	CanonSHA     string `json:"canon_sha"`
	Target       string `json:"target"`
	Flavor       string `json:"flavor"`
	FlavorSource string `json:"flavor_source"`
	RepoName     string `json:"repo_name"`
	CanonVersion string `json:"canon_version"`
	CodeStack    string `json:"code_stack"`
}

type Emitted struct {
	ACStub string `json:"ac_stub"`
}
type Report struct {
	Header        Header       `json:"header"`
	Files         []FileResult `json:"files"`
	Emitted       *Emitted     `json:"emitted"`
	selectedStack string
}

type validationOutcomeKind uint8

const (
	validationUnresolved validationOutcomeKind = iota
	validationInferred
	validationNotApplicable
)

type validationOutcome struct {
	kind     validationOutcomeKind
	evidence string
	reason   string
}

type baseline struct {
	Version version
	Entries map[string]baselineEntry
}
type baselineEntry struct{ Scope, Hash string }
type version struct{ Major, Minor, Patch uint64 }
type coherenceRule struct {
	Reference string
	Targets   []string
}

var coherenceRules = []coherenceRule{{
	Reference: "govna/roles.md",
	Targets:   []string{"govna/build-release.md", "govna/release.md"},
}}

var semverRE = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var shaRE = regexp.MustCompile(`^[0-9a-f]{64}$`)
var nameReferenceRE = regexp.MustCompile(`(?:^|[[:space:]'"])(govna/[A-Za-z0-9._/-]+|AGENTS\.md|CHANGELOG\.md|README\.md|arch\.md|plan\.md)(?:$|[[:space:]'"),.:;])`)
var adoptionCommitRE = regexp.MustCompile(`(?i)(govna|^govern[a-z]*)`)

func Run(args []string, stdout, stderr io.Writer, cwd, programVersion string) int {
	cfg, err := parse(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report, clean, err := inspect(cfg, cwd)
	if err != nil {
		fmt.Fprintf(stderr, "audit: %v\n", err)
		return 1
	}
	if !clean {
		path, reused, err := emission.AuditPath(cwd, "v"+canon.Version, nil)
		if err != nil {
			fmt.Fprintf(stderr, "audit: %v\n", err)
			return 1
		}
		full := filepath.Join(cwd, filepath.FromSlash(path))
		if reused {
			old, err := os.ReadFile(full)
			if err != nil || !emission.VerifyAuditBody(old) {
				fmt.Fprintf(stderr, "audit: %s has been edited since last audit emission — to re-run, commit edits and delete the stub to regenerate, or rename the stub off the audit-v%s slug\n", path, canon.Version)
				return 1
			}
		}
		body := emission.AuditBody(programVersion, canon.Version, []byte(buildAC(report, path, validationDisposition(cwd, report))))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			fmt.Fprintf(stderr, "audit: %v\n", err)
			return 1
		}
		if err := os.WriteFile(full, body, 0o644); err != nil {
			fmt.Fprintf(stderr, "audit: %v\n", err)
			return 1
		}
		report.Emitted = &Emitted{ACStub: path}
	}
	if cfg.JSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(stderr, "audit: encode JSON report: %v\n", err)
			return 1
		}
	} else if clean {
		fmt.Fprintf(stdout, "No Govna updates or Director choices found (%s). No AC was written.\n", plainTally(report.Files))
	} else {
		fmt.Fprintf(stdout, "Wrote %s for review (%s).\n", report.Emitted.ACStub, plainTally(report.Files))
	}
	return 0
}

func parse(args []string) (Config, error) {
	c := Config{DiffLines: 200, invocation: strings.Join(append([]string{"govna", "audit"}, args...), " ")}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--flavor":
			i++
			if i >= len(args) {
				return c, fmt.Errorf("audit: -f, --flavor <code|doc> requires a value")
			}
			c.Flavor = args[i]
		case "-s", "--stack":
			i++
			if i >= len(args) {
				return c, fmt.Errorf("audit: -s, --stack <name> requires a value")
			}
			c.Stack = strings.TrimSpace(args[i])
			if c.Stack == "" {
				return c, fmt.Errorf("audit: -s, --stack <name> requires a non-empty value")
			}
		case "-j", "--json":
			c.JSON = true
		case "-l", "--diff-lines":
			i++
			if i >= len(args) {
				return c, fmt.Errorf("audit: -l, --diff-lines <N> requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				return c, fmt.Errorf("audit: --diff-lines must be a positive integer, got %q", args[i])
			}
			c.DiffLines = n
		case "-n", "--repo-name":
			i++
			if i >= len(args) {
				return c, fmt.Errorf("audit: -n, --repo-name <name> requires a value")
			}
			c.RepoName = args[i]
		default:
			return c, fmt.Errorf("audit: no positional arguments accepted; run from the target repo root (got: [%s])", args[i])
		}
	}
	if c.Flavor != "" && c.Flavor != "code" && c.Flavor != "doc" {
		return c, fmt.Errorf("audit: --flavor must be code or doc, got %q", c.Flavor)
	}
	return c, nil
}

func inspect(cfg Config, root string) (Report, bool, error) {
	var report Report
	if repository.IsSource(root) {
		return report, false, fmt.Errorf("an audit AC cannot be created inside the Govna source checkout at %s; run this command from the target repository", root)
	}
	if err := repository.RequireAdopted(root); err != nil {
		return report, false, err
	}
	if err := repository.RequireGitWorktree(root); err != nil {
		return report, false, err
	}
	metadata, metadataPresent, err := readMetadata(root)
	if err != nil {
		return report, false, err
	}
	flavorSource := "explicit"
	flavor, err := repository.Flavor(root, cfg.Flavor)
	if err != nil {
		return report, false, err
	}
	if cfg.Flavor == "" {
		if metadataPresent {
			flavorSource = "metadata"
		} else {
			flavorSource = "manifest"
		}
	}
	stack := cfg.Stack
	if flavor == canon.Code {
		if stack == "" {
			stack = metadata["code_stack"]
		}
		if stack == "" {
			stack = repository.Stack(root)
		}
		canonical, ok := canon.CanonicalStack(stack)
		if !ok {
			return report, false, fmt.Errorf("unsupported CODE stack %q", stack)
		}
		stack = canonical
	} else if stack != "" {
		return report, false, fmt.Errorf("CODE-only option used with DOC canon")
	}
	report.selectedStack = stack
	module := ""
	if stack == "Go" {
		module = repository.ModulePath(root)
	}
	name := repository.Name(root, module, cfg.RepoName)
	files, err := canon.Render(canon.Config{Flavor: flavor, RepoName: name, Stack: stack, ModulePath: module})
	if err != nil {
		return report, false, err
	}
	canonMap := map[string][]byte{}
	for _, file := range files {
		canonMap[file.Path] = file.Content
	}
	if err := checkCoherence(canonMap); err != nil {
		return report, false, err
	}
	preserve, err := ParsePreserve(root)
	if err != nil {
		return report, false, err
	}
	baseBytes, basePresent, err := readOptional(filepath.Join(root, filepath.FromSlash(baselinePath)))
	if err != nil {
		return report, false, err
	}
	var prior *baseline
	if basePresent {
		p, err := parseBaseline(baseBytes, flavor)
		if err != nil {
			return report, false, err
		}
		prior = &p
	}
	report.Header = Header{Invocation: cfg.invocation, CanonSHA: "v" + canon.Version, Target: root, Flavor: strings.ToLower(string(flavor)), FlavorSource: flavorSource, RepoName: name, CanonVersion: metadata["canon_version"], CodeStack: metadata["code_stack"]}
	legacy := legacyMarkers(root)
	paths := make([]string, 0, len(canonMap))
	for path := range canonMap {
		if path != baselinePath {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		fr := classify(root, path, canonMap[path], prior, preserve, cfg.DiffLines)
		if path == "govna/metadata.txt" {
			if !metadataPresent {
				fr.Classification = "migration-required"
				fr.CanonReference = "metadata absent"
			} else if metadata["canon_version"] != "v"+canon.Version {
				target, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
				replaced := strings.Replace(string(target), "canon_version = "+metadata["canon_version"], "canon_version = v"+canon.Version, 1)
				if replaced == string(canonMap[path]) {
					fr.Classification = "clear-sync"
					fr.PreserveEntries = nil
				} else {
					fr.Classification = "ambiguity"
				}
			}
		}
		formatNeedsSync := (path == "AGENTS.md" || path == "govna/ac-template.md") && fr.Classification != "match" && fr.Classification != "expected-divergence"
		if markers := legacy[path]; len(markers) > 0 {
			fr.LegacyPreserveMarkers = markers
			fr.legacyOnly = !actionable(fr.Classification) && !formatNeedsSync
			fr.Classification = "ambiguity"
			if fr.legacyOnly {
				captureTargetState(root, path, &fr)
			}
			delete(legacy, path)
		}
		if formatNeedsSync {
			fr.PreserveEntries = nil
			fr.forceSync = true
			fr.EffectiveClassification = "force-sync"
		}
		if fr.legacyOnly {
			fr.CompareCommand = "review exact legacy preserve phrase for " + path
		} else {
			fr.CompareCommand = comparisonDescription(fr, path)
		}
		report.Files = append(report.Files, fr)
	}
	if !basePresent {
		report.Files = append(report.Files, FileResult{Path: baselinePath, Classification: "migration-required", CanonReference: "generated baseline manifest", CompareCommand: "compare generated baseline with target govna/canon-baseline.txt"})
	} else if !bytes.Equal(baseBytes, canonMap[baselinePath]) {
		report.Files = append(report.Files, FileResult{Path: baselinePath, Classification: "clear-sync", CanonReference: "generated baseline manifest", CompareCommand: "compare generated baseline with target govna/canon-baseline.txt"})
	}
	for path, evidence := range targetOnly(root, canonMap, prior, flavor, name) {
		classification := "target-has-no-canon"
		if preserve[path] {
			classification = "preserve"
		}
		fr := FileResult{Path: path, Classification: classification, CanonReference: evidence, CompareCommand: "review " + path + " because it is not in the selected embedded Govna files"}
		if preserve[path] {
			fr.PreserveEntries = []string{path}
		}
		data, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		fr.Diff = diffText(nil, data, path, cfg.DiffLines)
		if markers := legacy[path]; len(markers) > 0 {
			fr.LegacyPreserveMarkers = markers
			if preserve[path] {
				fr.Classification = "ambiguity"
				fr.legacyOnly = true
				captureTargetState(root, path, &fr)
			} else {
				fr.Classification = "target-has-no-canon"
			}
			delete(legacy, path)
		}
		report.Files = append(report.Files, fr)
	}
	for path, markers := range legacy {
		fr := FileResult{Path: path, Classification: "ambiguity", LegacyPreserveMarkers: markers, legacyOnly: true}
		if preserve[path] {
			fr.PreserveEntries = []string{path}
		}
		data, present, _ := readOptional(filepath.Join(root, filepath.FromSlash(path)))
		fr.targetPresent = present
		if present {
			hash := sha256.Sum256(data)
			fr.targetHash = fmt.Sprintf("%x", hash)
		}
		report.Files = append(report.Files, fr)
	}
	sort.Slice(report.Files, func(i, j int) bool { return report.Files[i].Path < report.Files[j].Path })
	clean := true
	for _, file := range report.Files {
		if actionable(file.Classification) || file.forceSync {
			clean = false
		}
	}
	return report, clean, nil
}

func readMetadata(root string) (map[string]string, bool, error) {
	path := filepath.Join(root, "govna", "metadata.txt")
	data, present, err := readOptional(path)
	if err != nil || !present {
		return map[string]string{}, present, err
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return nil, true, fmt.Errorf("invalid govna/metadata.txt: require a final newline")
	}
	values := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, value, ok := strings.Cut(line, " = ")
		if !ok || key == "" || value == "" {
			return nil, true, fmt.Errorf("invalid govna/metadata.txt: each line must use `key = value`")
		}
		if _, found := values[key]; found {
			return nil, true, fmt.Errorf("invalid govna/metadata.txt: duplicate key %s", key)
		}
		values[key] = value
	}
	if values["repo_type"] != "CODE" && values["repo_type"] != "DOC" {
		return nil, true, fmt.Errorf("invalid govna/metadata.txt: repo_type must be CODE or DOC")
	}
	if values["canon_version"] == "" {
		return nil, true, fmt.Errorf("invalid govna/metadata.txt: missing canon_version")
	}
	if values["repo_type"] == "CODE" && values["code_stack"] == "" {
		return nil, true, fmt.Errorf("invalid govna/metadata.txt: missing code_stack")
	}
	if raw := values["canon_version"]; raw != "" {
		v, err := parseVersion(raw)
		if err != nil {
			return nil, true, fmt.Errorf("invalid target canon_version %q", raw)
		}
		embedded, _ := parseVersion("v" + canon.Version)
		if less(embedded, v) {
			return nil, true, fmt.Errorf("target canon_version %s is newer than embedded canon v%s; upgrade govna before auditing", raw, canon.Version)
		}
	}
	return values, true, nil
}

func parseVersion(raw string) (version, error) {
	m := semverRE.FindStringSubmatch(raw)
	if m == nil {
		return version{}, fmt.Errorf("strict vMAJOR.MINOR.PATCH required")
	}
	a, _ := strconv.ParseUint(m[1], 10, 64)
	b, _ := strconv.ParseUint(m[2], 10, 64)
	c, _ := strconv.ParseUint(m[3], 10, 64)
	return version{a, b, c}, nil
}
func less(a, b version) bool {
	if a.Major != b.Major {
		return a.Major < b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor < b.Minor
	}
	return a.Patch < b.Patch
}

func checkCoherence(files map[string][]byte) error {
	for _, rule := range coherenceRules {
		content, ok := files[rule.Reference]
		if !ok {
			return usererr.Errorf("embedded Govna files disagree with each other: %s is missing; report this to the Govna maintainer", rule.Reference)
		}
		present := ""
		for _, target := range rule.Targets {
			_, exists := files[target]
			referenced := bytes.Contains(content, []byte(target))
			if exists {
				if present != "" {
					return usererr.Errorf("embedded Govna files disagree with each other: %s has more than one release document; report this to the Govna maintainer", rule.Reference)
				}
				present = target
				if !referenced {
					return usererr.Errorf("embedded Govna files disagree with each other: %s must reference %s; report this to the Govna maintainer", rule.Reference, target)
				}
			} else if referenced {
				return usererr.Errorf("embedded Govna files disagree with each other: %s references missing %s; report this to the Govna maintainer", rule.Reference, target)
			}
		}
		if present == "" {
			return usererr.Errorf("embedded Govna files disagree with each other: %s has no release document; report this to the Govna maintainer", rule.Reference)
		}
	}
	return nil
}

func parseBaseline(data []byte, flavor canon.Flavor) (baseline, error) {
	bad := func(s string) (baseline, error) { return baseline{}, fmt.Errorf("invalid %s: %s", baselinePath, s) }
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return bad("require a final newline")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) < 2 || lines[0] != "govna-canon-baseline-v1" {
		return bad("first line must be govna-canon-baseline-v1")
	}
	raw, ok := strings.CutPrefix(lines[1], "canon_version = ")
	if !ok {
		return bad("second line must be canon_version = vMAJOR.MINOR.PATCH")
	}
	v, err := parseVersion(raw)
	if err != nil {
		return bad(err.Error())
	}
	embedded, _ := parseVersion("v" + canon.Version)
	if less(embedded, v) {
		return bad("baseline version is newer than embedded canon")
	}
	b := baseline{Version: v, Entries: map[string]baselineEntry{}}
	previous := ""
	for _, line := range lines[2:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 || fields[0] == "" || fields[1] == "" || !shaRE.MatchString(fields[2]) {
			return bad("each entry must be <path><TAB><scope><TAB><sha256>")
		}
		p, scope := fields[0], fields[1]
		if p <= previous {
			return bad("paths must be unique and byte-sorted")
		}
		previous = p
		if p == baselinePath || p == preservePath {
			return bad("manifest contains excluded control path")
		}
		boundary, mixed := canon.Boundary(p)
		valid := !mixed && scope == "full" || mixed && scope == "before:"+boundary || flavor == canon.Code && p == "govna/build-release.md" && scope == "full" && less(v, version{0, 11, 0})
		if !valid {
			return bad("invalid scope for " + p)
		}
		b.Entries[p] = baselineEntry{scope, fields[2]}
	}
	return b, nil
}

// ParsePreserve validates and returns the optional durable preserve registry.
func ParsePreserve(root string) (map[string]bool, error) {
	data, present, err := readOptional(filepath.Join(root, filepath.FromSlash(preservePath)))
	if err != nil || !present {
		return map[string]bool{}, err
	}
	bad := func(s string) (map[string]bool, error) { return nil, fmt.Errorf("invalid %s: %s", preservePath, s) }
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return bad("require a final newline")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if lines[0] != "govna-preserve-v1" {
		return bad("first line must be govna-preserve-v1")
	}
	out := map[string]bool{}
	previous := ""
	for _, p := range lines[1:] {
		if p == "" || p <= previous || p == preservePath || strings.ContainsAny(p, "\\\t") || filepath.IsAbs(p) || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
			return bad("entries must be nonempty, normalized, unique, and byte-sorted")
		}
		for part := range strings.SplitSeq(p, "/") {
			if part == "." || part == ".." || part == "" {
				return bad("entries contain invalid path components")
			}
		}
		previous = p
		out[p] = true
	}
	return out, nil
}

func classify(root, path string, want []byte, base *baseline, preserve map[string]bool, limit int) FileResult {
	fr := FileResult{Path: path, CanonReference: "govna @ v" + canon.Version + ": " + path}
	if preserve[path] {
		fr.PreserveEntries = []string{path}
	}
	got, present, _ := readOptional(filepath.Join(root, filepath.FromSlash(path)))
	if !present {
		if preserve[path] {
			fr.Classification = "match"
		} else {
			fr.Classification = "missing-in-target"
			fr.Diff = diffText(want, nil, path, limit)
		}
		return fr
	}
	if bytes.Equal(got, want) {
		fr.Classification = "match"
		return fr
	}
	if boundary, mixed := canon.Boundary(path); mixed {
		wr, wok := canon.ComparisonRegion(path, want)
		gr, gok := canon.ComparisonRegion(path, got)
		if wok && gok {
			fr.Boundary = boundary
			protected, _ := canon.ProtectedRegion(path, got)
			hash := sha256.Sum256(protected)
			fr.protectedHash = fmt.Sprintf("%x", hash)
			if bytes.Equal(wr, gr) {
				fr.Classification = "match"
				return fr
			}
		}
		if path == "govna/build-release.md" && !gok {
			fr.Classification = "ambiguity"
			fr.Diff = diffText(want, got, path, limit)
			return fr
		}
	}
	if path == "plan.md" || path == "arch.md" {
		fr.Classification = "expected-divergence"
		return fr
	}
	fr.Diff = diffText(want, got, path, limit)
	if preserve[path] {
		fr.Classification = "preserve"
		return fr
	}
	if base != nil {
		if entry, ok := base.Entries[path]; ok {
			region, valid := canon.ComparisonRegion(path, got)
			if valid {
				hash := sha256.Sum256(region)
				if fmt.Sprintf("%x", hash) == entry.Hash {
					fr.Classification = "clear-sync"
					return fr
				}
			}
		}
		fr.Classification = "ambiguity"
		return fr
	}
	fr.PriorCommits = gitLog(root, path)
	if len(fr.PriorCommits) == 0 {
		fr.Classification = "clear-sync"
	} else {
		fr.Classification = "ambiguity"
	}
	return fr
}

func gitLog(root, path string) []string {
	out, err := exec.Command("git", "-C", root, "log", "-n5", "--follow", "--pretty=oneline", "--", path).Output()
	if err != nil {
		return nil
	}
	var lines []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		subject := line
		if i := strings.IndexByte(line, ' '); i >= 0 {
			subject = line[i+1:]
		}
		if adoptionCommitRE.MatchString(subject) {
			line += " (adoption)"
		}
		lines = append(lines, line)
	}
	return lines
}

func targetOnly(root string, current map[string][]byte, base *baseline, flavor canon.Flavor, name string) map[string]string {
	out := map[string]string{}
	add := func(path, evidence string) {
		if path == preservePath {
			return
		}
		if _, ok := current[path]; ok {
			return
		}
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err == nil && info.Mode().IsRegular() {
			if _, exists := out[path]; !exists {
				out[path] = evidence
			}
		}
	}
	if base != nil {
		for path := range base.Entries {
			add(path, "present in prior canon baseline")
		}
	}
	if _, err := os.Stat(filepath.Join(root, "govna", "audit.md")); err == nil {
		add("govna/drift-scan.md", "retired canon path; replacement present: govna/audit.md")
	} else {
		add("govna/drift-scan.md", "retired canon path; replacement missing: govna/audit.md")
	}
	other := map[string]bool{}
	if flavor == canon.Code {
		files, _ := canon.Render(canon.Config{Flavor: canon.Doc, RepoName: name})
		for _, f := range files {
			other[f.Path] = true
		}
	} else {
		for _, stack := range canon.Stacks() {
			files, _ := canon.Render(canon.Config{Flavor: canon.Code, RepoName: name, Stack: stack, ModulePath: name})
			for _, f := range files {
				other[f.Path] = true
			}
		}
	}
	for path := range other {
		add(path, "present in other flavor canon")
	}
	for path := range current {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil || bytes.Equal(data, current[path]) {
			continue
		}
		for _, m := range nameReferenceRE.FindAllStringSubmatch(string(data), -1) {
			add(strings.TrimRight(m[1], ".,:;)]}"), "name-referenced from divergent governed file")
		}
	}
	return out
}

func legacyMarkers(root string) map[string][]string {
	out := map[string][]string{}
	data, _ := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	text := string(data)
	_, after, ok := strings.Cut(text, "## Unreleased")
	if !ok {
		return out
	}
	section := after
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	patterns := []*regexp.Regexp{regexp.MustCompile(`(?m)preserve ([A-Za-z0-9._/-]+)`), regexp.MustCompile(`(?m)do not sync ([A-Za-z0-9._/-]+)`), regexp.MustCompile(`(?m)intentional divergence: ([A-Za-z0-9._/-]+)`), regexp.MustCompile(`(?m)([A-Za-z0-9._/-]+): keep local`)}
	for _, re := range patterns {
		for _, m := range re.FindAllStringSubmatch(section, -1) {
			out[m[1]] = append(out[m[1]], m[0])
		}
	}
	return out
}

func diffText(want, got []byte, path string, limit int) string {
	a, err := os.CreateTemp("", "govna-canon-*")
	if err != nil {
		return "[diff unavailable]"
	}
	defer os.Remove(a.Name())
	defer a.Close()
	b, err := os.CreateTemp("", "govna-target-*")
	if err != nil {
		return "[diff unavailable]"
	}
	defer os.Remove(b.Name())
	defer b.Close()
	a.Write(want)
	b.Write(got)
	out, _ := exec.Command("diff", "-u", "-L", "canon/"+path, "-L", "target/"+path, a.Name(), b.Name()).CombinedOutput()
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(lines) > limit {
		extra := len(lines) - limit
		lines = lines[:limit]
		lines = append(lines, fmt.Sprintf("[... %d more lines truncated ...]", extra))
	}
	return strings.Join(lines, "\n")
}
func readOptional(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func captureTargetState(root, path string, file *FileResult) {
	data, present, _ := readOptional(filepath.Join(root, filepath.FromSlash(path)))
	file.targetPresent = present
	if present {
		hash := sha256.Sum256(data)
		file.targetHash = fmt.Sprintf("%x", hash)
	}
}

func comparisonDescription(f FileResult, path string) string {
	switch f.Classification {
	case "match":
		return "matches the embedded Govna file: " + path
	case "missing-in-target":
		return "repository is missing " + path + "; compare it with the embedded Govna file"
	case "target-has-no-canon":
		return "review " + path + " because it is not in the selected embedded Govna files"
	default:
		return "compare the embedded Govna file with the repository file: " + path
	}
}
func actionable(c string) bool {
	return c == "clear-sync" || c == "missing-in-target" || c == "migration-required" || c == "ambiguity" || c == "target-has-no-canon"
}

type classificationInfo struct{ singular, plural, meaning string }

// classificationOrder fixes the rendering order for plainTally; classificationInfos is the
// single source of truth for plainTally and classificationMeaning, plus the synthetic
// "force-sync" entry used for a file whose Classification is overridden by forceSync.
var classificationOrder = []string{"match", "expected-divergence", "preserve", "ambiguity", "clear-sync", "missing-in-target", "target-has-no-canon", "migration-required", "force-sync"}

var classificationInfos = map[string]classificationInfo{
	"match":               {"file needs no update", "files need no update", "the file already needs no Govna update"},
	"expected-divergence": {"expected local difference", "expected local differences", "the repository is expected to keep its own version of this file"},
	"preserve":            {"file kept by Director choice", "files kept by Director choice", "the preserve list says to keep the repository's version"},
	"ambiguity":           {"file needs a Director choice", "files need a Director choice", "Govna cannot safely choose between updating and keeping the file"},
	"clear-sync":          {"file is safe to update", "files are safe to update", "the file still matches the previously installed Govna version and is safe to update"},
	"missing-in-target":   {"missing Govna file", "missing Govna files", "a file from current Govna rules is missing from the repository"},
	"target-has-no-canon": {"Govna-linked extra file", "Govna-linked extra files", "the file is absent from the selected current canon but specific repository evidence connects it to Govna"},
	"migration-required":  {"missing required control file", "missing required control files", "a required Govna control file is missing and must be added through the AC"},
	"force-sync":          {"file always synced regardless of local edits", "files always synced regardless of local edits", "this file's governed structure always syncs to canon, regardless of local edits or the preserve list"},
}

// tallyKey resolves the classificationInfos key for a file, overriding Classification with the
// synthetic "force-sync" entry whenever forceSync is set so a governance-critical file can never
// render as its underlying (possibly stale) Classification in generated text or the CLI summary.
func tallyKey(f FileResult) string {
	if f.forceSync {
		return "force-sync"
	}
	return f.Classification
}

func plainTally(files []FileResult) string {
	count := map[string]int{}
	for _, file := range files {
		count[tallyKey(file)]++
	}
	var parts []string
	for _, classification := range classificationOrder {
		value := count[classification]
		if value == 0 {
			continue
		}
		info := classificationInfos[classification]
		label := info.plural
		if value == 1 {
			label = info.singular
		}
		parts = append(parts, fmt.Sprintf("%d %s", value, label))
	}
	if len(parts) == 0 {
		return "0 files"
	}
	return strings.Join(parts, ", ")
}

func classificationMeaning(classification string) string {
	return classificationInfos[classification].meaning
}

func replacementMissingPath(file FileResult) string {
	const marker = "replacement missing: "
	_, replacement, ok := strings.Cut(file.CanonReference, marker)
	if !ok {
		return ""
	}
	return strings.TrimSpace(replacement)
}

func markerOnly(file FileResult) bool {
	return len(file.LegacyPreserveMarkers) > 0 && file.legacyOnly
}

func quotedList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "`"+value+"`")
	}
	switch len(quoted) {
	case 0:
		return ""
	case 1:
		return quoted[0]
	case 2:
		return quoted[0] + " and " + quoted[1]
	default:
		return strings.Join(quoted[:len(quoted)-1], ", ") + ", and " + quoted[len(quoted)-1]
	}
}

func writeAT(b *strings.Builder, number *int, format string, args ...any) {
	fmt.Fprintf(b, "**AT%d** [Automated] [Pre-release gate] — ", *number)
	fmt.Fprintf(b, format, args...)
	b.WriteString("\n\n")
	(*number)++
}

func writeLegacyOrderingAT(b *strings.Builder, number *int, path string) {
	writeAT(b, number, "Verify legacy-phrase cleanup for `%s` starts only after every applicable target-state and registry-state AT passes.", path)
}

func writeLegacyPreservationATs(b *strings.Builder, number *int, path, phrases string) {
	writeAT(b, number, "Verify the Unreleased CHANGELOG Summary changes only through exact removal of %s for `%s`.", phrases, path)
	writeAT(b, number, "Verify every CHANGELOG line outside the Unreleased Summary remains byte-identical for `%s`.", path)
}

func writeRouteATs(b *strings.Builder, number *int, file FileResult) {
	path := file.Path
	phrases := quotedList(file.LegacyPreserveMarkers)
	if replacement := replacementMissingPath(file); replacement != "" {
		writeAT(b, number, "Verify `%s` matches its applicable rendered canon region before retired-source routing for `%s`.", replacement, path)
	}
	if markerOnly(file) {
		if file.targetPresent {
			writeAT(b, number, "Verify `%s` remains byte-identical with SHA-256 `%s` for every marker-only choice.", path, file.targetHash)
		} else {
			writeAT(b, number, "Verify `%s` remains absent for every marker-only choice.", path)
		}
		writeAT(b, number, "Verify `%s` occurs exactly once in `govna/preserve.txt` when its marker-only action is convert.", path)
		if len(file.PreserveEntries) > 0 {
			writeAT(b, number, "Verify `%s` remains in `govna/preserve.txt` when its marker-only action is remove.", path)
		} else {
			writeAT(b, number, "Verify `%s` remains absent from `govna/preserve.txt` when its marker-only action is remove.", path)
		}
		writeLegacyOrderingAT(b, number, path)
		writeLegacyPreservationATs(b, number, path, phrases)
		writeAT(b, number, "Verify every exact legacy phrase in %s is absent from the Unreleased CHANGELOG Summary after the marker-only choice for `%s`.", phrases, path)
		return
	}

	if file.Classification == "ambiguity" {
		writeAT(b, number, "Verify `%s` matches its applicable rendered canon region when its resolved action is sync.", path)
		writeAT(b, number, "Verify `%s` is absent from `govna/preserve.txt` when its resolved action is sync.", path)
	}
	writeAT(b, number, "Verify `%s` remains present when its resolved action is preserve.", path)
	writeAT(b, number, "Verify `%s` occurs exactly once in `govna/preserve.txt` when its resolved action is preserve.", path)
	writeAT(b, number, "Verify `%s` is absent when its resolved action is delete.", path)
	writeAT(b, number, "Verify `%s` is absent from `govna/preserve.txt` when its resolved action is delete.", path)
	writeAT(b, number, "Verify the Director response names a migration destination for `%s` when its resolved action is migrate.", path)
	writeAT(b, number, "Verify `%s` is absent unless the Director explicitly preserves it when its resolved action is migrate.", path)
	writeAT(b, number, "Verify any canon-backed migration destination for `%s` matches its applicable rendered canon region.", path)
	writeAT(b, number, "Verify any repository-owned migration destination for `%s` matches the Director-stated result.", path)
	writeAT(b, number, "Verify `%s` is absent from `govna/preserve.txt` when its resolved action is a canon-backed migration.", path)
	if len(file.LegacyPreserveMarkers) > 0 {
		writeLegacyOrderingAT(b, number, path)
		writeLegacyPreservationATs(b, number, path, phrases)
		writeAT(b, number, "Verify every exact legacy phrase in %s is absent from the Unreleased CHANGELOG Summary after the resolved action for `%s`.", phrases, path)
	}
}

func buildAC(report Report, path string, validation validationOutcome) string {
	base := filepath.Base(path)
	number := ""
	if m := regexp.MustCompile(`^ac([0-9]+)-`).FindStringSubmatch(base); m != nil {
		number = m[1]
	}
	var sync, migrate, preserve, review []FileResult
	for _, f := range report.Files {
		if f.forceSync {
			sync = append(sync, f)
			continue
		}
		switch f.Classification {
		case "clear-sync", "missing-in-target":
			sync = append(sync, f)
		case "migration-required":
			migrate = append(migrate, f)
		case "preserve", "expected-divergence":
			preserve = append(preserve, f)
		case "ambiguity", "target-has-no-canon":
			review = append(review, f)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# AC%s Review Govna File Updates\n\n## Summary\n\n", number)
	fmt.Fprintf(&b, "Govna found %s, %s, %s, and %s.\n\n", countPhrase(len(sync), "file ready to update", "files ready to update"), countPhrase(len(migrate), "required control file to add", "required control files to add"), countPhrase(len(review), "file needing a Director decision", "files needing a Director decision"), countPhrase(len(preserve), "file that will stay unchanged", "files that will stay unchanged"))
	fmt.Fprintf(&b, "This AC updates `%s` to Govna's embedded governance files (canon v%s). The result label (classification) beside each path explains why Govna can update it, must leave it unchanged, or needs a Director choice. Installing the selected updates is the adoption step.\n\n## In Scope\n\n", report.Header.RepoName, canon.Version)
	writeGroup := func(title string, files []FileResult) {
		fmt.Fprintf(&b, "### %s\n\n", title)
		if len(files) == 0 {
			b.WriteString("- None.\n\n")
			return
		}
		for _, f := range files {
			key := tallyKey(f)
			meaning := classificationMeaning(key)
			if markerOnly(f) {
				meaning = "the Director must choose whether to convert the exact legacy phrase or remove only that phrase"
			}
			fmt.Fprintf(&b, "- `%s` — `%s`: %s.\n", f.Path, key, meaning)
		}
		b.WriteString("\n")
	}
	writeGroup("Files ready to update", sync)
	writeGroup("Required control files", migrate)
	writeGroup("Files needing a Director choice", review)
	b.WriteString("### Adoption Instructions\n\n- Resolve every Director choice in chat.\n- Leave this generated AC unchanged.\n- Create a temporary copy of the embedded Govna files with `govna render`.\n")
	if report.Header.Flavor == "code" {
		b.WriteString("- Confirm each file selected for update exists in the selected CODE render.\n")
	}
	b.WriteString("- Apply each Director choice only to its authorized file region.\n- Write `govna/canon-baseline.txt` as the final file update.\n")
	for _, f := range sync {
		if len(f.LegacyPreserveMarkers) == 0 {
			continue
		}
		phrases := quotedList(f.LegacyPreserveMarkers)
		fmt.Fprintf(&b, "- Verify every applicable direct-update AT for `%s` before legacy-phrase cleanup.\n", f.Path)
		fmt.Fprintf(&b, "- Remove %s from `CHANGELOG.md` after resolved-state verification for `%s`.\n", phrases, f.Path)
	}
	for _, f := range review {
		if replacement := replacementMissingPath(f); replacement != "" {
			fmt.Fprintf(&b, "- Install `%s` before retired-source routing for `%s`.\n", replacement, f.Path)
		}
		if len(f.LegacyPreserveMarkers) == 0 {
			continue
		}
		phrases := quotedList(f.LegacyPreserveMarkers)
		if markerOnly(f) {
			fmt.Fprintf(&b, "- Convert %s into `govna/preserve.txt` for a conversion choice on `%s`.\n", phrases, f.Path)
			fmt.Fprintf(&b, "- Verify every applicable marker-only AT for `%s` before legacy-phrase cleanup.\n", f.Path)
			fmt.Fprintf(&b, "- Remove %s from `CHANGELOG.md` after marker-only verification for `%s`.\n", phrases, f.Path)
			continue
		}
		fmt.Fprintf(&b, "- Convert %s into `govna/preserve.txt` for a preserve choice on `%s`.\n", phrases, f.Path)
		fmt.Fprintf(&b, "- Verify every applicable resolved-route AT for `%s` before legacy-phrase cleanup.\n", f.Path)
		fmt.Fprintf(&b, "- Remove %s from `CHANGELOG.md` after resolved-state verification for `%s`.\n", phrases, f.Path)
	}
	b.WriteString("\n")
	if len(review) > 0 || validation.kind == validationUnresolved {
		b.WriteString("### Routing Decisions\n\n")
		for i, f := range review {
			switch {
			case markerOnly(f):
				fmt.Fprintf(&b, "%d. **`%s`**: Which action should Govna record for %s: convert the exact legacy phrase into `govna/preserve.txt`, or remove only the phrase?\n", i+1, f.Path, quotedList(f.LegacyPreserveMarkers))
			case replacementMissingPath(f) != "":
				fmt.Fprintf(&b, "%d. **`%s`**: Which action should Govna record after installing `%s`: keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?\n", i+1, f.Path, replacementMissingPath(f))
			case f.Classification == "target-has-no-canon":
				fmt.Fprintf(&b, "%d. **`%s`**: Which action should Govna record: keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?\n", i+1, f.Path)
			default:
				fmt.Fprintf(&b, "%d. **`%s`**: Which action should Govna record: update (sync), keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?\n", i+1, f.Path)
			}
		}
		if validation.kind == validationUnresolved {
			fmt.Fprintf(&b, "%d. **Repository check**: Which command should run after the selected file updates, or what repository evidence shows that no command applies?\n", len(review)+1)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Out Of Scope\n\n")
	if len(preserve) == 0 {
		b.WriteString("- No preserved or expected-divergence entries.\n\n")
	} else {
		for _, f := range preserve {
			fmt.Fprintf(&b, "- `%s` — `%s`: %s.\n", f.Path, f.Classification, classificationMeaning(f.Classification))
		}
		b.WriteString("\n")
	}
	b.WriteString("## Migration findings\n\n")
	if len(migrate) == 0 {
		b.WriteString("- None.\n\n")
	} else {
		for _, f := range migrate {
			switch f.Path {
			case baselinePath:
				b.WriteString("- Write `govna/canon-baseline.txt` from the final temporary render only after all other work is complete and the repository check succeeds.\n")
			case "govna/metadata.txt":
				b.WriteString("- Create `govna/metadata.txt` from the selected temporary render before `govna/canon-baseline.txt` installation.\n")
			default:
				fmt.Fprintf(&b, "- Create `%s` from the selected temporary render.\n", f.Path)
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("## Acceptance Tests\n\n**AT1** [Automated] [Pre-release gate] — Verify every file selected for update except `govna/canon-baseline.txt` against the rendered Govna files and every preserved file against `govna/preserve.txt`.\n\n")
	at := 2
	for _, f := range append(sync, review...) {
		if f.protectedHash != "" {
			fmt.Fprintf(&b, "**AT%d** [Automated] [Pre-release gate] — Preserve the protected region in `%s` from `%s` through EOF with SHA-256 `%s` for any update choice.\n\n", at, f.Path, f.Boundary, f.protectedHash)
			at++
		}
	}
	for _, f := range sync {
		if len(f.LegacyPreserveMarkers) == 0 {
			continue
		}
		writeAT(&b, &at, "Verify `%s` matches its applicable rendered canon region when its resolved action is sync.", f.Path)
		writeAT(&b, &at, "Verify `%s` is absent from `govna/preserve.txt` when its resolved action is sync.", f.Path)
		writeLegacyOrderingAT(&b, &at, f.Path)
		writeLegacyPreservationATs(&b, &at, f.Path, quotedList(f.LegacyPreserveMarkers))
		writeAT(&b, &at, "Verify every exact legacy phrase in %s is absent from the Unreleased CHANGELOG Summary after the resolved action for `%s`.", quotedList(f.LegacyPreserveMarkers), f.Path)
	}
	for _, f := range review {
		writeRouteATs(&b, &at, f)
	}
	if validation.kind == validationUnresolved {
		fmt.Fprintf(&b, "**AT%d** [Manual] [Pre-release gate] — Choose the repository check in chat.\n\n", at)
		at++
		fmt.Fprintf(&b, "**AT%d** [Automated] [Pre-release gate] — Verify the chosen repository check succeeds before `govna/canon-baseline.txt` installation.\n\n", at)
		at++
	} else if validation.kind == validationInferred {
		fmt.Fprintf(&b, "**AT%d** [Automated] [Pre-release gate] — Run `./build.sh` after the selected file updates and before `govna/canon-baseline.txt` installation (selected from exact AGENTS.md declarations).\n\n", at)
		at++
	} else {
		reason := validation.reason
		if reason == "" {
			reason = "no reason recorded"
		}
		fmt.Fprintf(&b, "**AT%d** [Automated] [Pre-release gate] — Verify the `Not applicable` evidence still holds after the selected file updates and before `govna/canon-baseline.txt` installation (%s).\n\n", at, reason)
		at++
	}
	fmt.Fprintf(&b, "**AT%d** [Automated] [Pre-release gate] — Verify the final file update installed `govna/canon-baseline.txt` from the same temporary render.\n\n## Status\n\n`PENDING` — audit emission; awaiting explicit Director Audit.\n", at)
	return b.String()
}

func countPhrase(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func validationDisposition(root string, report Report) validationOutcome {
	baselineUpdate := false
	for _, file := range report.Files {
		if file.Path == baselinePath && (file.Classification == "migration-required" || file.Classification == "clear-sync") {
			baselineUpdate = true
		}
	}
	if !baselineUpdate {
		return validationOutcome{kind: validationNotApplicable, evidence: "`Not applicable` because no baseline migration is present", reason: "no baseline migration is present"}
	}
	agents, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	first := regexp.MustCompile(`(?m)^- Run `+"`"+`([^`+"`"+`\n]+)`+"`"+` as the first validation command(?:\s|\.|$)[^\n]*$`).FindAllSubmatch(agents, -1)
	wide := regexp.MustCompile(`(?m)^- Use `+"`"+`([^`+"`"+`\n]+)`+"`"+` for repository-wide [^\n]*validation[^\n]*$`).FindAllSubmatch(agents, -1)
	if report.Header.Flavor == "code" && len(first) == 1 && len(wide) == 1 && string(first[0][1]) == "./build.sh" && string(wide[0][1]) == "./build.sh" {
		if info, err := os.Stat(filepath.Join(root, "build.sh")); err == nil && info.Mode().IsRegular() {
			if stackManifestReachable(root, report.selectedStack) {
				return validationOutcome{kind: validationInferred, evidence: "`./build.sh` inferred from exact AGENTS.md declarations"}
			}
		}
	}
	if report.Header.Flavor == "doc" && len(first) == 0 && len(wide) == 0 {
		release, _ := os.ReadFile(filepath.Join(root, "govna", "release.md"))
		const declaration = "DOC repositories do not need a compiler toolchain for release preparation or release orchestration and define no automated content-validation command."
		if bytes.Contains(release, []byte(declaration)) {
			return validationOutcome{kind: validationNotApplicable, evidence: "`Not applicable` inferred from exact DOC governance evidence", reason: "inferred from exact DOC governance evidence"}
		}
	}
	return validationOutcome{kind: validationUnresolved}
}

func stackManifestReachable(root, stack string) bool {
	manifests := map[string][]string{
		"Go":     {"go.mod"},
		"Rust":   {"Cargo.toml"},
		"Swift":  {"Package.swift"},
		"Node":   {"package.json"},
		"Python": {"pyproject.toml"},
		"Java":   {"pom.xml", "build.gradle"},
	}
	for _, manifest := range manifests[stack] {
		if regularFile(filepath.Join(root, manifest)) {
			return true
		}
	}
	if stack != "Terraform" {
		return false
	}
	if regularFile(filepath.Join(root, ".terraform.lock.hcl")) {
		return true
	}
	matches, err := filepath.Glob(filepath.Join(root, "*.tf"))
	if err != nil {
		return false
	}
	return slices.ContainsFunc(matches, regularFile)
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
