package emission

import (
	"fmt"
	"strings"
)

// ParentProfile holds the text that an emitted child AC needs when the parent
// gains its first child or loses its last one: the parent's contract and
// documents then gain or lose the child rules.
type ParentProfile struct {
	// Instructions is the complete instruction section, ending with a blank line.
	Instructions string
	// Paths lists the parent files the AC updates, then the baseline.
	Paths []string
}

// BaselinePath is the saved-hash file that every profile update replaces last.
const BaselinePath = "govna/canon-baseline.txt"

// NewParentProfile renders the profile update for a parent. adds is true when
// the parent gains the child rules and false when it loses them. update lists
// the files to update and kept notes each file the AC leaves alone.
func NewParentProfile(adds bool, update, kept []string) ParentProfile {
	var b strings.Builder
	b.WriteString("### Parent Profile Update\n\n")
	command := "govna render --with-children <scratch>"
	if adds {
		b.WriteString("This parent declares its first child, so its contract and documents gain the child rules.\n\n")
		b.WriteString("- Complete this section before any other work in this AC.\n")
	} else {
		command = "govna render <scratch>"
		b.WriteString("This AC drops the parent's last child, so its contract and documents lose the child rules.\n\n")
		b.WriteString("- Complete this section after the orphan rewrite and the registry change.\n")
	}
	fmt.Fprintf(&b, "- Create a temporary copy of the parent's Govna files with `%s`.\n", command)
	b.WriteString("- Apply each file below from that copy through `govna/audit.md` `## Adoption procedure`.\n")
	b.WriteString("- Replace only the text above the registered boundary heading of a file that has one.\n")
	b.WriteString("- Keep that boundary heading and every line below it.\n")
	fmt.Fprintf(&b, "- Replace each updated file's line in `%s` with its line from `<scratch>/%s` after that file matches the copy.\n\n", BaselinePath, BaselinePath)
	b.WriteString("Files to update:\n\n")
	for _, path := range update {
		fmt.Fprintf(&b, "- `%s`\n", path)
	}
	fmt.Fprintf(&b, "- `%s`\n\n", BaselinePath)
	if len(kept) > 0 {
		b.WriteString("Files left unchanged:\n\n")
		for _, note := range kept {
			fmt.Fprintf(&b, "- %s\n", note)
		}
		b.WriteString("\n")
	}
	return ParentProfile{Instructions: b.String(), Paths: append(append([]string{}, update...), BaselinePath)}
}

// Tests returns the three acceptance tests of a profile update, numbered from
// first, each followed by a blank line.
func (p ParentProfile) Tests(first int) string {
	return fmt.Sprintf("**AT%d** [Automated] [Pre-release gate] — Verify each updated parent file matches the temporary copy above its registered boundary.\n\n"+
		"**AT%d** [Automated] [Pre-release gate] — Verify every line from the boundary heading down is unchanged in each updated parent file.\n\n"+
		"**AT%d** [Automated] [Pre-release gate] — Verify each updated file's line in `%s` equals its line in the same temporary copy's baseline.\n\n", first, first+1, first+2, BaselinePath)
}
