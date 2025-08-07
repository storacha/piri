package lib

import (
	"testing"
)

func TestParseAndNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "empty string returns error",
			input:   "",
			want:    "",
			wantErr: true,
		},
		{
			name:    "simple URL without trailing slash",
			input:   "http://example.com",
			want:    "http://example.com",
			wantErr: false,
		},
		{
			name:    "URL with single trailing slash",
			input:   "http://example.com/",
			want:    "http://example.com",
			wantErr: false,
		},
		{
			name:    "URL with multiple trailing slashes",
			input:   "http://example.com///",
			want:    "http://example.com",
			wantErr: false,
		},
		{
			name:    "URL with path and no trailing slash",
			input:   "http://example.com/path",
			want:    "http://example.com/path",
			wantErr: false,
		},
		{
			name:    "URL with path and trailing slash",
			input:   "http://example.com/path/",
			want:    "http://example.com/path",
			wantErr: false,
		},
		{
			name:    "URL with nested path and trailing slashes",
			input:   "http://example.com/path/to/resource///",
			want:    "http://example.com/path/to/resource",
			wantErr: false,
		},
		{
			name:    "URL with query parameters",
			input:   "http://example.com/path?foo=bar",
			want:    "http://example.com/path?foo=bar",
			wantErr: false,
		},
		{
			name:    "URL with query parameters and trailing slash",
			input:   "http://example.com/path/?foo=bar",
			want:    "http://example.com/path?foo=bar",
			wantErr: false,
		},
		{
			name:    "URL with fragment",
			input:   "http://example.com/path#section",
			want:    "http://example.com/path#section",
			wantErr: false,
		},
		{
			name:    "URL with fragment and trailing slash",
			input:   "http://example.com/path/#section",
			want:    "http://example.com/path#section",
			wantErr: false,
		},
		{
			name:    "URL with port",
			input:   "http://example.com:8080/",
			want:    "http://example.com:8080",
			wantErr: false,
		},
		{
			name:    "HTTPS URL",
			input:   "https://example.com/secure/",
			want:    "https://example.com/secure",
			wantErr: false,
		},
		{
			name:    "URL with authentication",
			input:   "http://user:pass@example.com/",
			want:    "http://user:pass@example.com",
			wantErr: false,
		},
		{
			name:    "invalid URL",
			input:   "://invalid",
			want:    "",
			wantErr: true,
		},
		{
			name:    "URL with only slash as path",
			input:   "http://example.com/",
			want:    "http://example.com",
			wantErr: false,
		},
		{
			name:    "file URL",
			input:   "file:///path/to/file/",
			want:    "file:///path/to/file",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAndNormalizeURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAndNormalizeURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got.String() != tt.want {
				t.Errorf("ParseAndNormalizeURL() = %v, want %v", got.String(), tt.want)
			}
		})
	}
}
