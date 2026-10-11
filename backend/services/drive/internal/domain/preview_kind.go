package domain

import (
	"path"
	"strings"
)

// previewClass is how a file can be shown.
type previewClass int

const (
	previewUnsupported previewClass = iota
	// previewNative: the browser renders the original itself.
	previewNative
	// previewSheet: a spreadsheet the client reads itself and shows as a table.
	previewSheet
	// previewOffice: an Office or OpenDocument file the server turns into PDF.
	previewOffice
)

// officeExt are the extensions LibreOffice converts.
var officeExt = map[string]string{
	"doc":  "application/msword",
	"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"ppt":  "application/vnd.ms-powerpoint",
	"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	"odt":  "application/vnd.oasis.opendocument.text",
	"odp":  "application/vnd.oasis.opendocument.presentation",
	"rtf":  "application/rtf",
}

// sheetExt are the spreadsheets, shown as a table rather than converted.
var sheetExt = map[string]string{
	"xls":  "application/vnd.ms-excel",
	"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"ods":  "application/vnd.oasis.opendocument.spreadsheet",
}

var sheetMimes = map[string]bool{
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	"application/vnd.oasis.opendocument.spreadsheet":                    true,
}

// officeMimeExt names the extension for an Office MIME type, for a file whose
// name does not carry one.
var officeMimeExt = map[string]string{
	"application/msword": "doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"application/vnd.ms-excel": "xls",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.ms-powerpoint":                                             "ppt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/vnd.oasis.opendocument.text":                                   "odt",
	"application/vnd.oasis.opendocument.presentation":                           "odp",
	"application/rtf": "rtf",
	"text/rtf":        "rtf",
}

// nativeExt gives the type of a file whose recorded MIME type says nothing
// (empty or octet-stream) but whose name does. Fixed here, not read from the
// host's MIME database, so the answer does not depend on the machine.
var nativeExt = map[string]string{
	"pdf": "application/pdf", "csv": "text/csv", "json": "application/json",
	"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg", "gif": "image/gif",
	"webp": "image/webp", "avif": "image/avif", "bmp": "image/bmp", "svg": "image/svg+xml", "html": "text/html", "htm": "text/html", "xhtml": "application/xhtml+xml",
	"mp4": "video/mp4", "webm": "video/webm", "mov": "video/quicktime", "ogv": "video/ogg",
	"mp3": "audio/mpeg", "wav": "audio/wav", "ogg": "audio/ogg", "m4a": "audio/mp4", "flac": "audio/flac",
	"txt": "text/plain", "md": "text/markdown", "log": "text/plain", "xml": "text/xml",
	"yaml": "text/yaml", "yml": "text/yaml", "tsv": "text/tab-separated-values",
	"go": "text/plain", "js": "text/plain", "ts": "text/plain", "tsx": "text/plain", "jsx": "text/plain",
	"py": "text/plain", "java": "text/plain", "rs": "text/plain", "c": "text/plain", "h": "text/plain",
	"cpp": "text/plain", "sh": "text/plain", "sql": "text/plain", "css": "text/plain", "toml": "text/plain",
}

// scriptCapable are the types a browser runs script from when it opens them as
// a page. They are previewed as source text: the client is told text/plain, so
// it fetches and shows them and never navigates to or embeds them.
var scriptCapable = map[string]bool{
	"text/html": true, "application/xhtml+xml": true, "image/svg+xml": true,
	"text/xml": true, "application/xml": true, "text/xsl": true, "application/xslt+xml": true,
	"text/xsd": true, "application/mathml+xml": true,
}

func nativeMime(m string) string {
	if scriptCapable[m] || strings.HasSuffix(m, "+xml") && !strings.HasPrefix(m, "application/vnd.") {
		return "text/plain"
	}
	return m
}

func isGenericMime(m string) bool {
	return m == "" || m == "application/octet-stream" || m == "binary/octet-stream"
}

// classifyPreview decides how a file named name with the recorded MIME type
// mimeType is shown. It returns the type the client should treat the content
// as, and for an Office file the file name to hand the converter (which needs
// the right extension).
func classifyPreview(name, mimeType string) (class previewClass, mime, convName string) {
	m := strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")

	// A spreadsheet MIME on a .csv is the common mislabel of an export: it is
	// plain text, and converting it would only produce a worse rendering.
	if ext == "csv" {
		return previewNative, "text/csv", ""
	}
	if se, ok := sheetExt[ext]; ok {
		if isGenericMime(m) {
			m = se
		}
		return previewSheet, m, ""
	}
	if sheetMimes[m] {
		return previewSheet, m, ""
	}
	if oe, ok := officeExt[ext]; ok {
		if isGenericMime(m) {
			m = oe
		}
		return previewOffice, m, name
	}
	if ox, ok := officeMimeExt[m]; ok {
		return previewOffice, m, name + "." + ox
	}
	if isGenericMime(m) {
		if nm, ok := nativeExt[ext]; ok {
			return previewNative, nativeMime(nm), ""
		}
		return previewUnsupported, m, ""
	}
	switch {
	case strings.HasPrefix(m, "image/"), strings.HasPrefix(m, "video/"),
		strings.HasPrefix(m, "audio/"), strings.HasPrefix(m, "text/"),
		m == "application/pdf", m == "application/json", m == "application/csv",
		scriptCapable[m], strings.HasSuffix(m, "+xml") && !strings.HasPrefix(m, "application/vnd."):
		return previewNative, nativeMime(m), ""
	}
	return previewUnsupported, m, ""
}
