package application

import (
	"testing"
)

func TestExtractExtension(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		contentType string
		want        string
	}{
		{
			name:        "mp3 from URL",
			url:         "https://example.com/episode.mp3",
			contentType: "",
			want:        ".mp3",
		},
		{
			name:        "m4a from URL",
			url:         "https://example.com/podcast.m4a",
			contentType: "",
			want:        ".m4a",
		},
		{
			name:        "aac from URL",
			url:         "https://example.com/episode.aac",
			contentType: "",
			want:        ".aac",
		},
		{
			name:        "ogg from URL",
			url:         "https://example.com/episode.ogg",
			contentType: "",
			want:        ".ogg",
		},
		{
			name:        "webm from URL",
			url:         "https://example.com/episode.webm",
			contentType: "",
			want:        ".webm",
		},
		{
			name:        "no extension in URL falls back to content-type",
			url:         "https://example.com/download/12345",
			contentType: "audio/mpeg",
			want:        ".mp3",
		},
		{
			name:        "content-type audio/mp4",
			url:         "https://example.com/feed/abc",
			contentType: "audio/mp4",
			want:        ".m4a",
		},
		{
			name:        "content-type audio/x-m4a",
			url:         "https://example.com/feed/abc",
			contentType: "audio/x-m4a",
			want:        ".m4a",
		},
		{
			name:        "content-type audio/ogg",
			url:         "https://example.com/feed/abc",
			contentType: "audio/ogg",
			want:        ".ogg",
		},
		{
			name:        "content-type audio/wav",
			url:         "https://example.com/feed/abc",
			contentType: "audio/wav",
			want:        ".wav",
		},
		{
			name:        "content-type audio/flac",
			url:         "https://example.com/feed/abc",
			contentType: "audio/flac",
			want:        ".flac",
		},
		{
			name:        "content-type audio/webm",
			url:         "https://example.com/feed/abc",
			contentType: "audio/webm",
			want:        ".webm",
		},
		{
			name:        "content-type audio/opus",
			url:         "https://example.com/feed/abc",
			contentType: "audio/opus",
			want:        ".opus",
		},
		{
			name:        "unknown content-type falls back to .audio",
			url:         "https://example.com/feed/abc",
			contentType: "application/octet-stream",
			want:        ".audio",
		},
		{
			name:        "empty content-type falls back to .audio",
			url:         "https://example.com/feed/abc",
			contentType: "",
			want:        ".audio",
		},
		{
			name:        "URL with params doesn't confuse extension detection",
			url:         "https://example.com/episode.mp3?token=abc123",
			contentType: "",
			want:        ".mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractExtension(tt.url, tt.contentType)
			if got != tt.want {
				t.Errorf("extractExtension(%q, %q) = %q, want %q", tt.url, tt.contentType, got, tt.want)
			}
		})
	}
}

func TestSafeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World!", "Hello_World"},
		{"  spaces  ", "spaces"},
		{"multiple   spaces", "multiple___spaces"},
		{"special!@#$chars", "special____chars"},
		{"dots.in.name", "dots.in.name"},
		{"hyphens-are-ok", "hyphens-are-ok"},
		{"under_scores_ok", "under_scores_ok"},
		{"UPPERCASE", "UPPERCASE"},
		{"a b c d e f g h i j k l m n o p q r s t u v w x y z a b c d e f g h i j k l m n o p q r s t u v w x y z", "a_b_c_d_e_f_g_h_i_j_k_l_m_n_o_p_q_r_s_t_u_v_w_x_y_"},
		{".startswithdot", "startswithdot"},
		{"endswithspace ", "endswithspace"},
		{"", "download"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := safeFilename(tt.input)
			if got != tt.want {
				t.Errorf("safeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
