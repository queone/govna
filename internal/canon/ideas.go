package canon

import (
	"bytes"
	"regexp"
)

// ideaItemStart matches the first line of a plan.md IE item.
var ideaItemStart = regexp.MustCompile(`^- IE[0-9]+: `)

// SpaceIdeas returns plan with exactly one blank line between each pair of
// adjacent IE items, and reports whether plan needed that change. An IE item
// is a "- IE<N>: " line plus the indented lines that continue it; two items
// are adjacent when only blank lines separate them. Every other line, each
// item's text and order, the lines after the last item, and the input's line
// endings stay unchanged. An input that needs no change comes back as is.
func SpaceIdeas(plan []byte) ([]byte, bool) {
	var out bytes.Buffer
	var gap [][]byte
	inItem, afterItem, changed := false, false, false
	prevEOL := []byte("\n")
	for _, line := range bytes.SplitAfter(plan, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		content := bytes.TrimSuffix(bytes.TrimSuffix(line, []byte("\n")), []byte("\r"))
		switch {
		case len(content) == 0 && afterItem:
			gap = append(gap, line)
			inItem = false
			continue
		case len(content) == 0:
			out.Write(line)
		case ideaItemStart.Match(content):
			if afterItem {
				if len(gap) == 1 {
					out.Write(gap[0])
				} else {
					changed = true
					out.Write(prevEOL)
				}
				gap = nil
			}
			out.Write(line)
			inItem, afterItem = true, true
		case inItem && (content[0] == ' ' || content[0] == '\t'):
			out.Write(line)
		default:
			for _, blank := range gap {
				out.Write(blank)
			}
			gap = nil
			out.Write(line)
			inItem, afterItem = false, false
		}
		if bytes.HasSuffix(line, []byte("\r\n")) {
			prevEOL = []byte("\r\n")
		} else if bytes.HasSuffix(line, []byte("\n")) {
			prevEOL = []byte("\n")
		}
	}
	if !changed {
		return plan, false
	}
	for _, blank := range gap {
		out.Write(blank)
	}
	return out.Bytes(), true
}
