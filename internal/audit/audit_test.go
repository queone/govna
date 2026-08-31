package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/emission"
	rendercmd "github.com/queone/govna/internal/render"
)

const testProgramVersion = "9.8.7"

func inferredBuildOutcome() validationOutcome {
	return validationOutcome{kind: validationInferred, evidence: "`./build.sh` inferred from exact AGENTS.md declarations"}
}

func notApplicableOutcome(evidence, reason string) validationOutcome {
	return validationOutcome{kind: validationNotApplicable, evidence: evidence, reason: reason}
}

func fixture(t *testing.T) string {
	return fixtureFor(t, canon.Code, "Go")
}

func fixtureFor(t *testing.T, flavor canon.Flavor, stack string) string {
	t.Helper()
	root := t.TempDir()
	repoName := "widget"
	modulePath := ""
	if flavor == canon.Doc {
		repoName = "handbook"
	} else if stack == "Go" {
		modulePath = "example.com/widget"
	}
	files, err := canon.Render(canon.Config{Flavor: flavor, RepoName: repoName, Stack: stack, ModulePath: modulePath})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, file.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if flavor == canon.Code {
		manifestPath, manifest := auditProfileManifest(stack)
		if err := os.WriteFile(filepath.Join(root, manifestPath), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	git(t, root, "config", "user.name", "Fixture")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "govna apply")
	return root
}

func auditProfileManifest(stack string) (string, string) {
	switch stack {
	case "Go":
		return "go.mod", "module example.com/widget\n\ngo 1.27.0\n"
	case "Rust":
		return "Cargo.toml", "[package]\nname = \"widget\"\nversion = \"0.1.0\"\n"
	case "Swift":
		return "Package.swift", "// swift-tools-version: 6.0\n"
	case "Terraform":
		return ".terraform.lock.hcl", "# fixture\n"
	default:
		panic("unsupported audit fixture stack: " + stack)
	}
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	args = append([]string{"-C", root}, args...)
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestAuditCleanAndJSON(t *testing.T) {
	root := fixture(t)
	var stdout, stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	want, err := os.ReadFile("testdata/clean-golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != string(want) {
		t.Fatalf("clean output=%q want=%q", stdout.String(), want)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"--json"}, &stdout, &stderr, root, testProgramVersion); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Header.CanonSHA != "v0.49.0" || report.Emitted != nil || strings.Contains(stdout.String(), "no AC emitted") {
		t.Fatalf("bad JSON: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "canon_reference") || strings.Contains(stdout.String(), "prior_commits") || !strings.Contains(stdout.String(), `"canon_ref"`) || !strings.Contains(stdout.String(), `"compare_command"`) {
		t.Fatalf("incorrect JSON schema: %s", stdout.String())
	}
}

func TestAuditActionableEmissionAndGuard(t *testing.T) {
	root := fixture(t)
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("consumer edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--repo-name", "widget"}, &stdout, &stderr, root, testProgramVersion); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "Wrote govna/ac1-audit-v0.49.0.md for review") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	stub := filepath.Join(root, "govna", "ac1-audit-v0.49.0.md")
	body, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "## Summary") || !strings.Contains(string(body), "`README.md` — `ambiguity`") {
		t.Fatalf("bad stub: %s", body)
	}
	markerPrefix := "<!-- audit: emitted-by govna executable v9.8.7 with embedded canon v0.49.0 sha256:"
	if !strings.HasPrefix(string(body), markerPrefix) {
		t.Fatalf("bad marker: %s", body)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 {
		t.Fatalf("idempotent code=%d stderr=%q", code, stderr.String())
	}
	rerunBody, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, rerunBody) {
		t.Fatal("unchanged audit rerun rewrote body")
	}
	_, rawBody, ok := strings.Cut(string(body), "\n")
	if !ok {
		t.Fatal("guarded body has no marker line")
	}
	legacyBody := strings.Replace(
		rawBody,
		"Verify the final file update installed `govna/canon-baseline.txt` from the same temporary render.",
		"Install and verify `govna/canon-baseline.txt` from the same scratch render as the final adoption step.",
		1,
	)
	if legacyBody == rawBody {
		t.Fatal("failed to construct legacy audit body")
	}
	legacy := legacyGuardedBody(emission.AuditMarkerPrefix, "v"+canon.Version, []byte(legacyBody))
	if err := os.WriteFile(stub, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 {
		t.Fatalf("legacy upgrade code=%d stderr=%q", code, stderr.String())
	}
	upgraded, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(upgraded), markerPrefix) || bytes.Equal(upgraded, legacy) || strings.Contains(string(upgraded), "Install and verify") {
		t.Fatalf("legacy marker not upgraded: %s", upgraded)
	}
	matches, err := filepath.Glob(filepath.Join(root, "govna", "ac*-audit-v0.49.0.md"))
	if err != nil || len(matches) != 1 || matches[0] != stub {
		t.Fatalf("same-canon upgrade changed stub identity: matches=%v err=%v", matches, err)
	}
	if err := os.WriteFile(stub, append(upgraded, []byte("edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 1 || !strings.Contains(stderr.String(), "has been edited") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestAuditStateAndClassification(t *testing.T) {
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\nREADME.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	preserve, err := ParsePreserve(root)
	if err != nil || !preserve["README.md"] {
		t.Fatalf("preserve=%v err=%v", preserve, err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\n../bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePreserve(root); err == nil {
		t.Fatal("malformed preserve accepted")
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	base, err := parseBaseline(data, canon.Code)
	if err != nil || len(base.Entries) == 0 {
		t.Fatalf("baseline=%v err=%v", base, err)
	}
	if _, err := parseBaseline([]byte("bad\n"), canon.Code); err == nil {
		t.Fatal("malformed baseline accepted")
	}
}

func TestAuditFormatForcingAndCoherence(t *testing.T) {
	root := fixture(t)
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("consumer edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\nAGENTS.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, clean, err := inspect(Config{DiffLines: 200}, root)
	if err != nil || clean {
		t.Fatalf("clean=%v err=%v", clean, err)
	}
	for _, file := range report.Files {
		if file.Path == "AGENTS.md" && (file.Classification != "preserve" || !file.forceSync || file.EffectiveClassification != "force-sync") {
			t.Fatalf("format classification=%s effective=%s", file.Classification, file.EffectiveClassification)
		}
	}
	var encoded bytes.Buffer
	if err := json.NewEncoder(&encoded).Encode(report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), `"classification":"preserve"`) || !strings.Contains(encoded.String(), `"effective_classification":"force-sync"`) {
		t.Fatalf("JSON output omits real Classification or EffectiveClassification: %s", encoded.String())
	}
	body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
	if !strings.Contains(body, "### Files ready to update\n\n- `AGENTS.md` — `force-sync`: this file's governed structure always syncs to canon, regardless of local edits or the preserve list.") {
		t.Fatalf("force-synced preserve file did not render the force-sync explanation: %s", body)
	}
	if strings.Contains(body, "`preserve`: the preserve list says to keep the repository's version") {
		t.Fatalf("force-synced preserve file still contradicts its own placement: %s", body)
	}
}

func TestFlavorReferenceCoherence(t *testing.T) {
	profiles := []struct {
		name, flavorTarget, absentTarget, stack string
		flavor                                  canon.Flavor
	}{{name: "DOC", flavor: canon.Doc, flavorTarget: "govna/release.md", absentTarget: "govna/build-release.md"}}
	for _, stack := range canon.Stacks() {
		profiles = append(profiles, struct {
			name, flavorTarget, absentTarget, stack string
			flavor                                  canon.Flavor
		}{name: "CODE/" + stack, flavor: canon.Code, flavorTarget: "govna/build-release.md", absentTarget: "govna/release.md", stack: stack})
	}
	for _, tc := range profiles {
		t.Run(tc.name, func(t *testing.T) {
			files, err := canon.Render(canon.Config{Flavor: tc.flavor, RepoName: "widget", Stack: tc.stack, ModulePath: "example.com/widget"})
			if err != nil {
				t.Fatal(err)
			}
			rendered := map[string][]byte{}
			for _, file := range files {
				rendered[file.Path] = file.Content
			}
			if err := checkCoherence(rendered); err != nil {
				t.Fatalf("coherent %s render rejected: %v", tc.name, err)
			}
			roles := string(rendered["govna/roles.md"])
			if !strings.Contains(roles, tc.flavorTarget) || strings.Contains(roles, tc.absentTarget) {
				t.Fatalf("%s roles are not flavor-safe: %s", tc.name, roles)
			}
			delete(rendered, tc.flavorTarget)
			if err := checkCoherence(rendered); err == nil || !strings.Contains(err.Error(), tc.flavorTarget) || !strings.Contains(err.Error(), "report this to the Govna maintainer") {
				t.Fatalf("missing %s coherence err=%v", tc.flavorTarget, err)
			}
		})
	}
}

func TestRenderedContractCoherenceRejectsConsumerVisibleMutations(t *testing.T) {
	files, err := canon.Render(canon.Config{Flavor: canon.Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	if err != nil {
		t.Fatal(err)
	}
	original := map[string][]byte{}
	for _, file := range files {
		original[file.Path] = file.Content
	}
	if err := checkCoherence(original); err != nil {
		t.Fatalf("unmodified CODE/Go render rejected: %v", err)
	}

	tests := []struct {
		name, path, old, replacement, want string
	}{
		{
			name:        "missing release reference",
			path:        "govna/roles.md",
			old:         "govna/build-release.md",
			replacement: "govna/release.md",
			want:        "must reference govna/build-release.md",
		},
		{
			name:        "command-only Package report",
			path:        "govna/build-release.md",
			old:         "End the structured Package completion report with `Run below to release:`.",
			replacement: "Present only the release command after prep.",
			want:        "must contain exactly one",
		},
		{
			name:        "integrated audit cannot enter Refine",
			path:        "AGENTS.md",
			old:         "Exempt integrated audit adoption and an eligible bounded completeness correction from a fresh Refine action instruction.",
			replacement: "Exempt only an eligible bounded completeness correction from a fresh Refine action instruction.",
			want:        "must contain exactly one",
		},
		{
			name:        "scratch review lacks original authorization",
			path:        "AGENTS.md",
			old:         "Authorize the bounded scratch-review procedure in `govna/audit.md` from the original explicit `govna audit` request.",
			replacement: "Require separate scratch-review authorization after audit emission.",
			want:        "Authorize the bounded scratch-review procedure",
		},
		{
			name:        "scratch renderer identity is unbound",
			path:        "govna/audit.md",
			old:         "Require the emitted AC marker versions to match the recorded detailed version.",
			replacement: "Allow any available Govna executable to render the review files.",
			want:        "Require the emitted AC marker versions",
		},
		{
			name:        "scratch cleanup is optional",
			path:        "AGENTS.md",
			old:         "Remove the exact scratch directory before reporting Audit completion or a blocker.",
			replacement: "Keep the scratch directory for later review.",
			want:        "Remove the exact scratch directory",
		},
		{
			name:        "unfitted AC accumulation is allowed",
			path:        "AGENTS.md",
			old:         "Start another Implement only when the projected complete pending release batch can fit one compliant release message.",
			replacement: "Start another Implement without checking the pending release batch.",
			want:        "Start another Implement only when",
		},
		{
			name:        "partial Package is allowed",
			path:        "govna/build-release.md",
			old:         "Prohibit a smaller release batch while excluded implemented work remains.",
			replacement: "Allow a smaller release batch while excluded implemented work remains.",
			want:        "Prohibit a smaller release batch",
		},
		{
			name: "combined Audit cycle command",
			path: "govna/development-cycle.md",
			old: "2. **Audit.**\n   - Review the AC for missing scope, unsafe assumptions, and untestable requirements without editing it.\n" +
				"   - Start this review immediately when an explicit agent-mediated `govna audit` request emits or reuses one guarded adoption AC.",
			replacement: "2. **Audit.** Review the AC and start this review immediately after audit emission.",
			want:        "2. **Audit.**",
		},
		{
			name: "combined Refine cycle command",
			path: "govna/development-cycle.md",
			old: "3. **Refine.**\n   - Update a hand-authored AC with settled findings and Director decisions.\n" +
				"   - Keep an audit-emitted AC unchanged.\n   - Record its resolved decisions in the active session.",
			replacement: "3. **Refine.** Update the AC and record its decisions in the active session.",
			want:        "3. **Refine.**",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutated := make(map[string][]byte, len(original))
			for path, content := range original {
				mutated[path] = bytes.Clone(content)
			}
			before := string(mutated[tc.path])
			after := strings.Replace(before, tc.old, tc.replacement, 1)
			if after == before {
				t.Fatalf("mutation source missing from %s: %s", tc.path, tc.old)
			}
			mutated[tc.path] = []byte(after)
			err := checkCoherence(mutated)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "report this to the Govna maintainer") {
				t.Fatalf("coherence error=%v, want %q and maintainer recovery", err, tc.want)
			}
		})
	}
}

func TestConsumerEquivalentAuditProfiles(t *testing.T) {
	profiles := []struct {
		name, stack string
		flavor      canon.Flavor
	}{{name: "DOC", flavor: canon.Doc}}
	for _, stack := range canon.Stacks() {
		profiles = append(profiles, struct {
			name, stack string
			flavor      canon.Flavor
		}{name: "CODE/" + stack, flavor: canon.Code, stack: stack})
	}

	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			root := fixtureFor(t, profile.flavor, profile.stack)
			metadata := filepath.Join(root, "govna", "metadata.txt")
			replaceFileText(t, metadata, "canon_version = v"+canon.Version+"\n", "canon_version = v0.44.0\n")

			args := []string{"--json", "--flavor", strings.ToLower(string(profile.flavor))}
			if profile.flavor == canon.Code {
				args = append(args, "--stack", profile.stack)
			}
			report, body := runAuditFixture(t, root, args)
			if report.Emitted == nil {
				t.Fatal("stale profile emitted no adoption AC")
			}
			for _, required := range []string{
				"# AC1 Adopt Govna Governance Files v" + canon.Version,
				"This AC updates",
				"### Adoption Instructions",
				"`PENDING` — immutable audit emission; workflow state is tracked in the active session.",
			} {
				if !strings.Contains(body, required) {
					t.Errorf("emitted AC omits %q", required)
				}
			}
			if profile.flavor == canon.Doc && strings.Contains(body, "selected CODE render") {
				t.Error("DOC emitted AC contains CODE-only reachability instruction")
			}
			if profile.flavor == canon.Code && !strings.Contains(body, "selected CODE render") {
				t.Error("CODE emitted AC omits rendered-file reachability instruction")
			}

			if strings.Contains(body, "**Repository check**") {
				t.Error("supported profile with bounded repository evidence retained an unresolved repository check")
			}

			firstPath := report.Emitted.ACStub
			second, secondBody := runAuditFixture(t, root, args)
			if second.Emitted == nil || second.Emitted.ACStub != firstPath || secondBody != body {
				t.Fatal("identical consumer audit did not reuse one guarded adoption AC")
			}
			matches, err := filepath.Glob(filepath.Join(root, "govna", "ac*-audit-v"+canon.Version+".md"))
			if err != nil || len(matches) != 1 {
				t.Fatalf("guarded adoption AC count=%d err=%v", len(matches), err)
			}
		})
	}
}

func TestOrdinaryAuditScratchReviewAcrossProfiles(t *testing.T) {
	mixedPaths := []string{
		"AGENTS.md",
		"govna/build-release.md",
		"govna/development-guidelines.md",
		"govna/editing-guidelines.md",
	}
	coveredMixedPath := map[string]bool{}
	profiles := []struct {
		name, repoName, stack string
		flavor                canon.Flavor
	}{{name: "DOC", repoName: "handbook", flavor: canon.Doc}}
	for _, stack := range canon.Stacks() {
		profiles = append(profiles, struct {
			name, repoName, stack string
			flavor                canon.Flavor
		}{name: "CODE/" + stack, repoName: "widget", flavor: canon.Code, stack: stack})
	}

	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			root := fixtureFor(t, profile.flavor, profile.stack)
			for _, path := range mixedPaths {
				if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
					addMixedReviewSentinels(t, root, path)
					coveredMixedPath[path] = true
				} else if !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
			replaceFileText(t, filepath.Join(root, "govna", "metadata.txt"), "canon_version = v"+canon.Version+"\n", "canon_version = v0.44.0\n")

			args := []string{"--repo-name", profile.repoName, "--flavor", strings.ToLower(string(profile.flavor))}
			if profile.flavor == canon.Code {
				args = append(args, "--stack", profile.stack)
			}
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr, root, testProgramVersion); code != 0 || stderr.Len() != 0 {
				t.Fatalf("ordinary audit code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if !strings.HasPrefix(stdout.String(), "Wrote govna/ac1-audit-v"+canon.Version+".md for review") || strings.HasPrefix(strings.TrimSpace(stdout.String()), "{") {
				t.Fatalf("ordinary audit output=%q", stdout.String())
			}

			acPath := filepath.Join(root, "govna", "ac1-audit-v"+canon.Version+".md")
			acBefore, err := os.ReadFile(acPath)
			if err != nil {
				t.Fatal(err)
			}
			if !emission.VerifyAuditBody(acBefore) || !strings.Contains(string(acBefore), "### Audit Review\n") {
				t.Fatalf("ordinary audit emitted an invalid review AC: %s", acBefore)
			}
			for _, forbidden := range []string{"```diff", "companion review artifact", "<scratch>/diff"} {
				if strings.Contains(string(acBefore), forbidden) {
					t.Errorf("emitted review AC contains stored review material %q", forbidden)
				}
			}

			report, clean, err := inspect(Config{Flavor: strings.ToLower(string(profile.flavor)), Stack: profile.stack, RepoName: profile.repoName, DiffLines: 200, invocation: "govna audit"}, root)
			if err != nil || clean {
				t.Fatalf("post-emission inspection clean=%v err=%v", clean, err)
			}
			consumerBefore := snapshotConsumerTree(t, root)
			scratch, err := os.MkdirTemp("", "govna-audit-review-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(scratch)
			rel, err := filepath.Rel(root, scratch)
			if err != nil || rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Fatalf("scratch directory is not outside consumer: root=%s scratch=%s rel=%s err=%v", root, scratch, rel, err)
			}

			renderArgs := []string{"--flavor", strings.ToLower(string(profile.flavor))}
			if profile.flavor == canon.Code {
				renderArgs = append(renderArgs, "--stack", profile.stack)
				if profile.stack == "Go" {
					renderArgs = append(renderArgs, "--module-path", "example.com/widget")
				}
			}
			renderArgs = append(renderArgs, scratch)
			var renderOut, renderErr bytes.Buffer
			if code := rendercmd.Run(renderArgs, &renderOut, &renderErr, root); code != 0 || renderErr.Len() != 0 {
				t.Fatalf("scratch render code=%d stdout=%q stderr=%q", code, renderOut.String(), renderErr.String())
			}
			baseline, err := os.ReadFile(filepath.Join(scratch, "govna", "canon-baseline.txt"))
			if err != nil {
				t.Fatal(err)
			}
			detailedVersion := []byte(fmt.Sprintf("Govna executable version: v%s\nEmbedded governance-file version (canon version): v%s\n", testProgramVersion, canon.Version))
			if !scratchReviewIdentityMatches(acBefore, detailedVersion, baseline) {
				t.Fatal("matching executable, marker, and rendered baseline identity was rejected")
			}

			reviewed := 0
			for _, file := range report.Files {
				if !auditReviewActionable(file) {
					continue
				}
				reviewed++
				assertAuditReviewPath(t, string(acBefore), root, scratch, file)
			}
			if reviewed == 0 {
				t.Fatal("ordinary audit fixture exposed no actionable review paths")
			}

			if err := os.RemoveAll(scratch); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(scratch); !os.IsNotExist(err) {
				t.Fatalf("scratch directory remains after review: %v", err)
			}
			assertConsumerTree(t, root, consumerBefore)
			acAfter, err := os.ReadFile(acPath)
			if err != nil || !bytes.Equal(acAfter, acBefore) {
				t.Fatalf("scratch review changed immutable AC: err=%v", err)
			}

			stdout.Reset()
			stderr.Reset()
			if code := Run(args, &stdout, &stderr, root, testProgramVersion); code != 0 || stderr.Len() != 0 {
				t.Fatalf("ordinary audit reuse code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			reused, err := os.ReadFile(acPath)
			if err != nil || !bytes.Equal(reused, acBefore) {
				t.Fatalf("ordinary audit did not reuse the immutable AC: err=%v", err)
			}
		})
	}
	for _, path := range mixedPaths {
		if !coveredMixedPath[path] {
			t.Errorf("ordinary audit profiles omitted registered mixed path %s", path)
		}
	}
}

func TestAuditReviewCanonZoneCommand(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, "left.md")
	right := filepath.Join(root, "right.md")
	boundary := "## Project Rules"

	t.Run("boundary line endings do not enter comparison", func(t *testing.T) {
		if err := os.WriteFile(left, []byte("same canon\n"+boundary+"\nleft tail\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(right, []byte("same canon\n"+boundary+"\r\nright tail\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		output, err := exec.Command("bash", "-c", auditReviewCanonZoneCommand(boundary, left, right)).CombinedOutput()
		if err != nil || len(output) != 0 {
			t.Fatalf("boundary-only line-ending difference entered canon comparison: err=%v output=%s", err, output)
		}
	})

	t.Run("canon bytes remain exact", func(t *testing.T) {
		if err := os.WriteFile(left, []byte("left canon\r\n"+boundary+"\nleft tail sentinel\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(right, []byte("right canon\n"+boundary+"\r\nright tail sentinel\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		output, err := exec.Command("bash", "-c", auditReviewCanonZoneCommand(boundary, left, right)).CombinedOutput()
		exitErr, differs := err.(*exec.ExitError)
		if !differs || exitErr.ExitCode() != 1 {
			t.Fatalf("canon difference status=%v output=%s", err, output)
		}
		text := string(output)
		for _, want := range []string{"left canon", "right canon"} {
			if !strings.Contains(text, want) {
				t.Errorf("canon comparison omits %q: %s", want, text)
			}
		}
		for _, forbidden := range []string{boundary, "left tail sentinel", "right tail sentinel"} {
			if strings.Contains(text, forbidden) {
				t.Errorf("canon comparison includes excluded %q: %s", forbidden, text)
			}
		}
	})
}

func TestAuditReviewMixedBoundaryBranches(t *testing.T) {
	for _, tc := range []struct {
		name, boundary string
		wantScoped     bool
	}{
		{name: "exact boundary", boundary: "## Project Rules", wantScoped: true},
		{name: "missing boundary"},
		{name: "altered boundary", boundary: "## Local Rules"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := FileResult{Path: "AGENTS.md", Classification: "ambiguity", Boundary: tc.boundary, canonPresent: true, targetInspected: true, targetPresent: true}
			var body strings.Builder
			writeAuditReview(&body, []FileResult{file})
			got := body.String()
			hasScoped := strings.Contains(got, "Compare the canon zone of `AGENTS.md` above exact boundary `## Project Rules`")
			if hasScoped != tc.wantScoped {
				t.Errorf("scoped comparison=%t want=%t: %s", hasScoped, tc.wantScoped, got)
			}
			hasWhole := strings.Contains(got, "Compare `AGENTS.md` with `diff -ru <scratch>/AGENTS.md AGENTS.md`.")
			if hasWhole == tc.wantScoped {
				t.Errorf("whole comparison=%t scoped=%t: %s", hasWhole, tc.wantScoped, got)
			}
		})
	}
}

func TestScratchReviewIdentityRejectsMismatches(t *testing.T) {
	ac := emission.AuditBody(testProgramVersion, canon.Version, []byte("# AC1 Example\n"))
	detailed := []byte(fmt.Sprintf("Govna executable version: v%s\nEmbedded governance-file version (canon version): v%s\n", testProgramVersion, canon.Version))
	baseline := []byte("govna-canon-baseline-v1\ncanon_version = v" + canon.Version + "\n")
	if !scratchReviewIdentityMatches(ac, detailed, baseline) {
		t.Fatal("matching review identity rejected")
	}
	for _, tc := range []struct {
		name                 string
		ac, detail, baseline []byte
	}{
		{name: "resolved executable", ac: ac, detail: bytes.Replace(detailed, []byte("v"+testProgramVersion), []byte("v0.0.0"), 1), baseline: baseline},
		{name: "embedded canon", ac: ac, detail: bytes.Replace(detailed, []byte("v"+canon.Version), []byte("v0.0.0"), 1), baseline: baseline},
		{name: "rendered baseline", ac: ac, detail: detailed, baseline: bytes.Replace(baseline, []byte("v"+canon.Version), []byte("v0.0.0"), 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if scratchReviewIdentityMatches(tc.ac, tc.detail, tc.baseline) {
				t.Fatal("mismatched review identity accepted")
			}
		})
	}
}

func TestAuditRejectsUnsupportedCODEStacks(t *testing.T) {
	for _, stack := range []string{"Java", "Node", "Python"} {
		t.Run(stack+" metadata", func(t *testing.T) {
			root := fixture(t)
			replaceFileText(t, filepath.Join(root, "govna", "metadata.txt"), "code_stack = Go\n", "code_stack = "+stack+"\n")
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"--json"}, &stdout, &stderr, root, testProgramVersion); code != 1 || !strings.Contains(stderr.String(), "use Go, Rust, Swift, or Terraform") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
		t.Run(stack+" explicit", func(t *testing.T) {
			root := fixture(t)
			var stdout, stderr bytes.Buffer
			if code := Run([]string{"--json", "--flavor", "code", "--stack", stack}, &stdout, &stderr, root, testProgramVersion); code != 1 || !strings.Contains(stderr.String(), "use Go, Rust, Swift, or Terraform") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestConsumerEquivalentAuditStateMatrix(t *testing.T) {
	root := fixture(t)
	replaceFileText(t, filepath.Join(root, "govna", "metadata.txt"), "canon_version = v"+canon.Version+"\n", "canon_version = v0.44.0\n")
	replaceFileText(t, filepath.Join(root, "README.md"), "Govna makes", "Consumer wording makes")
	replaceFileText(t, filepath.Join(root, "plan.md"), "# widget Plan", "# Consumer Plan")
	replaceFileText(t, filepath.Join(root, "AGENTS.md"), "# AGENTS.md", "# Consumer AGENTS.md")
	if err := os.Remove(filepath.Join(root, "govna", "audit.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "govna", "drift-scan.md"), []byte("retired local guidance\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "govna", "preserve.txt"), []byte("govna-preserve-v1\ngovna/development-cycle.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	replaceFileText(t, filepath.Join(root, "govna", "development-cycle.md"), "# Development Cycle", "# Preserved Development Cycle")
	changelog := filepath.Join(root, "CHANGELOG.md")
	data, err := os.ReadFile(changelog)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, []byte("\n## Unreleased\n\npreserve govna/drift-scan.md\npreserve marker.md\n")...)
	if err := os.WriteFile(changelog, data, 0o644); err != nil {
		t.Fatal(err)
	}

	report, body := runAuditFixture(t, root, []string{"--json", "--flavor", "code", "--stack", "Go"})
	seen := map[string]bool{}
	forceSync := false
	for _, file := range report.Files {
		seen[file.Classification] = true
		forceSync = forceSync || file.EffectiveClassification == "force-sync"
	}
	for _, classification := range []string{"match", "expected-divergence", "preserve", "ambiguity", "clear-sync", "missing-in-target", "target-has-no-canon"} {
		if !seen[classification] {
			t.Errorf("consumer state matrix omits classification %s", classification)
		}
	}
	if !forceSync {
		t.Error("consumer state matrix omits force-sync")
	}
	for _, required := range []string{
		"update (sync)",
		"keep local (preserve)",
		"destination named in the response (migrate)",
		"remove (delete)",
		"convert the exact legacy phrase into `govna/preserve.txt`",
		"remove only the phrase",
		"Install `govna/audit.md` before retired-source routing for `govna/drift-scan.md`.",
		"Preserve the protected region in `AGENTS.md`",
		"Verify every applicable resolved-route AT for `govna/drift-scan.md` before legacy-phrase cleanup.",
		"Verify any repository-owned migration destination for `govna/drift-scan.md` matches the Director-stated result.",
	} {
		if !strings.Contains(body, required) {
			t.Errorf("consumer state AC omits %q", required)
		}
	}

	migrationRoot := fixture(t)
	if err := os.Remove(filepath.Join(migrationRoot, filepath.FromSlash(baselinePath))); err != nil {
		t.Fatal(err)
	}
	migration, migrationBody := runAuditFixture(t, migrationRoot, []string{"--json", "--flavor", "code", "--stack", "Go"})
	foundMigration := false
	for _, file := range migration.Files {
		foundMigration = foundMigration || file.Classification == "migration-required"
	}
	if !foundMigration || !strings.Contains(migrationBody, "### Required control files") {
		t.Fatal("consumer state matrix omits migration-required emission")
	}

	unresolvedRoot := fixture(t)
	replaceFileText(t, filepath.Join(unresolvedRoot, "govna", "metadata.txt"), "canon_version = v"+canon.Version+"\n", "canon_version = v0.44.0\n")
	if err := os.Remove(filepath.Join(unresolvedRoot, "go.mod")); err != nil {
		t.Fatal(err)
	}
	_, unresolvedBody := runAuditFixture(t, unresolvedRoot, []string{"--json", "--flavor", "code", "--stack", "Go"})
	for _, required := range []string{
		"**Repository check**: Which command should run after the selected file updates",
		"Choose the repository check in chat.",
		"Verify the chosen repository check succeeds before `govna/canon-baseline.txt` installation.",
	} {
		if !strings.Contains(unresolvedBody, required) {
			t.Errorf("consumer state unresolved-check AC omits %q", required)
		}
	}
}

func runAuditFixture(t *testing.T, root string, args []string) (Report, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr, root, testProgramVersion); code != 0 || stderr.Len() != 0 {
		t.Fatalf("audit code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode audit report: %v\n%s", err, stdout.String())
	}
	if report.Emitted == nil {
		return report, ""
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(report.Emitted.ACStub)))
	if err != nil {
		t.Fatal(err)
	}
	if !emission.VerifyAuditBody(data) {
		t.Fatal("emitted adoption AC failed its body guard")
	}
	return report, string(data)
}

func TestMissingFormatFileKeepsClassificationAndForcesSync(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "govna", "ac-template.md")); err != nil {
		t.Fatal(err)
	}
	report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
	if err != nil || clean {
		t.Fatalf("clean=%v err=%v", clean, err)
	}
	for _, file := range report.Files {
		if file.Path == "govna/ac-template.md" {
			if file.Classification != "missing-in-target" || !file.forceSync || file.EffectiveClassification != "force-sync" {
				t.Fatalf("file=%+v", file)
			}
			body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
			if !strings.Contains(body, "### Files ready to update\n\n- `govna/ac-template.md` — `force-sync`: this file's governed structure always syncs to canon, regardless of local edits or the preserve list.") {
				t.Fatalf("routing=%s", body)
			}
			return
		}
	}
	t.Fatal("missing format file absent from report")
}

func TestAuditArgumentsAndPreconditions(t *testing.T) {
	for _, args := range [][]string{{"extra"}, {"--diff-lines", "0"}, {"--flavor", "bad"}, {"--stack", ""}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err, t.TempDir(), testProgramVersion); code != 2 {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, err.String())
		}
	}
	root := t.TempDir()
	var out, err bytes.Buffer
	if code := Run(nil, &out, &err, root, testProgramVersion); code != 1 || !strings.Contains(err.String(), "AGENTS.md") {
		t.Fatalf("code=%d stderr=%q", code, err.String())
	}
}

func TestAuditRejectsGovnaSourceCheckout(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/queone/govna\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal", "canon", "assets", "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "canon", "assets", "base", "AGENTS.md.tmpl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd", "govna"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd", "govna", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	if code := Run(nil, &out, &errBuf, root, testProgramVersion); code != 1 {
		t.Fatalf("code=%d stderr=%q", code, errBuf.String())
	}
	want := "audit: an audit AC cannot be created inside the Govna source checkout at " + root + "; run this command from the target repository\n"
	if errBuf.String() != want {
		t.Fatalf("stderr=%q want=%q", errBuf.String(), want)
	}
}

func TestAuditMultipleStubsErrorHasPrefix(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "govna", "ac-template.md")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ac1-audit-v0.49.0.md", "ac2-audit-v0.49.0.md"} {
		if err := os.WriteFile(filepath.Join(root, "govna", name), []byte("stub\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out, errBuf bytes.Buffer
	if code := Run(nil, &out, &errBuf, root, testProgramVersion); code != 1 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errBuf.String())
	}
	if !strings.HasPrefix(errBuf.String(), "audit: ") || !strings.Contains(errBuf.String(), "Govna found more than one generated audit AC") {
		t.Fatalf("stderr=%q", errBuf.String())
	}
}

func TestAuditDiffTruncationAndCrossFlavor(t *testing.T) {
	diff := diffText([]byte("a\nb\nc\n"), []byte("x\ny\nz\n"), "x", 3)
	if !strings.Contains(diff, "more lines truncated") {
		t.Fatalf("diff=%q", diff)
	}
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root, "govna", "release.md"), []byte("doc-only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, _ := canon.Render(canon.Config{Flavor: canon.Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	current := map[string][]byte{}
	for _, f := range files {
		current[f.Path] = f.Content
	}
	got := targetOnly(root, current, nil, canon.Code, "widget")
	if got["govna/release.md"] != "present in other flavor canon" {
		t.Fatalf("target-only=%v", got)
	}
}

func TestTargetOnlyPreserveIsDurable(t *testing.T) {
	root := fixture(t)
	target := "govna/release.md"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(target)), []byte("repository-owned release notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\n"+target+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for inspection := 1; inspection <= 2; inspection++ {
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || !clean {
			t.Fatalf("inspection %d clean=%v err=%v", inspection, clean, err)
		}
		file := reportFile(t, report, target)
		if file.Classification != "preserve" || actionable(file.Classification) || len(file.PreserveEntries) != 1 || file.PreserveEntries[0] != target {
			t.Fatalf("inspection %d target-only preserve=%+v", inspection, file)
		}
		encoded, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(encoded, []byte(`"relpath":"govna/release.md"`)) || !bytes.Contains(encoded, []byte(`"classification":"preserve"`)) {
			t.Fatalf("inspection %d JSON omits settled target-only file: %s", inspection, encoded)
		}
		body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
		if strings.Contains(body, "**`govna/release.md`**:") {
			t.Fatalf("inspection %d repeated the settled routing question: %s", inspection, body)
		}
	}
}

func TestReplacementMissingRoutesAfterDirectUpdate(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "govna", "audit.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "govna", "drift-scan.md"), []byte("retired local content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
	if err != nil || clean {
		t.Fatalf("clean=%v err=%v", clean, err)
	}
	if replacement := reportFile(t, report, "govna/audit.md"); replacement.Classification != "missing-in-target" {
		t.Fatalf("replacement=%+v", replacement)
	}
	retired := reportFile(t, report, "govna/drift-scan.md")
	if retired.Classification != "target-has-no-canon" || replacementMissingPath(retired) != "govna/audit.md" {
		t.Fatalf("retired=%+v", retired)
	}
	body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
	install := "Install `govna/audit.md` before retired-source routing for `govna/drift-scan.md`."
	route := "Which action should Govna record after installing `govna/audit.md`: keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?"
	orderingAT := "Verify `govna/audit.md` matches its applicable rendered canon region before retired-source routing for `govna/drift-scan.md`."
	for _, want := range []string{"- `govna/audit.md` — `missing-in-target`", install, route, orderingAT} {
		if !strings.Contains(body, want) {
			t.Errorf("replacement route omits %q: %s", want, body)
		}
	}
	if strings.Index(body, install) > strings.Index(body, route) {
		t.Errorf("replacement installation is not ordered before retired-source routing: %s", body)
	}
	if strings.Contains(strings.ToLower(body), "restore") {
		t.Errorf("replacement-missing route still offers restore: %s", body)
	}
}

func TestRoutingCapabilityMatrixAndConditionalATs(t *testing.T) {
	report := Report{
		Header: Header{Flavor: "code", RepoName: "widget"},
		Files: []FileResult{
			{Path: "canon.md", Classification: "ambiguity", CanonReference: "govna @ v0.49.0: canon.md"},
			{Path: "target.md", Classification: "target-has-no-canon", CanonReference: "present in prior canon baseline"},
			{Path: "govna/drift-scan.md", Classification: "target-has-no-canon", CanonReference: "retired canon path; replacement missing: govna/audit.md"},
			{Path: "legacy.md", Classification: "target-has-no-canon", CanonReference: "present in prior canon baseline", LegacyPreserveMarkers: []string{"preserve legacy.md"}},
			{Path: "marker.md", Classification: "ambiguity", LegacyPreserveMarkers: []string{"preserve marker.md"}, PreserveEntries: []string{"marker.md"}, legacyOnly: true, targetPresent: true, targetHash: "abc123"},
		},
	}
	body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
	if again := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome()); again != body {
		t.Fatal("identical routing reports produced unstable emitted ATs")
	}
	if !strings.Contains(body, "- `marker.md` — `ambiguity`: the Director must choose whether to convert the exact legacy phrase or remove only that phrase.") {
		t.Errorf("marker-only classification explanation describes a file action: %s", body)
	}

	questions := map[string][]string{
		"canon.md":            {"update (sync)", "keep local (preserve)", "destination named in the response (migrate)", "remove (delete)"},
		"target.md":           {"keep local (preserve)", "destination named in the response (migrate)", "remove (delete)"},
		"govna/drift-scan.md": {"after installing `govna/audit.md`", "keep local (preserve)", "destination named in the response (migrate)", "remove (delete)"},
		"legacy.md":           {"keep local (preserve)", "destination named in the response (migrate)", "remove (delete)"},
		"marker.md":           {"convert the exact legacy phrase into `govna/preserve.txt`", "remove only the phrase"},
	}
	for path, required := range questions {
		line := lineContaining(t, body, "**`"+path+"`**:")
		for _, want := range required {
			if !strings.Contains(line, want) {
				t.Errorf("%s routing question omits %q: %s", path, want, line)
			}
		}
		if path != "canon.md" && strings.Contains(line, "update (sync)") {
			t.Errorf("%s routing question offers impossible sync: %s", path, line)
		}
	}
	if strings.Contains(strings.ToLower(body), "restore") {
		t.Errorf("routing matrix contains restore: %s", body)
	}

	for _, want := range []string{
		"Verify `canon.md` matches its applicable rendered canon region when its resolved action is sync.",
		"Verify `canon.md` is absent from `govna/preserve.txt` when its resolved action is sync.",
		"Verify `target.md` remains present when its resolved action is preserve.",
		"Verify `target.md` occurs exactly once in `govna/preserve.txt` when its resolved action is preserve.",
		"Verify `target.md` is absent when its resolved action is delete.",
		"Verify `target.md` is absent from `govna/preserve.txt` when its resolved action is delete.",
		"Verify the Director response names a migration destination for `target.md` when its resolved action is migrate.",
		"Verify `target.md` is absent unless the Director explicitly preserves it when its resolved action is migrate.",
		"Verify any canon-backed migration destination for `target.md` matches its applicable rendered canon region.",
		"Verify any repository-owned migration destination for `target.md` matches the Director-stated result.",
		"Verify `target.md` is absent from `govna/preserve.txt` when its resolved action is a canon-backed migration.",
		"Verify `govna/audit.md` matches its applicable rendered canon region before retired-source routing for `govna/drift-scan.md`.",
		"Convert `preserve legacy.md` into `govna/preserve.txt` for a preserve choice on `legacy.md`.",
		"Verify legacy-phrase cleanup for `legacy.md` starts only after every applicable target-state and registry-state AT passes.",
		"Verify the Unreleased CHANGELOG Summary changes only through exact removal of `preserve legacy.md` for `legacy.md`.",
		"Verify every CHANGELOG line outside the Unreleased Summary remains byte-identical for `legacy.md`.",
		"Verify every exact legacy phrase in `preserve legacy.md` is absent from the Unreleased CHANGELOG Summary after the resolved action for `legacy.md`.",
		"Verify `marker.md` remains byte-identical with SHA-256 `abc123` for every marker-only choice.",
		"Verify `marker.md` occurs exactly once in `govna/preserve.txt` when its marker-only action is convert.",
		"Verify `marker.md` remains in `govna/preserve.txt` when its marker-only action is remove.",
		"Verify every exact legacy phrase in `preserve marker.md` is absent from the Unreleased CHANGELOG Summary after the marker-only choice for `marker.md`.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("conditional routing ATs omit %q", want)
		}
	}
	if strings.Contains(body, "Verify `target.md` matches its applicable rendered canon region when its resolved action is sync.") {
		t.Errorf("target-only ATs include impossible sync: %s", body)
	}

	wantNumber := 1
	for line := range strings.SplitSeq(body, "\n") {
		if !strings.HasPrefix(line, "**AT") {
			continue
		}
		wantPrefix := fmt.Sprintf("**AT%d** ", wantNumber)
		if !strings.HasPrefix(line, wantPrefix) {
			t.Fatalf("unstable AT numbering at %q, want prefix %q", line, wantPrefix)
		}
		wantNumber++
	}
	if wantNumber == 2 {
		t.Fatal("routing report emitted no conditional ATs")
	}
}

func TestMarkerOnlyRoutingStateVariants(t *testing.T) {
	for _, tc := range []struct {
		name           string
		file           FileResult
		wantTargetAT   string
		wantRegistryAT string
	}{
		{
			name:           "absent target and registry",
			file:           FileResult{Path: "ghost.md", Classification: "ambiguity", LegacyPreserveMarkers: []string{"preserve ghost.md"}, legacyOnly: true},
			wantTargetAT:   "Verify `ghost.md` remains absent for every marker-only choice.",
			wantRegistryAT: "Verify `ghost.md` remains absent from `govna/preserve.txt` when its marker-only action is remove.",
		},
		{
			name:           "present target and registry",
			file:           FileResult{Path: "local.md", Classification: "ambiguity", LegacyPreserveMarkers: []string{"local.md: keep local"}, PreserveEntries: []string{"local.md"}, legacyOnly: true, targetPresent: true, targetHash: "abc123"},
			wantTargetAT:   "Verify `local.md` remains byte-identical with SHA-256 `abc123` for every marker-only choice.",
			wantRegistryAT: "Verify `local.md` remains in `govna/preserve.txt` when its marker-only action is remove.",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := buildAC(Report{Header: Header{Flavor: "doc", RepoName: "handbook"}, Files: []FileResult{tc.file}}, "govna/ac1-audit-v0.49.0.md", notApplicableOutcome("`Not applicable`", "DOC fixture"))
			for _, want := range []string{tc.wantTargetAT, tc.wantRegistryAT, "occurs exactly once in `govna/preserve.txt` when its marker-only action is convert", "legacy-phrase cleanup for `" + tc.file.Path + "` starts only after every applicable target-state and registry-state AT passes", "Unreleased CHANGELOG Summary changes only through exact removal", "CHANGELOG line outside the Unreleased Summary remains byte-identical", "is absent from the Unreleased CHANGELOG Summary after the marker-only choice"} {
				if !strings.Contains(body, want) {
					t.Errorf("marker-only route omits %q: %s", want, body)
				}
			}
		})
	}
}

func TestMarkerOnlyClassificationUsesIndependentFileAction(t *testing.T) {
	writeLegacy := func(t *testing.T, root, phrase string) {
		t.Helper()
		content := "# Changelog\n\n## Unreleased\n\n- " + phrase + "\n"
		if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("matching canon target", func(t *testing.T) {
		root := fixture(t)
		writeLegacy(t, root, "preserve README.md")
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || clean {
			t.Fatalf("clean=%v err=%v", clean, err)
		}
		file := reportFile(t, report, "README.md")
		if !markerOnly(file) || !file.targetPresent || file.targetHash == "" || file.Classification != "ambiguity" {
			t.Fatalf("matching canon marker=%+v", file)
		}
		body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
		line := lineContaining(t, body, "**`README.md`**:")
		if !strings.Contains(line, "convert the exact legacy phrase") || strings.Contains(line, "update (sync)") || strings.Contains(line, "migrate") {
			t.Fatalf("matching canon marker route=%s", line)
		}
	})

	t.Run("independently actionable target", func(t *testing.T) {
		root := fixture(t)
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("repository edit\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		writeLegacy(t, root, "preserve README.md")
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || clean {
			t.Fatalf("clean=%v err=%v", clean, err)
		}
		file := reportFile(t, report, "README.md")
		if markerOnly(file) || file.Classification != "ambiguity" {
			t.Fatalf("actionable canon marker=%+v", file)
		}
		body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
		line := lineContaining(t, body, "**`README.md`**:")
		for _, want := range []string{"update (sync)", "keep local (preserve)", "destination named in the response (migrate)", "remove (delete)"} {
			if !strings.Contains(line, want) {
				t.Errorf("actionable marker route omits %q: %s", want, line)
			}
		}
		if !strings.Contains(body, "Convert `preserve README.md` into `govna/preserve.txt` for a preserve choice on `README.md`.") {
			t.Errorf("actionable marker omits preserve conversion: %s", body)
		}
	})

	t.Run("missing actionable target", func(t *testing.T) {
		root := fixture(t)
		if err := os.Remove(filepath.Join(root, "README.md")); err != nil {
			t.Fatal(err)
		}
		writeLegacy(t, root, "preserve README.md")
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || clean {
			t.Fatalf("clean=%v err=%v", clean, err)
		}
		file := reportFile(t, report, "README.md")
		if markerOnly(file) || file.Classification != "ambiguity" || file.targetPresent || !file.targetInspected {
			t.Fatalf("missing actionable marker=%+v", file)
		}
		body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
		if !strings.Contains(body, "- Compare `README.md` with `diff -u /dev/null <scratch>/README.md`.") || strings.Contains(body, "diff -ru <scratch>/README.md README.md") {
			t.Fatalf("missing target has invalid Audit Review command: %s", body)
		}
	})

	t.Run("settled target-only preserve", func(t *testing.T) {
		root := fixture(t)
		target := "govna/release.md"
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(target)), []byte("repository-owned release notes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\n"+target+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		originalChangeLog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
		if err != nil {
			t.Fatal(err)
		}
		writeLegacy(t, root, "preserve "+target)
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || clean {
			t.Fatalf("clean=%v err=%v", clean, err)
		}
		file := reportFile(t, report, target)
		if !markerOnly(file) || len(file.PreserveEntries) != 1 || file.PreserveEntries[0] != target {
			t.Fatalf("settled target-only marker=%+v", file)
		}
		body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
		line := lineContaining(t, body, "**`"+target+"`**:")
		if !strings.Contains(line, "convert the exact legacy phrase") || strings.Contains(line, "migrate") {
			t.Fatalf("settled target-only marker route=%s", line)
		}
		if err := os.WriteFile(filepath.Join(root, "CHANGELOG.md"), originalChangeLog, 0o644); err != nil {
			t.Fatal(err)
		}
		second, settled, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || !settled {
			t.Fatalf("second inspection settled=%v err=%v", settled, err)
		}
		if got := reportFile(t, second, target); got.Classification != "preserve" || actionable(got.Classification) {
			t.Fatalf("second inspection target=%+v", got)
		}
	})
}

func TestDocDotfileAuditRegression(t *testing.T) {
	root := renderedFixture(t, canon.Doc, "")
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	git(t, root, "config", "user.name", "Fixture")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "govna apply")

	intended, err := os.ReadFile(filepath.Join("..", "canon", "assets", "overlays", "doc", "files", ".gitignore.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, intended) {
		t.Fatalf("DOC fixture .gitignore differs from intended source\ngot:\n%s\nwant:\n%s", actual, intended)
	}
	baselineData, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	prior, err := parseBaseline(baselineData, canon.Doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := prior.Entries[".gitignore"]; !ok {
		t.Fatal("DOC prior baseline omits .gitignore")
	}
	report, clean, err := inspect(Config{RepoName: "widget", DiffLines: 200, invocation: "govna audit --repo-name widget"}, root)
	if err != nil || !clean {
		t.Fatalf("DOC inspect clean=%v err=%v report=%+v", clean, err, report)
	}
	ignore := reportFile(t, report, ".gitignore")
	if ignore.Classification != "match" || ignore.CanonReference == "" {
		t.Fatalf("DOC .gitignore classification=%+v", ignore)
	}
	body := buildAC(report, "govna/ac1-audit-v0.49.0.md", notApplicableOutcome("`Not applicable`", "DOC fixture"))
	if strings.Contains(body, "**`.gitignore`**:") || strings.Contains(body, "unresolved rendered reference") {
		t.Fatalf("DOC .gitignore produced impossible routing: %s", body)
	}
}

func TestAuditGoldens(t *testing.T) {
	report := Report{
		Header: Header{Invocation: "govna audit --repo-name widget", CanonSHA: "v0.49.0", Target: "<TARGET>", Flavor: "code", FlavorSource: "explicit", RepoName: "widget", CanonVersion: "v0.28.0", CodeStack: "Go"},
		Files: []FileResult{
			{Path: "README.md", Classification: "clear-sync", PriorCommits: []string{"abc123"}, CanonReference: "govna @ v0.49.0: README.md", CompareCommand: "compare the embedded Govna file with the repository file: README.md"},
			{Path: "govna/canon-baseline.txt", Classification: "clear-sync", CanonReference: "generated baseline manifest", CompareCommand: "compare generated baseline with target govna/canon-baseline.txt"},
			{Path: "local.md", Classification: "target-has-no-canon", CanonReference: "name-referenced from divergent governed file", CompareCommand: "review local.md because it is not in the selected embedded Govna files"},
			{Path: "plan.md", Classification: "expected-divergence"},
		},
		Emitted: &Emitted{ACStub: "govna/ac7-audit-v0.49.0.md"},
	}
	markdown, err := os.ReadFile("testdata/actionable-golden.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := buildAC(report, report.Emitted.ACStub, inferredBuildOutcome()); got != string(markdown) {
		t.Fatalf("markdown golden mismatch\n%s", got)
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(report); err != nil {
		t.Fatal(err)
	}
	jsonGolden, err := os.ReadFile("testdata/actionable-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if encoded.String() != string(jsonGolden) {
		t.Fatalf("JSON golden mismatch\n%s", encoded.String())
	}
}

func TestExistingAndMissingBaselineClassification(t *testing.T) {
	t.Run("existing stale baseline is sync", func(t *testing.T) {
		root := fixture(t)
		path := filepath.Join(root, filepath.FromSlash(baselinePath))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.Replace(data, []byte("canon_version = v0.49.0\n"), []byte("canon_version = v0.41.0\n"), 1)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || clean {
			t.Fatalf("clean=%v err=%v", clean, err)
		}
		baseline := reportFile(t, report, baselinePath)
		if baseline.Classification != "clear-sync" {
			t.Fatalf("baseline=%+v", baseline)
		}
		disposition := validationDisposition(root, report)
		if disposition.kind != validationInferred || !strings.Contains(disposition.evidence, "`./build.sh`") {
			t.Fatalf("validation disposition=%+v", disposition)
		}
		body := buildAC(report, "govna/ac1-audit-v0.49.0.md", disposition)
		if !strings.Contains(body, "### Files ready to update\n\n- `govna/canon-baseline.txt` — `clear-sync`: the file still matches the previously installed Govna version and is safe to update.") || !strings.Contains(body, "## Migration findings\n\n- None.") {
			t.Fatalf("body=%s", body)
		}
	})
	t.Run("missing baseline is migration", func(t *testing.T) {
		root := fixture(t)
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(baselinePath))); err != nil {
			t.Fatal(err)
		}
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || clean {
			t.Fatalf("clean=%v err=%v", clean, err)
		}
		baseline := reportFile(t, report, baselinePath)
		if baseline.Classification != "migration-required" {
			t.Fatalf("baseline=%+v", baseline)
		}
		body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
		want := "- Write `govna/canon-baseline.txt` from the final temporary render only after all other work is complete and the repository check succeeds."
		if !strings.Contains(body, want) {
			t.Fatalf("body=%s", body)
		}
	})
}

func TestValidationManifestReachability(t *testing.T) {
	for _, tc := range []struct {
		name, stack, manifest string
	}{
		{"go", "Go", "go.mod"},
		{"rust", "Rust", "Cargo.toml"},
		{"swift", "Swift", "Package.swift"},
		{"terraform lock", "Terraform", ".terraform.lock.hcl"},
		{"terraform root file", "Terraform", "main.tf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tc.manifest), []byte("fixture\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if !stackManifestReachable(root, tc.stack) {
				t.Fatalf("%s manifest %s is not reachable", tc.stack, tc.manifest)
			}
		})
	}

	for _, tc := range []struct {
		name, stack, manifest string
		directory             bool
	}{
		{"missing", "Go", "", false},
		{"directory", "Go", "go.mod", true},
		{"other stack only", "Go", "Cargo.toml", false},
		{"terraform directory", "Terraform", "main.tf", true},
		{"unsupported Node", "Node", "package.json", false},
		{"unsupported Python", "Python", "pyproject.toml", false},
		{"unsupported Java", "Java", "pom.xml", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.manifest != "" {
				path := filepath.Join(root, tc.manifest)
				var err error
				if tc.directory {
					err = os.Mkdir(path, 0o755)
				} else {
					err = os.WriteFile(path, []byte("fixture\n"), 0o644)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if stackManifestReachable(root, tc.stack) {
				t.Fatalf("%s accepted invalid manifest evidence %q", tc.stack, tc.manifest)
			}
		})
	}
}

func TestValidationStackSelectionSources(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configure  func(*testing.T, string) Config
		wantHeader string
	}{
		{
			name: "explicit",
			configure: func(t *testing.T, root string) Config {
				replaceFileText(t, filepath.Join(root, "govna", "metadata.txt"), "code_stack = Go\n", "code_stack = Rust\n")
				return Config{Flavor: "code", Stack: "Go", RepoName: "widget", DiffLines: 200, invocation: "govna audit --flavor code --stack Go"}
			},
			wantHeader: "Rust",
		},
		{
			name: "metadata",
			configure: func(_ *testing.T, _ string) Config {
				return Config{DiffLines: 200, invocation: "govna audit"}
			},
			wantHeader: "Go",
		},
		{
			name: "manifest",
			configure: func(t *testing.T, root string) Config {
				if err := os.Remove(filepath.Join(root, "govna", "metadata.txt")); err != nil {
					t.Fatal(err)
				}
				return Config{Flavor: "code", RepoName: "widget", DiffLines: 200, invocation: "govna audit --flavor code"}
			},
			wantHeader: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			makeBaselineStale(t, root)
			cfg := tc.configure(t, root)
			report, clean, err := inspect(cfg, root)
			if err != nil || clean {
				t.Fatalf("clean=%v err=%v", clean, err)
			}
			if report.Header.CodeStack != tc.wantHeader {
				t.Fatalf("public code_stack=%q want=%q", report.Header.CodeStack, tc.wantHeader)
			}
			outcome := validationDisposition(root, report)
			if outcome.kind != validationInferred || outcome.evidence != inferredBuildOutcome().evidence {
				t.Fatalf("validation outcome=%+v", outcome)
			}
			encoded, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(encoded, []byte("selectedStack")) || bytes.Contains(encoded, []byte("selected_stack")) {
				t.Fatalf("private selected stack leaked into JSON: %s", encoded)
			}
		})
	}
}

func TestValidationEvidenceOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{
			name: "missing selected manifest",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "directory selected manifest",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(root, "go.mod"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "other stack only",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname = \"widget\"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			makeBaselineStale(t, root)
			tc.mutate(t, root)
			report, clean, err := inspect(Config{Flavor: "code", Stack: "Go", RepoName: "widget", DiffLines: 200, invocation: "govna audit --flavor code --stack Go"}, root)
			if err != nil || clean {
				t.Fatalf("clean=%v err=%v", clean, err)
			}
			if outcome := validationDisposition(root, report); outcome.kind != validationUnresolved {
				t.Fatalf("validation outcome=%+v", outcome)
			}
		})
	}

	t.Run("exact declarations", func(t *testing.T) {
		root := fixture(t)
		makeBaselineStale(t, root)
		report, _, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil {
			t.Fatal(err)
		}
		replaceFileText(t, filepath.Join(root, "AGENTS.md"), "- Run `./build.sh` as the first validation command", "- Run `make` as the first validation command")
		if outcome := validationDisposition(root, report); outcome.kind != validationUnresolved {
			t.Fatalf("validation outcome=%+v", outcome)
		}
	})

	t.Run("regular build file", func(t *testing.T) {
		root := fixture(t)
		makeBaselineStale(t, root)
		report, _, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil {
			t.Fatal(err)
		}
		build := filepath.Join(root, "build.sh")
		if err := os.Remove(build); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(build, 0o755); err != nil {
			t.Fatal(err)
		}
		if outcome := validationDisposition(root, report); outcome.kind != validationUnresolved {
			t.Fatalf("validation outcome=%+v", outcome)
		}
	})

	t.Run("no baseline update", func(t *testing.T) {
		outcome := validationDisposition(t.TempDir(), Report{Header: Header{Flavor: "code"}, selectedStack: "Go"})
		if outcome.kind != validationNotApplicable || outcome.evidence != "`Not applicable` because no baseline migration is present" || outcome.reason != "no baseline migration is present" {
			t.Fatalf("validation outcome=%+v", outcome)
		}
	})

	t.Run("doc evidence", func(t *testing.T) {
		root := renderedFixture(t, canon.Doc, "")
		report := Report{Header: Header{Flavor: "doc"}, Files: []FileResult{{Path: baselinePath, Classification: "clear-sync"}}}
		outcome := validationDisposition(root, report)
		if outcome.kind != validationNotApplicable || outcome.evidence != "`Not applicable` inferred from exact DOC governance evidence" || outcome.reason != "inferred from exact DOC governance evidence" {
			t.Fatalf("validation outcome=%+v", outcome)
		}
		replaceFileText(t, filepath.Join(root, "govna", "release.md"), "define no automated content-validation command.", "define repository content validation locally.")
		if outcome := validationDisposition(root, report); outcome.kind != validationUnresolved {
			t.Fatalf("validation outcome=%+v", outcome)
		}
	})
}

func TestUnresolvedValidationGolden(t *testing.T) {
	root := fixture(t)
	makeBaselineStale(t, root)
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	inspected, _, err := inspect(Config{Flavor: "code", Stack: "Go", RepoName: "widget", DiffLines: 200, invocation: "govna audit --flavor code --stack Go"}, root)
	if err != nil {
		t.Fatal(err)
	}
	outcome := validationDisposition(root, inspected)
	if outcome.kind != validationUnresolved {
		t.Fatalf("validation outcome=%+v", outcome)
	}
	report := Report{
		Header: Header{Flavor: "code", RepoName: "widget"},
		Files: []FileResult{
			{Path: "AGENTS.md", Classification: "clear-sync", Boundary: "## Project Rules", protectedHash: "abc123"},
			{Path: baselinePath, Classification: "clear-sync"},
			{Path: "local.md", Classification: "ambiguity"},
			{Path: "plan.md", Classification: "preserve"},
		},
	}
	body := buildAC(report, "govna/ac7-audit-v0.49.0.md", outcome)
	want, err := os.ReadFile("testdata/unresolved-validation-golden.md")
	if err != nil {
		t.Fatal(err)
	}
	if body != string(want) {
		t.Fatalf("unresolved validation golden mismatch\n%s", body)
	}
	ordered := []string{
		"1. **`local.md`**: Which action should Govna record: update (sync), keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?",
		"2. **Repository check**: Which command should run after the selected file updates, or what repository evidence shows that no command applies?",
		"Preserve the protected region in `AGENTS.md` from `## Project Rules` through EOF with SHA-256 `abc123` for any update choice.",
		"Choose the repository check in chat.",
		"Verify the chosen repository check succeeds before `govna/canon-baseline.txt` installation.",
		"Verify the final file update installed `govna/canon-baseline.txt` from the same temporary render.",
	}
	position := -1
	for _, text := range ordered {
		next := strings.Index(body, text)
		if next <= position {
			t.Fatalf("unresolved instruction %q is missing or out of order: %s", text, body)
		}
		position = next
	}
	if strings.Contains(body, "Repository check**: `./build.sh`") {
		t.Fatalf("unresolved validation was hard-coded: %s", body)
	}
}

func TestInferredValidationOmitsManualRouting(t *testing.T) {
	report := Report{Header: Header{Flavor: "code", RepoName: "widget"}, Files: []FileResult{{Path: baselinePath, Classification: "clear-sync"}}}
	code := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
	for _, absent := range []string{"**Repository check**:", "Choose the repository check in chat."} {
		if strings.Contains(code, absent) {
			t.Fatalf("inferred CODE emission contains %q: %s", absent, code)
		}
	}
	report.Header.Flavor = "doc"
	doc := buildAC(report, "govna/ac1-audit-v0.49.0.md", notApplicableOutcome("`Not applicable` inferred from exact DOC governance evidence", "inferred from exact DOC governance evidence"))
	for _, absent := range []string{"**Repository check**:", "Choose the repository check in chat."} {
		if strings.Contains(doc, absent) {
			t.Fatalf("inferred DOC emission contains %q: %s", absent, doc)
		}
	}
}

func TestEmittedSummaryAndFlavorInstructions(t *testing.T) {
	report := Report{Header: Header{Flavor: "code", RepoName: "widget"}, Files: []FileResult{{Path: "README.md", Classification: "clear-sync"}, {Path: "local.md", Classification: "ambiguity"}, {Path: "plan.md", Classification: "preserve"}}}
	body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
	wantPrefix := "# AC1 Adopt Govna Governance Files v0.49.0\n\n## Summary\n\nThis AC updates `widget` to Govna's embedded governance files (canon v0.49.0). The result label (classification) beside each path explains why Govna can update it, must leave it unchanged, or needs a Director choice. Installing the selected updates is the adoption step.\n\nGovna found 1 file ready to update, 0 required control files to add, 1 file needing a Director decision, and 1 file that will stay unchanged.\n\n## In Scope"
	if !strings.HasPrefix(body, wantPrefix) {
		t.Fatalf("summary=%s", body)
	}
	if strings.Contains(body, "Review Govna File Updates") {
		t.Fatalf("emitted title is not adoption-specific: %s", body)
	}
	reachability := "Confirm each file selected for update exists in the selected CODE render."
	if !strings.Contains(body, reachability) {
		t.Fatalf("CODE instruction missing: %s", body)
	}
	report.Header.Flavor = "doc"
	doc := buildAC(report, "govna/ac1-audit-v0.49.0.md", notApplicableOutcome("`Not applicable`", ""))
	if !strings.HasPrefix(doc, wantPrefix) {
		t.Fatalf("DOC summary=%s", doc)
	}
	if strings.Contains(doc, reachability) {
		t.Fatalf("DOC contains CODE instruction: %s", doc)
	}
	if strings.Contains(doc, "installation ().") || !strings.Contains(doc, "installation (no reason recorded).") {
		t.Fatalf("empty NotApplicable reason did not render the fallback phrase: %s", doc)
	}

	pluralReport := Report{
		Header: Header{Flavor: "code", RepoName: "widget"},
		Files: []FileResult{
			{Path: "README.md", Classification: "clear-sync"},
			{Path: baselinePath, Classification: "clear-sync"},
			{Path: "govna/metadata.txt", Classification: "migration-required"},
			{Path: "local.md", Classification: "preserve"},
			{Path: "plan.md", Classification: "expected-divergence"},
		},
	}
	plural := buildAC(pluralReport, "govna/ac2-audit-v0.49.0.md", inferredBuildOutcome())
	wantCounts := "Govna found 2 files ready to update, 1 required control file to add, 0 files needing a Director decision, and 2 files that will stay unchanged.\n\n## In Scope"
	if !strings.Contains(plural, wantCounts) {
		t.Fatalf("plural summary=%s", plural)
	}
}

func TestGeneratedInstructionBranches(t *testing.T) {
	report := Report{
		Header: Header{Flavor: "code", RepoName: "widget"},
		Files: []FileResult{
			{Path: "AGENTS.md", Classification: "ambiguity", Boundary: "## Project Rules", protectedHash: "abc123", forceSync: true, CanonReference: "govna @ v0.49.0: AGENTS.md", LegacyPreserveMarkers: []string{"do not sync AGENTS.md"}},
			{Path: baselinePath, Classification: "migration-required"},
			{Path: "govna/metadata.txt", Classification: "migration-required"},
			{Path: "govna/other.md", Classification: "migration-required"},
			{Path: "local.md", Classification: "ambiguity", CanonReference: "govna @ v0.49.0: local.md", LegacyPreserveMarkers: []string{"preserve local.md"}},
		},
	}
	body := buildAC(report, "govna/ac1-audit-v0.49.0.md", inferredBuildOutcome())
	for _, want := range []string{
		"- Confirm each file selected for update exists in the selected CODE render.",
		"- Apply each Director choice only to its authorized file region.",
		"- Verify every applicable direct-update AT for `AGENTS.md` before legacy-phrase cleanup.",
		"- Remove `do not sync AGENTS.md` from `CHANGELOG.md` after resolved-state verification for `AGENTS.md`.",
		"- Convert `preserve local.md` into `govna/preserve.txt` for a preserve choice on `local.md`.",
		"- Verify every applicable resolved-route AT for `local.md` before legacy-phrase cleanup.",
		"- Remove `preserve local.md` from `CHANGELOG.md` after resolved-state verification for `local.md`.",
		"- Write `govna/canon-baseline.txt` from the final temporary render only after all other work is complete and the repository check succeeds.",
		"- Create `govna/metadata.txt` from the selected temporary render before `govna/canon-baseline.txt` installation.",
		"- Create `govna/other.md` from the selected temporary render.",
		"Preserve the protected region in `AGENTS.md` from `## Project Rules` through EOF with SHA-256 `abc123` for any update choice.",
		"Verify `AGENTS.md` matches its applicable rendered canon region when its resolved action is sync.",
		"Verify `AGENTS.md` is absent from `govna/preserve.txt` when its resolved action is sync.",
		"Verify legacy-phrase cleanup for `AGENTS.md` starts only after every applicable target-state and registry-state AT passes.",
		"Verify the Unreleased CHANGELOG Summary changes only through exact removal of `do not sync AGENTS.md` for `AGENTS.md`.",
		"Verify every CHANGELOG line outside the Unreleased Summary remains byte-identical for `AGENTS.md`.",
		"Verify every exact legacy phrase in `do not sync AGENTS.md` is absent from the Unreleased CHANGELOG Summary after the resolved action for `AGENTS.md`.",
		"Verify the final file update installed `govna/canon-baseline.txt` from the same temporary render.",
		"`PENDING` — immutable audit emission; workflow state is tracked in the active session.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("audit body omits %q", want)
		}
	}
	legacyIndex := strings.Index(body, "Convert `preserve local.md`")
	routingIndex := strings.Index(body, "### Routing Decisions")
	if legacyIndex < 0 || routingIndex < 0 || legacyIndex > routingIndex {
		t.Errorf("legacy instruction is not under Adoption Instructions before routing: %s", body)
	}
	for _, invalid := range []string{"When `AGENTS.md` is synced", "Install and verify", "Resolve the legacy preserve phrase", "before applying changes", "restore"} {
		if strings.Contains(body, invalid) {
			t.Errorf("audit body retains invalid text %q", invalid)
		}
	}
	report.Header.Flavor = "doc"
	if doc := buildAC(report, "govna/ac1-audit-v0.49.0.md", notApplicableOutcome("`Not applicable`", "")); strings.Contains(doc, "selected CODE render") {
		t.Errorf("DOC body contains CODE reachability instruction: %s", doc)
	}
}

func TestClassificationExplanationsAndPlainTallies(t *testing.T) {
	meanings := map[string]string{
		"match":               "the file already needs no Govna update",
		"missing-in-target":   "a file from current Govna rules is missing from the repository",
		"expected-divergence": "the repository is expected to keep its own version of this file",
		"preserve":            "the preserve list says to keep the repository's version",
		"clear-sync":          "the file still matches the previously installed Govna version and is safe to update",
		"ambiguity":           "Govna cannot safely choose between updating and keeping the file",
		"target-has-no-canon": "the file is absent from the selected current canon but specific repository evidence connects it to Govna",
		"migration-required":  "a required Govna control file is missing and must be added through the AC",
	}
	if got, want := classificationMeaning("force-sync"), "this file's governed structure always syncs to canon, regardless of local edits or the preserve list"; got != want {
		t.Errorf("force-sync meaning=%q want=%q", got, want)
	}
	var files []FileResult
	for classification, meaning := range meanings {
		if got := classificationMeaning(classification); got != meaning {
			t.Errorf("%s meaning=%q want=%q", classification, got, meaning)
		}
		files = append(files, FileResult{Classification: classification})
	}
	// A force-synced file must tally under "force-sync" regardless of its underlying Classification.
	files = append(files, FileResult{Classification: "preserve", forceSync: true})
	tally := plainTally(files)
	for _, label := range []string{
		"1 file needs no update",
		"1 expected local difference",
		"1 file kept by Director choice",
		"1 file needs a Director choice",
		"1 file is safe to update",
		"1 missing Govna file",
		"1 Govna-linked extra file",
		"1 missing required control file",
		"1 file always synced regardless of local edits",
	} {
		if !strings.Contains(tally, label) {
			t.Errorf("plain tally %q omits %q", tally, label)
		}
	}
}

func TestComparisonDescriptionsExplainTheFileState(t *testing.T) {
	for _, tc := range []struct {
		classification string
		want           string
	}{
		{"match", "matches the embedded Govna file: README.md"},
		{"missing-in-target", "repository is missing README.md; compare it with the embedded Govna file"},
		{"target-has-no-canon", "review README.md because it is not in the selected embedded Govna files"},
		{"ambiguity", "compare the embedded Govna file with the repository file: README.md"},
	} {
		if got := comparisonDescription(FileResult{Classification: tc.classification}, "README.md"); got != tc.want {
			t.Errorf("%s description=%q want=%q", tc.classification, got, tc.want)
		}
	}
}

func reportFile(t *testing.T, report Report, path string) FileResult {
	t.Helper()
	for _, file := range report.Files {
		if file.Path == path {
			return file
		}
	}
	t.Fatalf("report omits %s", path)
	return FileResult{}
}

func lineContaining(t *testing.T, body, needle string) string {
	t.Helper()
	for line := range strings.SplitSeq(body, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	t.Fatalf("body omits line containing %q", needle)
	return ""
}

func makeBaselineStale(t *testing.T, root string) {
	t.Helper()
	replaceFileText(t, filepath.Join(root, filepath.FromSlash(baselinePath)), "canon_version = v"+canon.Version+"\n", "canon_version = v0.1.0\n")
}

func replaceFileText(t *testing.T, path, old, replacement string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), old, replacement, 1)
	if updated == string(data) {
		t.Fatalf("%s omits replacement source %q", path, old)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func addMixedReviewSentinels(t *testing.T, root, path string) {
	t.Helper()
	filePath := filepath.Join(root, filepath.FromSlash(path))
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, valid := canon.ComparisonRegion(path, content); !valid {
		t.Fatalf("%s lacks its registered boundary", path)
	}
	updated := append([]byte("canon-zone-sentinel:"+path+"\n"), content...)
	if len(updated) > 0 && updated[len(updated)-1] != '\n' {
		updated = append(updated, '\n')
	}
	updated = append(updated, []byte("repository-owned-tail-sentinel:"+path+"\n")...)
	if err := os.WriteFile(filePath, updated, 0o644); err != nil {
		t.Fatal(err)
	}
}

func renderedFixture(t *testing.T, flavor canon.Flavor, stack string) string {
	t.Helper()
	root := t.TempDir()
	files, err := canon.Render(canon.Config{Flavor: flavor, RepoName: "widget", Stack: stack})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, file.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func auditReviewActionable(file FileResult) bool {
	if file.forceSync {
		return true
	}
	switch file.Classification {
	case "clear-sync", "missing-in-target", "migration-required", "ambiguity", "target-has-no-canon":
		return true
	default:
		return false
	}
}

func assertAuditReviewPath(t *testing.T, body, root, scratch string, file FileResult) {
	t.Helper()
	var instruction string
	var command *exec.Cmd
	canonZoneOnly := false
	switch {
	case !auditReviewCanonPresent(file):
		instruction = fmt.Sprintf("- Verify `%s` is absent from the selected scratch render with `test ! -e <scratch>/%s`.", file.Path, file.Path)
		if _, err := os.Lstat(filepath.Join(scratch, filepath.FromSlash(file.Path))); !os.IsNotExist(err) {
			t.Fatalf("target-only path %s unexpectedly exists in scratch render: %v", file.Path, err)
		}
	case !auditReviewTargetPresent(file):
		instruction = fmt.Sprintf("- Compare `%s` with `diff -u /dev/null <scratch>/%s`.", file.Path, file.Path)
		command = exec.Command("diff", "-u", "/dev/null", filepath.Join(scratch, filepath.FromSlash(file.Path)))
	case auditReviewHasExactBoundary(file):
		placeholderCommand := auditReviewCanonZoneCommand(file.Boundary, "<scratch>/"+file.Path, file.Path)
		instruction = fmt.Sprintf("- Compare the canon zone of `%s` above exact boundary `%s` with `%s`.", file.Path, file.Boundary, placeholderCommand)
		actualCommand := auditReviewCanonZoneCommand(
			file.Boundary,
			filepath.Join(scratch, filepath.FromSlash(file.Path)),
			filepath.Join(root, filepath.FromSlash(file.Path)),
		)
		command = exec.Command("bash", "-c", actualCommand)
		canonZoneOnly = true
	default:
		instruction = fmt.Sprintf("- Compare `%s` with `diff -ru <scratch>/%s %s`.", file.Path, file.Path, file.Path)
		command = exec.Command("diff", "-ru", filepath.Join(scratch, filepath.FromSlash(file.Path)), filepath.Join(root, filepath.FromSlash(file.Path)))
	}
	if strings.Count(body, instruction) != 1 {
		t.Fatalf("Audit Review instruction count for %s is not one: %s", file.Path, instruction)
	}
	if command == nil {
		return
	}
	output, err := command.CombinedOutput()
	if canonZoneOnly {
		if !strings.Contains(string(output), "canon-zone-sentinel:"+file.Path) {
			t.Errorf("canon-zone comparison for %s omits canon sentinel: %s", file.Path, output)
		}
		if strings.Contains(string(output), "repository-owned-tail-sentinel:"+file.Path) {
			t.Errorf("canon-zone comparison for %s includes repository tail: %s", file.Path, output)
		}
	}
	if err == nil {
		return
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("Audit Review comparison for %s is not executable: %v: %s", file.Path, err, output)
	}
}

func scratchReviewIdentityMatches(ac, detailedVersion, baseline []byte) bool {
	marker, _, ok := strings.Cut(string(ac), "\n")
	if !ok || !emission.VerifyAuditBody(ac) {
		return false
	}
	detailLines := strings.Split(strings.TrimSuffix(string(detailedVersion), "\n"), "\n")
	if len(detailLines) != 2 {
		return false
	}
	executableVersion, ok := strings.CutPrefix(detailLines[0], "Govna executable version: ")
	if !ok {
		return false
	}
	canonVersion, ok := strings.CutPrefix(detailLines[1], "Embedded governance-file version (canon version): ")
	if !ok {
		return false
	}
	wantMarker := emission.AuditMarkerPrefix + "executable " + executableVersion + " with embedded canon " + canonVersion + " sha256:"
	if !strings.HasPrefix(marker, wantMarker) {
		return false
	}
	baselineLines := strings.Split(strings.TrimSuffix(string(baseline), "\n"), "\n")
	return len(baselineLines) >= 2 && baselineLines[0] == "govna-canon-baseline-v1" && baselineLines[1] == "canon_version = "+canonVersion
}

func snapshotConsumerTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == ".git" && info.IsDir() {
			return filepath.SkipDir
		}
		if rel == "." || info.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			snapshot[filepath.ToSlash(rel)] = "symlink:" + target
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(content)
		snapshot[filepath.ToSlash(rel)] = fmt.Sprintf("file:%x", hash)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func assertConsumerTree(t *testing.T, root string, want map[string]string) {
	t.Helper()
	got := snapshotConsumerTree(t, root)
	if len(got) != len(want) {
		t.Fatalf("consumer file count changed during scratch review: got=%d want=%d", len(got), len(want))
	}
	for path, wantState := range want {
		if got[path] != wantState {
			t.Errorf("consumer path changed during scratch review: %s", path)
		}
	}
}

func legacyGuardedBody(prefix, version string, body []byte) []byte {
	hash := sha256.Sum256(body)
	return []byte(fmt.Sprintf("%s%s sha256:%x -->\n%s", prefix, version, hash, body))
}

func TestAuditInvocationPreservesOptionOrder(t *testing.T) {
	cfg, err := parse([]string{"--repo-name", "widget", "--json", "--diff-lines", "12", "--flavor", "code"})
	if err != nil {
		t.Fatal(err)
	}
	want := "govna audit --repo-name widget --json --diff-lines 12 --flavor code"
	if cfg.invocation != want {
		t.Fatalf("invocation=%q want=%q", cfg.invocation, want)
	}
}
