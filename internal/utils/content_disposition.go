package utils

import (
	"regexp"
	"strconv"
)

var (
	reForwardedHost  = regexp.MustCompile(`host="?([^;"]+)`)
	reForwardedProto = regexp.MustCompile(`proto=(https?)`)
	reMimeType       = regexp.MustCompile(`^[a-z]+\/[a-z0-9\-\+\.]+$`)
	// We only allow certain URL-safe characters in upload IDs. URL-safe in this means
	// that their are allowed in a URI's path component according to RFC 3986.
	// See https://datatracker.ietf.org/doc/html/rfc3986#section-3.3
	reValidUploadId = regexp.MustCompile(`^[A-Za-z0-9\-._~%!$'()*+,;=/:@]*$`)
)

var mimeInlineBrowserWhitelist = map[string]struct{}{
	"text/plain":       {},
	"application/json": {},

	"image/png":  {},
	"image/jpeg": {},
	"image/gif":  {},
	"image/bmp":  {},
	"image/webp": {},

	"audio/wave":      {},
	"audio/wav":       {},
	"audio/mp3":       {},
	"audio/x-wav":     {},
	"audio/x-pn-wav":  {},
	"audio/webm":      {},
	"video/webm":      {},
	"audio/ogg":       {},
	"video/ogg":       {},
	"application/ogg": {},
}

// filterContentType returns the values for the Content-Type and
// Content-Disposition headers for a given upload. These values should be used
// in responses for GET requests to ensure that only non-malicious file types
// are shown directly in the browser. It will extract the file name and type
// from the "fileame" and "filetype".
// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition
func FilterContentType(filetype, filename string) (contentType string, contentDisposition string) {

	if reMimeType.MatchString(filetype) {
		// If the filetype from metadata is well formed, we forward use this
		// for the Content-Type header. However, only whitelisted mime types
		// will be allowed to be shown inline in the browser
		contentType = filetype
		if _, isWhitelisted := mimeInlineBrowserWhitelist[filetype]; isWhitelisted {
			contentDisposition = "inline"
		} else {
			contentDisposition = "attachment"
		}
	} else {
		// If the filetype from the metadata is not well formed, we use a
		// default type and force the browser to download the content.
		contentType = "application/octet-stream"
		contentDisposition = "attachment"
	}

	// Add a filename to Content-Disposition if one is available in the metadata
	contentDisposition += ";filename=" + strconv.Quote(filename)

	return contentType, contentDisposition
}
