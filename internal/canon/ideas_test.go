package canon

import "testing"

// TestSpaceIdeas checks that spacing leaves exactly one blank line between
// adjacent IE items, keeps every item's text and order, changes no other line,
// and returns an already-spaced plan unchanged.
func TestSpaceIdeas(t *testing.T) {
	const head = "# Plan\n\n## Ideas To Explore\n\nIdeas captured for future reference.\n\n"
	tests := []struct {
		name    string
		in      string
		want    string
		changed bool
	}{
		{
			name:    "compact five-item list",
			in:      head + "- IE1: One.\n- IE2: Two.\n- IE3: Three.\n- IE4: Four.\n- IE5: Five.\n",
			want:    head + "- IE1: One.\n\n- IE2: Two.\n\n- IE3: Three.\n\n- IE4: Four.\n\n- IE5: Five.\n",
			changed: true,
		},
		{
			name: "already spaced list",
			in:   head + "- IE1: One.\n\n- IE2: Two.\n",
			want: head + "- IE1: One.\n\n- IE2: Two.\n",
		},
		{
			name:    "partly spaced list",
			in:      head + "- IE1: One.\n\n- IE2: Two.\n- IE3: Three.\n",
			want:    head + "- IE1: One.\n\n- IE2: Two.\n\n- IE3: Three.\n",
			changed: true,
		},
		{
			name:    "several blank lines between items",
			in:      head + "- IE1: One.\n\n\n\n- IE2: Two.\n",
			want:    head + "- IE1: One.\n\n- IE2: Two.\n",
			changed: true,
		},
		{
			name:    "continuation lines stay with their item",
			in:      head + "- IE1: One,\n  continued.\n\tTabbed.\n- IE2: Two.\n",
			want:    head + "- IE1: One,\n  continued.\n\tTabbed.\n\n- IE2: Two.\n",
			changed: true,
		},
		{
			name:    "CRLF line endings",
			in:      "# Plan\r\n\r\n- IE1: One.\r\n- IE2: Two.\r\n",
			want:    "# Plan\r\n\r\n- IE1: One.\r\n\r\n- IE2: Two.\r\n",
			changed: true,
		},
		{
			name:    "trailing blank lines stay",
			in:      head + "- IE1: One.\n- IE2: Two.\n\n\n",
			want:    head + "- IE1: One.\n\n- IE2: Two.\n\n\n",
			changed: true,
		},
		{
			name:    "last item without final newline",
			in:      head + "- IE1: One.\n- IE2: Two.",
			want:    head + "- IE1: One.\n\n- IE2: Two.",
			changed: true,
		},
		{
			name: "other line between items",
			in:   head + "- IE1: One.\nA note.\n- IE2: Two.\n",
			want: head + "- IE1: One.\nA note.\n- IE2: Two.\n",
		},
		{
			name:    "section after the list stays",
			in:      head + "- IE1: One.\n- IE2: Two.\n\n## Later\n\n- Not an idea.\n",
			want:    head + "- IE1: One.\n\n- IE2: Two.\n\n## Later\n\n- Not an idea.\n",
			changed: true,
		},
		{
			name: "no IE items",
			in:   "# Plan\n\n- A.\n- B.\n",
			want: "# Plan\n\n- A.\n- B.\n",
		},
		{
			name: "empty file",
			in:   "",
			want: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, changed := SpaceIdeas([]byte(test.in))
			if string(got) != test.want || changed != test.changed {
				t.Fatalf("SpaceIdeas() = %q, %v; want %q, %v", got, changed, test.want, test.changed)
			}
		})
	}
}
