package instagramwebhook

import "testing"

func TestExtractMovieCode(t *testing.T) {
	tests := []struct {
		name    string
		caption string
		want    string
	}{
		{
			name:    "alphanumeric code",
			caption: "Daily pick #movie777 watch now",
			want:    "movie777",
		},
		{
			name:    "numeric code",
			caption: "Trailer #9988",
			want:    "9988",
		},
		{
			name:    "uppercase normalized by regex capture",
			caption: "Trailer #CODE1234",
			want:    "CODE1234",
		},
		{
			name:    "ignores missing hashtag",
			caption: "Trailer code1234",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractMovieCode(tt.caption); got != tt.want {
				t.Fatalf("ExtractMovieCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractShortcode(t *testing.T) {
	tests := []struct {
		name string
		link string
		want string
	}{
		{
			name: "reel url",
			link: "https://www.instagram.com/reel/C9abc123/?igsh=demo",
			want: "C9abc123",
		},
		{
			name: "post url",
			link: "https://www.instagram.com/p/XYZ987/",
			want: "XYZ987",
		},
		{
			name: "raw path",
			link: "/reels/abc123/",
			want: "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractShortcode(tt.link); got != tt.want {
				t.Fatalf("ExtractShortcode() = %q, want %q", got, tt.want)
			}
		})
	}
}
