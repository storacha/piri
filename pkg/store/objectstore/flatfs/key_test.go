package flatfs

import "testing"

var (
	validKeys = []string{
		"foo",
		"1bar1",
		"=emacs-is-king=",
	}
	invalidKeys = []string{
		"foo/bar",
		"FOO/BAR",
		`foo\bar`,
		"foo\000bar",
		"=Vim-IS-KING=",
	}
)

func TestKeyIsValid(t *testing.T) {
	for _, key := range validKeys {
		k := key
		if !keyIsValid(k) {
			t.Errorf("expected key %s to be valid", k)
		}
	}
	for _, key := range invalidKeys {
		k := key
		if keyIsValid(k) {
			t.Errorf("expected key %s to be invalid", k)
		}
	}
}
