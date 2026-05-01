package varsfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:  "simple key=value",
			input: "FOO=bar\n",
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "blank lines and full-line comments ignored",
			input: "\n# comment\nFOO=bar\n\n# another\nBAZ=qux\n",
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:  "double-quoted preserves spaces and # signs",
			input: `FOO="hello # world"` + "\n",
			want:  map[string]string{"FOO": "hello # world"},
		},
		{
			name:  "single-quoted preserves contents",
			input: `FOO='hello # world'` + "\n",
			want:  map[string]string{"FOO": "hello # world"},
		},
		{
			name:  "unquoted value strips trailing comment",
			input: "FOO=bar # trailing\n",
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "unquoted value strips trailing whitespace",
			input: "FOO=bar   \n",
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "empty value is empty string",
			input: "FOO=\n",
			want:  map[string]string{"FOO": ""},
		},
		{
			name:  "CRLF line endings",
			input: "FOO=bar\r\nBAZ=qux\r\n",
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:  "duplicate key last wins",
			input: "FOO=first\nFOO=second\n",
			want:  map[string]string{"FOO": "second"},
		},
		{
			name:  "leading whitespace before key allowed",
			input: "  FOO=bar\n",
			want:  map[string]string{"FOO": "bar"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.input)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Parse err = %v, wantErr %v", err, tc.wantErr)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("Parse got %d keys, want %d (got=%v want=%v)", len(got), len(tc.want), got, tc.want)
			}
			for k, v := range tc.want {
				if got[k] != v {
					t.Errorf("Parse[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestLoad_mergesFilesLaterWins(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.env")
	b := filepath.Join(dir, "b.env")

	if err := os.WriteFile(a, []byte("FOO=first\nBAR=untouched\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("FOO=second\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := Load([]string{a, b})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got["FOO"] != "second" {
		t.Errorf("FOO = %q, want second", got["FOO"])
	}
	if got["BAR"] != "untouched" {
		t.Errorf("BAR = %q, want untouched", got["BAR"])
	}
}

func TestLoad_missingFile(t *testing.T) {
	_, err := Load([]string{"/nonexistent/path/to/vars.env"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
