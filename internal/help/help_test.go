package help

import (
	"strings"
	"testing"
)

func samplePage() Page {
	return Page{
		Name:        "widget",
		Version:     "1.2.3",
		Description: "Shape widgets from the command line",
		URL:         "example.com/widget",
		Sections: []Section{
			{Title: "Usage", Rows: []Row{{Form: "widget COMMAND [options]"}}},
			{Title: "Commands", Rows: []Row{
				{Form: "make", Meaning: "write a widget"},
				{Form: "inspect", Meaning: "read a widget"},
			}, Note: "Run 'widget COMMAND -h' for command-specific options."},
		},
	}
}

func TestRenderPlainLayoutCreatesOptions(t *testing.T) {
	want := "widget v1.2.3\n" +
		"Shape widgets from the command line\n" +
		"example.com/widget\n" +
		"\n" +
		"Usage\n" +
		"  widget COMMAND [options]\n" +
		"\n" +
		"Commands\n" +
		"  make     write a widget\n" +
		"  inspect  read a widget\n" +
		"\n" +
		"  Run 'widget COMMAND -h' for command-specific options.\n" +
		"\n" +
		"Options\n" +
		"  -v, --version   print executable version\n" +
		"  -h, -?, --help  show this help\n"
	if got := samplePage().Render(false); got != want {
		t.Fatalf("plain render mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderAppendsRowsToExistingOptions(t *testing.T) {
	page := Page{Name: "widget", Version: "1.2.3", Description: "Shape widgets", URL: "example.com/widget", Sections: []Section{
		{Title: "Usage", Rows: []Row{{Form: "widget make [options] TARGET"}}, Note: "Write one widget to TARGET.\nExisting files stay."},
		{Title: "Options", Rows: []Row{{Form: "-s, --size N", Meaning: "widget size"}, {Form: "-f, --force", Meaning: "overwrite TARGET"}}},
		{Title: "Examples", Rows: []Row{{Form: "widget make -s 3 out"}}},
	}}
	want := "widget v1.2.3\nShape widgets\nexample.com/widget\n\n" +
		"Usage\n  widget make [options] TARGET\n\n  Write one widget to TARGET.\n  Existing files stay.\n\n" +
		"Options\n  -s, --size N    widget size\n  -f, --force     overwrite TARGET\n  -v, --version   print executable version\n  -h, -?, --help  show this help\n\n" +
		"Examples\n  widget make -s 3 out\n"
	if got := page.Render(false); got != want {
		t.Fatalf("options render mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
	if got := page.Sections[1].Rows; len(got) != 2 {
		t.Fatalf("render mutated the page's Options rows: %v", got)
	}
}

func TestRenderKeepsEveryLineInsideASection(t *testing.T) {
	lines := strings.Split(strings.TrimSuffix(samplePage().Render(false), "\n"), "\n")
	if lines[3] != "" {
		t.Fatalf("line four must be blank: %q", lines[3])
	}
	headings := 0
	for i, line := range lines[4:] {
		switch {
		case line == "":
			if i == 0 || lines[4+i-1] == "" {
				t.Fatalf("consecutive blank lines before %d", i)
			}
		case strings.HasPrefix(line, indent):
		case strings.ContainsAny(line, " :"):
			t.Fatalf("line outside a section: %q", line)
		default:
			headings++
			if i > 0 && lines[4+i-1] != "" {
				t.Fatalf("heading %q must follow one blank line", line)
			}
		}
	}
	if headings != 3 || lines[len(lines)-1] == "" {
		t.Fatalf("unexpected headings=%d or trailing blank line", headings)
	}
}

func TestRenderColorUsesOneSequencePerSpan(t *testing.T) {
	got := samplePage().Render(true)
	for _, want := range []string{
		"\x1b[1;38;5;231mwidget\x1b[0m v1.2.3\n",
		"\x1b[38;5;245mShape widgets from the command line\x1b[0m\n",
		"\x1b[38;5;242mexample.com/widget\x1b[0m\n",
		"\n\x1b[1;38;5;231mUsage\x1b[0m\n",
		"\n\x1b[1;38;5;231mCommands\x1b[0m\n",
		"\n\x1b[1;38;5;231mOptions\x1b[0m\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("colored render lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\x1b[1m") {
		t.Fatalf("bold must not be a separate sequence:\n%s", got)
	}
	if starts := strings.Count(got, "\x1b["); starts != 12 {
		t.Fatalf("expected 6 spans (12 sequences), got %d sequences", starts)
	}
	if strings.Count(got, "\x1b[0m") != 6 {
		t.Fatalf("expected 6 resets:\n%s", got)
	}
}

func TestRenderWithoutColorHasNoEscape(t *testing.T) {
	if got := samplePage().Render(false); strings.Contains(got, "\x1b") {
		t.Fatalf("plain render carries an escape: %q", got)
	}
}

func TestEnabled(t *testing.T) {
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) {
			value, ok := values[key]
			return value, ok
		}
	}
	for _, tc := range []struct {
		name     string
		terminal bool
		env      map[string]string
		want     bool
	}{
		{"non-terminal", false, map[string]string{"TERM": "xterm-256color"}, false},
		{"NO_COLOR empty", true, map[string]string{"NO_COLOR": "", "TERM": "xterm-256color"}, false},
		{"TERM dumb", true, map[string]string{"TERM": "dumb", "COLORTERM": "truecolor"}, false},
		{"unsupported", true, map[string]string{"TERM": "xterm"}, false},
		{"truecolor", true, map[string]string{"COLORTERM": "truecolor"}, true},
		{"24bit", true, map[string]string{"COLORTERM": "24bit"}, true},
		{"256color", true, map[string]string{"TERM": "screen-256color"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Enabled(tc.terminal, lookup(tc.env)); got != tc.want {
				t.Fatalf("Enabled=%v want %v", got, tc.want)
			}
		})
	}
	if Enabled(true, nil) {
		t.Fatal("nil lookup must disable color")
	}
}
