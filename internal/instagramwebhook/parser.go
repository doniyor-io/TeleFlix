package instagramwebhook

import (
	"net/url"
	"regexp"
	"strings"
)

var movieCodeTagPattern = regexp.MustCompile(`(?i)(?:^|[^\pL\pN_])#([a-z0-9]+)\b`)

func ExtractMovieCode(caption string) string {
	matches := movieCodeTagPattern.FindStringSubmatch(caption)
	if len(matches) < 2 {
		return ""
	}

	return matches[1]
}

func ExtractShortcode(instagramURL string) string {
	instagramURL = strings.TrimSpace(instagramURL)
	if instagramURL == "" {
		return ""
	}

	parsed, err := url.Parse(instagramURL)
	if err == nil && parsed.Path != "" {
		return shortcodeFromPath(parsed.Path)
	}

	return shortcodeFromPath(instagramURL)
}

func shortcodeFromPath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}

	parts := strings.Split(path, "/")
	for index, part := range parts {
		if (part == "reel" || part == "reels" || part == "p") && index+1 < len(parts) {
			return strings.TrimSpace(parts[index+1])
		}
	}

	return ""
}
