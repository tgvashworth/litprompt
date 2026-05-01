package build

import (
	"reflect"
	"testing"
)

func TestSubstituteVars(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		vars        map[string]string
		want        string
		wantMissing []string
	}{
		{
			name:  "simple inline",
			input: "Bot is @[`U1234`](#BOT_ID).\n",
			vars:  map[string]string{"BOT_ID": "U999"},
			want:  "Bot is U999.\n",
		},
		{
			name:  "two on one line",
			input: "@[a](#A) and @[b](#B)\n",
			vars:  map[string]string{"A": "alpha", "B": "beta"},
			want:  "alpha and beta\n",
		},
		{
			name:  "directive alone on a line",
			input: "@[`U1234`](#BOT_ID)\n",
			vars:  map[string]string{"BOT_ID": "U999"},
			want:  "U999\n",
		},
		{
			name:  "preserved inside fenced code block",
			input: "before\n\n```\n@[`U1234`](#BOT_ID)\n```\n\nafter @[`U1234`](#BOT_ID)\n",
			vars:  map[string]string{"BOT_ID": "U999"},
			want:  "before\n\n```\n@[`U1234`](#BOT_ID)\n```\n\nafter U999\n",
		},
		{
			name:  "preserved inside tilde fenced block",
			input: "~~~\n@[X](#FOO)\n~~~\n@[X](#FOO)\n",
			vars:  map[string]string{"FOO": "value"},
			want:  "~~~\n@[X](#FOO)\n~~~\nvalue\n",
		},
		{
			name:  "preserved inside inline code span",
			input: "Inside `@[X](#BOT_ID)` outside @[X](#BOT_ID)\n",
			vars:  map[string]string{"BOT_ID": "U999"},
			want:  "Inside `@[X](#BOT_ID)` outside U999\n",
		},
		{
			name:  "directive with backtick-wrapped placeholder still substitutes",
			input: "Bot @[`U1234`](#BOT_ID) here\n",
			vars:  map[string]string{"BOT_ID": "U999"},
			want:  "Bot U999 here\n",
		},
		{
			name:        "missing var collected",
			input:       "@[x](#MISSING)\n",
			vars:        map[string]string{},
			want:        "@[x](#MISSING)\n",
			wantMissing: []string{"MISSING"},
		},
		{
			name:        "multiple missing sorted and deduped",
			input:       "@[a](#ZULU) @[b](#ALPHA) @[c](#MIKE) @[d](#ALPHA)\n",
			vars:        map[string]string{},
			want:        "@[a](#ZULU) @[b](#ALPHA) @[c](#MIKE) @[d](#ALPHA)\n",
			wantMissing: []string{"ALPHA", "MIKE", "ZULU"},
		},
		{
			name:        "nil vars treats every directive as missing",
			input:       "@[x](#FOO) @[y](#BAR)\n",
			vars:        nil,
			want:        "@[x](#FOO) @[y](#BAR)\n",
			wantMissing: []string{"BAR", "FOO"},
		},
		{
			name:  "empty value substitutes to empty string",
			input: "Prefix:@[d](#EMPTY)End\n",
			vars:  map[string]string{"EMPTY": ""},
			want:  "Prefix:End\n",
		},
		{
			name:  "no directives passes through",
			input: "Hello world.\n",
			vars:  map[string]string{},
			want:  "Hello world.\n",
		},
		{
			name:  "lowercase var name not matched (falls through)",
			input: "@[X](#bot_id)\n",
			vars:  map[string]string{"BOT_ID": "U999"},
			want:  "@[X](#bot_id)\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, missing := SubstituteVars(tc.input, tc.vars)
			if got != tc.want {
				t.Errorf("output\n  got:  %q\n  want: %q", got, tc.want)
			}
			if len(missing) == 0 && len(tc.wantMissing) == 0 {
				return
			}
			if !reflect.DeepEqual(missing, tc.wantMissing) {
				t.Errorf("missing = %v, want %v", missing, tc.wantMissing)
			}
		})
	}
}
