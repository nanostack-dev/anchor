package smtp

import "mime"

// mimeBEncode wraps mime.BEncoding so the rest of the package does not need
// to import mime directly.
func mimeBEncode(s string) string {
	return mime.BEncoding.Encode("utf-8", s)
}
