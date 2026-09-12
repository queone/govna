// Package help renders command-line help pages in one shared layout: a
// three-line header, capitalized section headings, rows aligned two spaces
// past the longest form in their section, and the version and help rows the
// renderer appends to every Options section.
package help

import "strings"

// Row pairs one form (a command name, a flag, or a synopsis) with its meaning.
// A row with an empty meaning renders as the form alone.
type Row struct {
	Form    string
	Meaning string
}

// Section is one titled block of rows, optionally closed by one indented paragraph.
type Section struct {
	Title string
	Rows  []Row
	Note  string
}

// Page is one utility's help page: the header lines and its sections in order.
type Page struct {
	Name        string
	Version     string
	Description string
	URL         string
	Sections    []Section
}

const (
	headingSequence     = "\x1b[1;38;5;231m"
	descriptionSequence = "\x1b[38;5;245m"
	urlSequence         = "\x1b[38;5;242m"
	resetSequence       = "\x1b[0m"
	indent              = "  "
)

// AppendedRows returns the two rows the renderer appends to every Options section.
func AppendedRows() []Row {
	return []Row{{Form: "-v, --version", Meaning: "print executable version"}, {Form: "-h, -?, --help", Meaning: "show this help"}}
}

// Enabled reports whether colored output applies to a stream: the stream must
// be a terminal, NO_COLOR must be unset, TERM must not be dumb, and TERM or
// COLORTERM must announce 256-color or truecolor support.
func Enabled(terminal bool, lookupEnv func(string) (string, bool)) bool {
	if !terminal || lookupEnv == nil {
		return false
	}
	if _, exists := lookupEnv("NO_COLOR"); exists {
		return false
	}
	term, _ := lookupEnv("TERM")
	if term == "dumb" {
		return false
	}
	colorTerm, _ := lookupEnv("COLORTERM")
	return colorTerm == "truecolor" || colorTerm == "24bit" || strings.Contains(term, "256color")
}

// Render returns the page text, with one escape sequence per colored span when color is true.
func (p Page) Render(color bool) string {
	span := func(sequence, text string) string {
		if !color {
			return text
		}
		return sequence + text + resetSequence
	}
	var b strings.Builder
	b.WriteString(span(headingSequence, p.Name) + " v" + p.Version + "\n")
	b.WriteString(span(descriptionSequence, p.Description) + "\n")
	b.WriteString(span(urlSequence, p.URL) + "\n")
	for _, section := range p.withAppendedRows() {
		b.WriteString("\n" + span(headingSequence, section.Title) + "\n")
		width := 0
		for _, row := range section.Rows {
			width = max(width, len([]rune(row.Form)))
		}
		for _, row := range section.Rows {
			b.WriteString(indent + row.Form)
			if row.Meaning != "" {
				b.WriteString(strings.Repeat(" ", width-len([]rune(row.Form))+2) + row.Meaning)
			}
			b.WriteString("\n")
		}
		if section.Note != "" {
			if len(section.Rows) > 0 {
				b.WriteString("\n")
			}
			for line := range strings.SplitSeq(strings.TrimRight(section.Note, "\n"), "\n") {
				b.WriteString(indent + line + "\n")
			}
		}
	}
	return b.String()
}

// withAppendedRows returns the sections with the two standard rows appended to
// Options, creating Options after Commands (or Usage) when the page has none.
func (p Page) withAppendedRows() []Section {
	sections := make([]Section, 0, len(p.Sections)+1)
	insertAt := 0
	found := false
	for i, section := range p.Sections {
		if section.Title == "Options" {
			section.Rows = append(append([]Row{}, section.Rows...), AppendedRows()...)
			found = true
		}
		if section.Title == "Usage" || section.Title == "Commands" {
			insertAt = i + 1
		}
		sections = append(sections, section)
	}
	if found {
		return sections
	}
	options := Section{Title: "Options", Rows: AppendedRows()}
	result := make([]Section, 0, len(sections)+1)
	result = append(result, sections[:insertAt]...)
	result = append(result, options)
	return append(result, sections[insertAt:]...)
}

// Utility returns govna's own page: the shared header followed by the given sections.
func Utility(version string, sections ...Section) Page {
	return Page{Name: "govna", Version: version, Description: "Add and maintain Govna governance files", URL: "github.com/queone/govna", Sections: sections}
}
