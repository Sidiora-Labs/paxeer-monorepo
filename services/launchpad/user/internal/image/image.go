// Package image holds the pure (dependency-free) image validation helpers for
// user uploads: the MIME allowlist, MIME->extension mapping, and server-side SVG
// sanitisation. Kept pure so it is exhaustively unit-tested (100% target).
// Mirrors media/metadata/internal/image; the discriminators differ (avatar +
// banner for user images vs logo + banner for token images).
package image

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/url"
	"regexp"
	"strings"
)

const (
	svgExtension       = "svg"
	xmlNamespacePrefix = "xmlns"
)

// Image type discriminators stored in users.user_images.image_type.
const (
	TypeAvatar = "avatar"
	TypeBanner = "banner"
)

// allowedMimes is the upload MIME allowlist (parity with the TS allowedMimes).
var allowedMimes = map[string]string{
	"image/webp":    "webp",
	"image/png":     "png",
	"image/svg+xml": svgExtension,
	"image/jpeg":    "jpg",
}

// AllowedMime reports whether mime is an accepted upload content type.
func AllowedMime(mime string) bool {
	_, ok := allowedMimes[strings.ToLower(strings.TrimSpace(mime))]
	return ok
}

// ExtForMime returns the canonical file extension for an allowed MIME, or ""
// when the MIME is not allowed.
func ExtForMime(mime string) string {
	return allowedMimes[strings.ToLower(strings.TrimSpace(mime))]
}

// AllowedMimeList returns the accepted MIME types (for error messages).
func AllowedMimeList() []string {
	return []string{"image/webp", "image/png", "image/svg+xml", "image/jpeg"}
}

// IsValidType reports whether t is a known user image discriminator.
func IsValidType(t string) bool {
	return t == TypeAvatar || t == TypeBanner
}

const (
	svgNamespace   = "http://www.w3.org/2000/svg"
	xlinkNamespace = "http://www.w3.org/1999/xlink"
	xmlNamespace   = "http://www.w3.org/XML/1998/namespace"
)

var (
	svgFragment = regexp.MustCompile(`^#[A-Za-z_][A-Za-z0-9_.-]*$`)
	svgColor    = regexp.MustCompile(`^(#[0-9A-Fa-f]{3,8}|[A-Za-z]+|(?:rgb|rgba|hsl|hsla)\([0-9.,%+ \-]+\))$`)
)

// IsSVGSafe accepts a passive SVG subset using encoding/xml's namespace and
// character-reference decoding. Unknown elements/attributes, CSS, animation,
// external resources, DTDs and processing instructions other than the XML
// declaration are refused. URL attributes may only refer to a local fragment;
// xml:base is never allowed. The original bytes can then be stored unchanged.
func IsSVGSafe(buf []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(buf))
	depth := 0
	seenRoot, closedRoot, declaration := false, false, false
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return seenRoot && closedRoot
		}
		if err != nil {
			return false
		}
		switch t := token.(type) {
		case xml.StartElement:
			if closedRoot || depth >= 128 || !svgElementAllowed(t.Name) {
				return false
			}
			if depth == 0 {
				if t.Name.Local != svgExtension {
					return false
				}
				seenRoot = true
			}
			seenAttrs := make(map[xml.Name]bool, len(t.Attr))
			for _, attr := range t.Attr {
				if seenAttrs[attr.Name] || !svgAttributeAllowed(t.Name.Local, attr) {
					return false
				}
				seenAttrs[attr.Name] = true
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				closedRoot = true
			}
		case xml.CharData:
			if depth == 0 && strings.Trim(string(t), " \t\r\n") != "" {
				return false
			}
		case xml.Directive:
			return false
		case xml.ProcInst:
			if t.Target != "xml" || seenRoot || declaration {
				return false
			}
			declaration = true
		}
	}
}

func svgElementAllowed(name xml.Name) bool {
	if name.Space != "" && name.Space != svgNamespace {
		return false
	}
	switch name.Local {
	case svgExtension, "g", "defs", "title", "desc", "path", "rect", "circle",
		"ellipse", "line", "polyline", "polygon", "text", "tspan",
		"linearGradient", "radialGradient", "stop", "clipPath":
		return true
	}
	return false
}

func svgAttributeAllowed(element string, attr xml.Attr) bool {
	name, value := attr.Name, strings.TrimSpace(attr.Value)
	if name.Space == xmlNamespacePrefix || (name.Space == "" && name.Local == xmlNamespacePrefix) {
		return value == svgNamespace || (name.Space == xmlNamespacePrefix && value == xlinkNamespace)
	}
	if name.Space == xmlNamespace {
		return name.Local == "space" && (value == "default" || value == "preserve")
	}
	if name.Space != "" && (name.Space != xlinkNamespace || name.Local != "href") {
		return false
	}
	switch name.Local {
	case "href":
		return (element == "linearGradient" || element == "radialGradient") && svgLocalURL(value)
	case "fill", "stroke":
		return svgColor.MatchString(value) || svgLocalPaintURL(value)
	case "clip-path":
		return value == "none" || svgLocalPaintURL(value)
	case "color", "stop-color":
		return svgColor.MatchString(value)
	case "id", "version", "viewBox", "preserveAspectRatio", "x", "y", "x1", "y1", "x2", "y2",
		"cx", "cy", "r", "rx", "ry", "width", "height", "d", "points", "transform",
		"opacity", "fill-opacity", "fill-rule", "stroke-opacity", "stroke-width",
		"stroke-linecap", "stroke-linejoin", "stroke-miterlimit", "stroke-dasharray",
		"stroke-dashoffset", "clip-rule", "clipPathUnits", "gradientUnits", "gradientTransform",
		"spreadMethod", "fx", "fy", "fr", "offset", "stop-opacity", "font-size",
		"font-weight", "font-style", "text-anchor", "dominant-baseline", "dx", "dy",
		"rotate", "textLength", "lengthAdjust":
		return true
	}
	return false
}

func svgLocalURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "" && parsed.Host == "" && parsed.Path == "" &&
		parsed.RawQuery == "" && !parsed.ForceQuery && svgFragment.MatchString(value)
}

func svgLocalPaintURL(value string) bool {
	if !strings.HasPrefix(value, "url(") || !strings.HasSuffix(value, ")") {
		return false
	}
	return svgLocalURL(value[4 : len(value)-1])
}

// IsSVG reports whether mime is the SVG content type.
func IsSVG(mime string) bool {
	return strings.EqualFold(strings.TrimSpace(mime), "image/svg+xml")
}
