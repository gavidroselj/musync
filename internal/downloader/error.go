package downloader

import "strings"

func IsErrVideoUnavailable(err error) bool {
	if strings.Contains(err.Error(), "Video unavailable.") {
		return true
	}

	if strings.Contains(err.Error(), "Private video.") {
		return true
	}

	return false
}
