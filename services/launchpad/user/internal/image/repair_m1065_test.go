package image_test

import (
	"bytes"
	"encoding/xml"
	"io"
	"mime/multipart"
	"net/textproto"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/image"
)

func TestRepairM_1065_CharacterReferences(t *testing.T) {
	cases := map[string]string{
		"audit exploit":          `<svg><a href="java&#x73;cript:alert(1)">open</a></svg>`,
		"decimal reference":      `<svg><a href="java&#115;cript:alert(1)">open</a></svg>`,
		"gradient href":          `<svg><defs><linearGradient href="java&#x73;cript:alert(1)"/></defs></svg>`,
		"encoded colon":          `<svg><defs><linearGradient href="javascript&#58;alert(1)"/></defs></svg>`,
		"URL control characters": `<svg><defs><linearGradient href="java&#9;scr&#10;ipt:alert(1)"/></defs></svg>`,
		"encoded external paint": `<svg><rect fill="url(&#104;ttps://example.invalid/image.svg#paint)"/></svg>`,
		"encoded event":          `<svg onload="&#97;lert(1)"/>`,
		"encoded CSS":            `<svg><rect style="fill:u&#114;l(https://example.invalid/x)"/></svg>`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if image.IsSVGSafe([]byte(input)) {
				t.Fatalf("active content survived XML normalization: %s", input)
			}
		})
	}
}

func TestRepairM_1065_Namespaces(t *testing.T) {
	cases := map[string]string{
		"prefixed script":                   `<s:svg xmlns:s="http://www.w3.org/2000/svg"><s:script>alert(1)</s:script></s:svg>`,
		"prefixed self-closing script":      `<svg xmlns:s="http://www.w3.org/2000/svg"><s:script/></svg>`,
		"foreign object":                    `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><div xmlns="http://www.w3.org/1999/xhtml">active</div></foreignObject></svg>`,
		"foreign namespace":                 `<svg xmlns="http://www.w3.org/2000/svg"><s:rect xmlns:s="https://example.invalid/ns"/></svg>`,
		"default namespace rebound":         `<svg><g xmlns="http://www.w3.org/1999/xhtml"/></svg>`,
		"undeclared prefix":                 `<svg><s:rect/></svg>`,
		"foreign attribute":                 `<svg xmlns:s="https://example.invalid/ns"><rect s:fill="blue"/></svg>`,
		"xlink alias":                       `<svg xmlns:l="http://www.w3.org/1999/xlink"><linearGradient l:href="java&#x73;cript:alert(1)"/></svg>`,
		"xlink external":                    `<svg xmlns:xlink="http://www.w3.org/1999/xlink"><linearGradient xlink:href="https://example.invalid/a.svg#g"/></svg>`,
		"base changes fragment destination": `<svg xml:base="https://example.invalid/a.svg"><linearGradient href="#g"/></svg>`,
		"namespaced event":                  `<svg xmlns:s="http://www.w3.org/2000/svg" s:onload="alert(1)"/>`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if image.IsSVGSafe([]byte(input)) {
				t.Fatalf("unsafe namespace accepted: %s", input)
			}
		})
	}
}

func TestRepairM_1065_ExternalActiveContent(t *testing.T) {
	cases := map[string]string{
		"external script":                   `<svg><script href="https://example.invalid/run.js"/></svg>`,
		"external image":                    `<svg><image href="https://example.invalid/image.svg"/></svg>`,
		"data image":                        `<svg><image href="data:image/svg+xml;base64,PHN2Zy8+"/></svg>`,
		"external use":                      `<svg><use href="//example.invalid/image.svg#x"/></svg>`,
		"external gradient":                 `<svg><linearGradient href="https://example.invalid/image.svg#g"/></svg>`,
		"relative gradient":                 `<svg><linearGradient href="image.svg#g"/></svg>`,
		"protocol relative gradient":        `<svg><linearGradient href="//example.invalid/image.svg#g"/></svg>`,
		"data gradient":                     `<svg><linearGradient href="data:image/svg+xml,x#g"/></svg>`,
		"percent encoded scheme":            `<svg><linearGradient href="java%73cript:alert(1)"/></svg>`,
		"paint server":                      `<svg><rect fill="url(https://example.invalid/image.svg#g)"/></svg>`,
		"stroke server":                     `<svg><path stroke="url(//example.invalid/image.svg#g)"/></svg>`,
		"clip server":                       `<svg><rect clip-path="url(image.svg#clip)"/></svg>`,
		"escaped CSS URL":                   `<svg><rect fill="u\72l(https://example.invalid/image.svg#g)"/></svg>`,
		"CSS paint fallback":                `<svg><rect fill="url(#g) url(https://example.invalid/x)"/></svg>`,
		"CSS import":                        `<svg><style>@import url(https://example.invalid/style.css);</style></svg>`,
		"CSS style attribute":               `<svg style="fill:url(https://example.invalid/x)"/>`,
		"stylesheet processing instruction": `<?xml-stylesheet href="https://example.invalid/style.css"?><svg/>`,
		"animation changes href":            `<svg><linearGradient id="g"><set attributeName="href" to="https://example.invalid/x"/></linearGradient></svg>`,
		"animation changes paint":           `<svg><rect><animate attributeName="fill" values="blue;url(https://example.invalid/x)"/></rect></svg>`,
		"link navigation":                   `<svg><a href="https://example.invalid/">open</a></svg>`,
		"unknown element":                   `<svg><futureActiveElement/></svg>`,
		"unknown attribute":                 `<svg futureActiveAttribute="value"/>`,
		"event attribute":                   `<svg><rect onpointerover="alert(1)"/></svg>`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if image.IsSVGSafe([]byte(input)) {
				t.Fatalf("active or external content accepted: %s", input)
			}
		})
	}
}

func TestRepairM_1065_MalformedDocuments(t *testing.T) {
	cases := map[string]string{
		"empty":                         "",
		"not XML":                       "a plain string",
		"HTML document":                 `<html><svg/></html>`,
		"non SVG root":                  `<rect/>`,
		"unfinished":                    `<svg><rect/>`,
		"mismatched tags":               `<svg><g></svg>`,
		"multiple roots":                `<svg/><svg/>`,
		"trailing bytes":                `<svg/>active`,
		"duplicate attributes":          `<svg width="10" width="20"/>`,
		"duplicate expanded attributes": `<svg xmlns:a="http://www.w3.org/1999/xlink" xmlns:b="http://www.w3.org/1999/xlink"><linearGradient a:href="#a" b:href="#b"/></svg>`,
		"unknown entity":                `<svg><text>&unknown;</text></svg>`,
		"external entity":               `<!DOCTYPE svg [<!ENTITY x SYSTEM "https://example.invalid/x">]><svg><text>&x;</text></svg>`,
		"internal entity":               `<!DOCTYPE svg [<!ENTITY x "text">]><svg><text>&x;</text></svg>`,
		"external DTD":                  `<!DOCTYPE svg SYSTEM "https://example.invalid/svg.dtd"><svg/>`,
		"invalid character reference":   `<svg><text>&#0;</text></svg>`,
		"declaration inside root":       `<svg><?xml version="1.0"?></svg>`,
		"invalid UTF8":                  "<svg>\xff</svg>",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if image.IsSVGSafe([]byte(input)) {
				t.Fatalf("malformed or unsupported XML accepted: %s", input)
			}
		})
	}
}

func TestRepairM_1065_PassiveSVG(t *testing.T) {
	cases := map[string]string{
		"existing namespaced circle": `<svg xmlns="http://www.w3.org/2000/svg"><circle r="5"/></svg>`,
		"existing simple rect":       `<svg><rect width="10" height="10" fill="blue"/></svg>`,
		"prefixed SVG":               `<s:svg xmlns:s="http://www.w3.org/2000/svg"><s:rect width="10" height="20"/></s:svg>`,
		"XML declaration and text":   `<?xml version="1.0" encoding="UTF-8"?><svg xmlns="http://www.w3.org/2000/svg"><title>A &amp; B</title><text xml:space="preserve">&lt;script&gt; onload= javascript: &#9731;</text></svg>`,
		"local paint and clip":       `<svg viewBox="0 0 10 10"><defs><linearGradient id="g"><stop offset="0%" stop-color="#fff"/><stop offset="100%" stop-color="rgb(0, 0, 255)"/></linearGradient><clipPath id="clip"><circle r="5"/></clipPath></defs><rect width="10" height="10" fill="url(#g)" stroke="none" clip-path="url(#clip)"/></svg>`,
		"normalized local paint":     `<svg><defs><linearGradient id="g"/></defs><rect fill="url(&#35;g)"/></svg>`,
		"local gradient alias":       `<svg xmlns:xlink="http://www.w3.org/1999/xlink"><defs><linearGradient id="g"/><radialGradient xlink:href="&#35;g"/></defs></svg>`,
		"local modern gradient":      `<svg><defs><linearGradient id="g"/><linearGradient href="#g"/></defs></svg>`,
		"passive geometry":           `<svg><g transform="translate(1,2)" opacity="0.8"><path d="M0 0 L1 1" stroke="#123456"/><ellipse cx="2" cy="2" rx="1" ry="2"/><line x1="0" y1="0" x2="1" y2="2"/><polyline points="0,0 1,1"/><polygon points="0,0 1,1 1,0"/><text x="1" y="2" font-size="2"><tspan dx="1">safe</tspan></text></g></svg>`,
		"inert comment":              `<svg><!-- <script> onload= javascript: --><desc>passive</desc></svg>`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if !image.IsSVGSafe([]byte(input)) {
				t.Fatalf("passive SVG rejected: %s", input)
			}
		})
	}
}

// Exercise the real XML producer and multipart byte transport used for uploads
// before calling the production MIME and SVG validation boundary.
func TestRepairM_1065_UploadBytes(t *testing.T) {
	for _, tc := range []struct {
		name string
		href string
		want bool
	}{
		{"local", "#paint", true},
		{"external", "https://example.invalid/paint.svg#paint", false},
		{"active", "javascript:alert(1)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var svg bytes.Buffer
			enc := xml.NewEncoder(&svg)
			root := xml.StartElement{Name: xml.Name{Space: "http://www.w3.org/2000/svg", Local: "svg"}}
			gradient := xml.StartElement{Name: xml.Name{Local: "linearGradient"}, Attr: []xml.Attr{{Name: xml.Name{Local: "href"}, Value: tc.href}}}
			for _, token := range []xml.Token{root, gradient, gradient.End(), root.End()} {
				if err := enc.EncodeToken(token); err != nil {
					t.Fatal(err)
				}
			}
			if err := enc.Flush(); err != nil {
				t.Fatal(err)
			}
			var wire bytes.Buffer
			writer := multipart.NewWriter(&wire)
			part, err := writer.CreatePart(textproto.MIMEHeader{
				"Content-Disposition": {`form-data; name="avatar"; filename="avatar.svg"`},
				"Content-Type":        {"image/svg+xml"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := part.Write(svg.Bytes()); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			form, err := multipart.NewReader(&wire, writer.Boundary()).ReadForm(1 << 20)
			if err != nil {
				t.Fatal(err)
			}
			defer form.RemoveAll()
			file := form.File["avatar"][0]
			mime := file.Header.Get("Content-Type")
			if !image.AllowedMime(mime) || !image.IsSVG(mime) || image.ExtForMime(mime) != "svg" {
				t.Fatalf("SVG upload did not reach SVG validation: %q", mime)
			}
			body, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			buf, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf, svg.Bytes()) {
				t.Fatal("multipart transport changed the XML producer's bytes")
			}
			if got := image.IsSVGSafe(buf); got != tc.want {
				t.Fatalf("IsSVGSafe(upload bytes) = %v, want %v: %s", got, tc.want, buf)
			}
		})
	}
}
