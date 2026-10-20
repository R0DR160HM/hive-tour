package hive

import (
	"embed"
	"fmt"
	"hash/fnv"
	"hash/maphash"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type UiView struct {
	Kind     string
	Attrs    []UiAttr
	Children []UiView
	Text    string
	Rows    Table
	Options []string
	Flag    bool
	Marks   []UiMark

	Cuts []int32
}

type UiAttr struct {
	Kind string
	Num  int

Word string
Flag bool
Nums []float64

	heard *uiHeard
}

type uiHeard struct {
	Msg    any
	FnStr  func(string) any
	FnInt  func(int) any
	FnBool func(bool) any
	FnLook func(float64, float64) any
	FnPad  func(int, string, float64) any
	FnSize func(int, int) any
	FnEdit func(int, int, string) any
	FnDrag func(int, int, bool) any
}

func uiNode(kind string, attrs []UiAttr, children []UiView) UiView {
	return UiView{Kind: kind, Attrs: attrs, Children: children}
}

func UiRow(attrs []UiAttr, children []UiView) UiView {
	return uiNode("row", attrs, children)
}

func UiColumn(attrs []UiAttr, children []UiView) UiView {
	return uiNode("column", attrs, children)
}

func UiSpacer() UiView { return UiView{Kind: "spacer"} }

func UiOverlay(attrs []UiAttr, child UiView) UiView {
	return uiNode("overlay", attrs, []UiView{child})
}

func UiText(attrs []UiAttr, content string) UiView {
	return UiView{Kind: "text", Attrs: attrs, Text: content}
}

func UiImage(attrs []UiAttr, src string, alt string) UiView {
	return UiView{Kind: "image", Attrs: attrs, Text: src, Options: []string{alt}}
}

func UiIconOf(attrs []UiAttr, name UiIcon) UiView {
	return UiView{Kind: "icon", Attrs: attrs, Text: string(name)}
}

func UiLink(attrs []UiAttr, href string, label string) UiView {
	return UiView{Kind: "link", Attrs: attrs, Text: label, Options: []string{href}}
}

type UiMark struct {
	Kind  string
	Attrs []UiAttr
	Nums  []float64
	Text  string
}

func UiDot(attrs []UiAttr, x float64, y float64, radius float64) UiMark {
	return UiMark{Kind: "dot", Attrs: attrs, Nums: []float64{x, y, radius}}
}

func UiBar(attrs []UiAttr, x float64, y float64, width float64, height float64) UiMark {
	return UiMark{Kind: "bar", Attrs: attrs, Nums: []float64{x, y, width, height}}
}

func UiPath(attrs []UiAttr, points []float64) UiMark {
	return UiMark{Kind: "path", Attrs: attrs, Nums: points}
}

func UiPatch(attrs []UiAttr, points []float64) UiMark {
	return UiMark{Kind: "patch", Attrs: attrs, Nums: points}
}

func UiWords(attrs []UiAttr, x float64, y float64, height float64, text string) UiMark {
	return UiMark{Kind: "words", Attrs: attrs, Nums: []float64{x, y, height}, Text: text}
}

func UiCanvas(attrs []UiAttr, marks []UiMark) UiView {
	return UiView{Kind: "canvas", Attrs: attrs, Marks: marks}
}

type UiRun = UiMark

func UiCode(attrs []UiAttr, text string, runs []UiRun) UiView {
	return UiView{Kind: "code", Attrs: attrs, Text: text, Marks: runs}
}

func uiRunOf(kind string, from int, to int, tone UiTone) UiRun {
	return UiMark{Kind: kind, Nums: []float64{float64(from), float64(to)}, Text: string(tone)}
}

func UiInk(from int, to int, tone UiTone) UiRun      { return uiRunOf("ink", from, to, tone) }
func UiShade(from int, to int, tone UiTone) UiRun    { return uiRunOf("shade", from, to, tone) }
func UiSquiggle(from int, to int, tone UiTone) UiRun { return uiRunOf("squiggle", from, to, tone) }
func UiMarker(line int, tone UiTone) UiRun           { return uiRunOf("marker", line, line, tone) }
func UiUnderline(from int, to int, tone UiTone) UiRun { return uiRunOf("underline", from, to, tone) }
func UiBand(line int, tone UiTone) UiRun             { return uiRunOf("band", line, line, tone) }
func UiNote(line int, text string, tone UiTone) UiRun {
	r := uiRunOf("note", line, line, tone)
	r.Attrs = []UiAttr{uiStr("note", text)}
	return r
}

// What a note says: its text, kept beside the tone.
func uiNoteText(r UiRun) string {
	if a, ok := uiFind(r.Attrs, "note"); ok {
		return a.Word
	}
	return ""
}

func UiButton(attrs []UiAttr, label string) UiView {
	return UiView{Kind: "button", Attrs: attrs, Text: label}
}

func UiInput(attrs []UiAttr, value string) UiView {
	return UiView{Kind: "input", Attrs: attrs, Text: value}
}

func UiTextarea(attrs []UiAttr, value string) UiView {
	return UiView{Kind: "textarea", Attrs: attrs, Text: value}
}

func UiCheckbox(attrs []UiAttr, label string, checked bool) UiView {
	return UiView{Kind: "checkbox", Attrs: attrs, Text: label, Flag: checked}
}

func UiSelect(attrs []UiAttr, options []string, chosen string) UiView {
	return UiView{Kind: "select", Attrs: attrs, Options: options, Text: chosen}
}

func UiTable(attrs []UiAttr, rows Table) UiView {
	return UiView{Kind: "table", Attrs: attrs, Rows: rows}
}

func UiSpinner(attrs []UiAttr) UiView { return UiView{Kind: "spinner", Attrs: attrs} }

func UiNone() UiView { return UiView{Kind: "none"} }

type UiAlign string
type UiJustify string
type UiTextSize string
type UiTone string
type UiAxis string
type UiAnchor string
type UiInputKind string
type UiIcon string

type UiSurface string

type UiVoice string

func uiToneColour(e string) (string, bool) {
	if strings.HasPrefix(e, "#") || strings.HasPrefix(e, "rgba(") {
		return e, true
	}
	return "", false
}

func UiToneHex(hex string) UiTone {
	n := len(hex)
	if n != 4 && n != 5 && n != 7 && n != 9 {
		return UiTone("Normal")
	}
	if hex[0] != '#' {
		return UiTone("Normal")
	}
	for i := 1; i < n; i++ {
		c := hex[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return UiTone("Normal")
		}
	}
	return UiTone(hex)
}

func UiToneRGBA(red int, green int, blue int, alpha int) UiTone {
	return UiTone("rgba(" +
		strconv.Itoa(uiChannel(red)) + "," +
		strconv.Itoa(uiChannel(green)) + "," +
		strconv.Itoa(uiChannel(blue)) + "," +
		strconv.FormatFloat(float64(uiChannel(alpha))/255, 'f', 3, 64) + ")")
}

func uiChannel(n int) int {
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

func uiNum(kind string, n int) UiAttr     { return UiAttr{Kind: kind, Num: n} }
func uiStr(kind string, s string) UiAttr  { return UiAttr{Kind: kind, Word: s} }
func uiEnum(kind string, e string) UiAttr { return UiAttr{Kind: kind, Word: e} }

func UiAttrGap(n int) UiAttr    { return uiNum("gap", n) }
func UiAttrPad(n int) UiAttr    { return uiNum("pad", n) }
func UiAttrWidth(n int) UiAttr  { return uiNum("width", n) }
func UiAttrHeight(n int) UiAttr { return uiNum("height", n) }
func UiAttrGrow(n int) UiAttr   { return uiNum("grow", n) }

func UiAttrAlign(v UiAlign) UiAttr     { return uiEnum("align", string(v)) }
func UiAttrJustify(v UiJustify) UiAttr { return uiEnum("justify", string(v)) }
func UiAttrScroll(v UiAxis) UiAttr     { return uiEnum("scroll", string(v)) }
func UiAttrAnchor(v UiAnchor) UiAttr   { return uiEnum("anchor", string(v)) }
func UiAttrSize(v UiTextSize) UiAttr   { return uiEnum("size", string(v)) }
func UiAttrTone(v UiTone) UiAttr       { return uiEnum("tone", string(v)) }
func UiAttrBackground(v UiTone) UiAttr { return uiEnum("background", string(v)) }
func UiAttrKind(v UiInputKind) UiAttr  { return uiEnum("kind", string(v)) }

func uiNums(kind string, values []float64) UiAttr {
	return UiAttr{Kind: kind, Nums: values}
}

func UiAttrPaint(v UiTone) UiAttr { return uiEnum("paint", string(v)) }

func UiAttrThick(n float64) UiAttr { return uiNums("thick", []float64{n}) }

func UiAttrRegion(x float64, y float64, width float64, height float64) UiAttr {
	return uiNums("region", []float64{x, y, width, height})
}

func uiNumbers(attrs []UiAttr, kind string, fallback []float64) []float64 {
	if a, ok := uiFind(attrs, kind); ok && len(a.Nums) == len(fallback) {
		return a.Nums
	}
	return fallback
}

func UiAttrHeading(n int) UiAttr        { return uiNum("heading", n) }
func UiAttrPlaceholder(s string) UiAttr { return uiStr("placeholder", s) }
func UiAttrHint(s string) UiAttr        { return uiStr("hint", s) }
func UiAttrTouch(s string) UiAttr       { return uiStr("touch", s) }
func UiAttrTitle(s string) UiAttr       { return uiStr("title", s) }
func UiAttrKeys(keys []string) UiAttr   { return uiStr("keys", strings.Join(keys, " ")) }
func UiAttrCaret(anchor int, head int, serial int) UiAttr {
	return UiAttr{Kind: "caret", Nums: []float64{float64(anchor), float64(head), float64(serial)}}
}
func UiAttrFocus(serial int) UiAttr { return uiNum("focus", serial) }
func UiAttrClip(text string, serial int) UiAttr { return UiAttr{Kind: "clip", Word: text, Num: serial} }
func UiAttrPerform(command string, serial int) UiAttr {
	return UiAttr{Kind: "perform", Word: command, Num: serial}
}
func UiAttrNumbers(b bool) UiAttr  { return UiAttr{Kind: "numbers", Flag: b} }
func UiAttrReadOnly(b bool) UiAttr { return UiAttr{Kind: "readOnly", Flag: b} }
func UiAttrTabSize(n int) UiAttr   { return UiAttr{Kind: "tabSize", Num: n} }
func UiAttrFontSize(n int) UiAttr  { return UiAttr{Kind: "fontSize", Num: n} }
func UiAttrBorder(n int) UiAttr    { return UiAttr{Kind: "border", Num: n} }
func UiAttrRadius(n int) UiAttr    { return UiAttr{Kind: "radius", Num: n} }
func UiAttrRing(on bool) UiAttr    { return UiAttr{Kind: "ring", Flag: on} }
func UiAttrBorderTone(v UiTone) UiAttr { return uiEnum("borderTone", string(v)) }
func UiAttrHover(v UiTone) UiAttr      { return uiEnum("hover", string(v)) }

// The widgets drawn in a frame of their own: a button, a field, a list, an editor.
func uiFramed(kind string) bool {
	switch kind {
	case "button", "input", "textarea", "select", "code":
		return true
	}
	return false
}

func UiAttrDisabled(b bool) UiAttr { return UiAttr{Kind: "disabled", Flag: b} }
func UiAttrBusy(b bool) UiAttr     { return UiAttr{Kind: "busy", Flag: b} }

func UiAttrOn(msg any) UiAttr        { return UiAttr{Kind: "on", heard: &uiHeard{Msg: msg}} }
func UiAttrOnMenu(msg any) UiAttr    { return UiAttr{Kind: "onMenu", heard: &uiHeard{Msg: msg}} }
func UiAttrOnMiddle(msg any) UiAttr  { return UiAttr{Kind: "onMiddle", heard: &uiHeard{Msg: msg}} }
func UiAttrOnDouble(msg any) UiAttr  { return UiAttr{Kind: "onDouble", heard: &uiHeard{Msg: msg}} }
func UiAttrOnHover(f func(int) any) UiAttr { return UiAttr{Kind: "onHover", heard: &uiHeard{FnInt: f}} }
func UiAttrOnJump(f func(int) any) UiAttr  { return UiAttr{Kind: "onJump", heard: &uiHeard{FnInt: f}} }
func UiAttrOnProbe(f func(int) any) UiAttr { return UiAttr{Kind: "onProbe", heard: &uiHeard{FnInt: f}} }
func UiAttrOnDrag(f func(int, int, bool) any) UiAttr {
	return UiAttr{Kind: "onDrag", heard: &uiHeard{FnDrag: f}}
}
func UiAttrOnDismiss(msg any) UiAttr { return UiAttr{Kind: "onDismiss", heard: &uiHeard{Msg: msg}} }

func UiAttrOnInput(f func(string) any) UiAttr  { return UiAttr{Kind: "onInput", heard: &uiHeard{FnStr: f}} }
func UiAttrOnSubmit(f func(string) any) UiAttr { return UiAttr{Kind: "onSubmit", heard: &uiHeard{FnStr: f}} }
func UiAttrOnKey(f func(string) any) UiAttr    { return UiAttr{Kind: "onKey", heard: &uiHeard{FnStr: f}} }
func UiAttrOnCaret(f func(int, int) any) UiAttr { return UiAttr{Kind: "onCaret", heard: &uiHeard{FnSize: f}} }
func UiAttrOnView(f func(int, int) any) UiAttr  { return UiAttr{Kind: "onView", heard: &uiHeard{FnSize: f}} }
func UiAttrOnFit(f func(int, int) any) UiAttr   { return UiAttr{Kind: "onFit", heard: &uiHeard{FnSize: f}} }
func UiAttrOnPaste(f func(string) any) UiAttr   { return UiAttr{Kind: "onPaste", heard: &uiHeard{FnStr: f}} }
func UiAttrOnEdit(f func(int, int, string) any) UiAttr {
	return UiAttr{Kind: "onEdit", heard: &uiHeard{FnEdit: f}}
}
func UiAttrOnChoose(f func(string) any) UiAttr { return UiAttr{Kind: "onChoose", heard: &uiHeard{FnStr: f}} }
func UiAttrOnToggle(f func(bool) any) UiAttr   { return UiAttr{Kind: "onToggle", heard: &uiHeard{FnBool: f}} }
func UiAttrOnPick(f func(int) any) UiAttr      { return UiAttr{Kind: "onPick", heard: &uiHeard{FnInt: f}} }
func UiAttrOnSort(f func(int) any) UiAttr      { return UiAttr{Kind: "onSort", heard: &uiHeard{FnInt: f}} }

func uiFind(attrs []UiAttr, kind string) (UiAttr, bool) {
	for _, a := range attrs {
		if a.Kind == kind {
			return a, true
		}
	}
	return UiAttr{}, false
}

func uiHasEvent(attrs []UiAttr) bool {
	for _, a := range attrs {
		switch a.Kind {
		case "on", "onDismiss", "onInput", "onSubmit", "onChoose", "onToggle", "onPick", "onSort", "onKey",
			"onCaret", "onView", "onFit", "onEdit", "onPaste", "onProbe", "onMenu", "onMiddle", "onDouble", "onDrag", "onHover", "onJump":
			return true

		case "onFrame", "onRate", "onKeyDown", "onKeyUp", "onLook", "onGrab", "onPad", "onSize":
			return true
		}
	}
	return false
}

type uiRender struct {
	out      strings.Builder
	handlers map[string][]UiAttr
	next     int

scratch  []byte
scenes []string
sceneCuts [][]int32

	insets []string
	paced bool
}

var uiEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\"", "&quot;",
	"'", "&#39;",
)

func uiEscape(s string) string {
	return uiEscaper.Replace(s)
}

func uiEscapeTo(b *strings.Builder, s string) {
	at := 0
	for i := 0; i < len(s); i++ {
		var swap string
		switch s[i] {
		case '&':
			swap = "&amp;"
		case '<':
			swap = "&lt;"
		case '>':
			swap = "&gt;"
		case '"':
			swap = "&quot;"
		case '\'':
			swap = "&#39;"
		default:
			continue
		}
		b.WriteString(s[at:i])
		b.WriteString(swap)
		at = i + 1
	}
	b.WriteString(s[at:])
}

func uiEscapeInto(b []byte, s string) []byte {
	at := 0
	for i := 0; i < len(s); i++ {
		var swap string
		switch s[i] {
		case '&':
			swap = "&amp;"
		case '<':
			swap = "&lt;"
		case '>':
			swap = "&gt;"
		case '"':
			swap = "&quot;"
		case '\'':
			swap = "&#39;"
		default:
			continue
		}
		b = append(b, s[at:i]...)
		b = append(b, swap...)
		at = i + 1
	}
	return append(b, s[at:]...)
}

func uiStyle(b []byte, kind string, attrs []UiAttr) []byte {
	switch kind {
	case "row":
		b = append(b, "display:flex;flex-direction:row;"...)
	case "column", "inset":
		b = append(b, "display:flex;flex-direction:column;"...)
	case "spacer":
		b = append(b, "flex:1 1 auto;"...)
	}
	for _, a := range attrs {
		switch a.Kind {
		case "gap":
			b = append(b, "gap:"...)
			b = uiInt(b, a.Num)
			b = append(b, "px;"...)
		case "pad":
			b = append(b, "padding:"...)
			b = uiInt(b, a.Num)
			b = append(b, "px;"...)
		case "width":
			b = append(b, "width:"...)
			b = uiInt(b, a.Num)
			b = append(b, "px;"...)
			if kind == "row" || kind == "column" {
				b = append(b, "max-width:100%;"...)
			}
		case "height":
			b = append(b, "height:"...)
			b = uiInt(b, a.Num)
			b = append(b, "px;"...)
		case "align":
			b = append(b, "align-items:"...)
			b = append(b, uiFlexPos(a.Word)...)
			b = append(b, ";"...)
		case "justify":
			b = append(b, "justify-content:"...)
			b = append(b, uiFlexPos(a.Word)...)
			b = append(b, ";"...)
		case "scroll":
			switch a.Word {
			case "Horizontal":
				b = append(b, "overflow-x:auto;"...)
			case "Vertical":
				b = append(b, "overflow-y:auto;"...)
			default:
				b = append(b, "overflow:auto;"...)
			}
			b = append(b, "min-height:0;min-width:0;"...)
		case "tone":
			if c, colour := uiToneColour(a.Word); colour {
				b = append(b, "color:"...)
				b = append(b, c...)
				b = append(b, ";"...)
			}
		case "background":
			if c, colour := uiToneColour(a.Word); colour {
				b = append(b, "background:"...)
				b = append(b, c...)
				b = append(b, ";"...)
			}
		case "border":
			if a.Num <= 0 {
				b = append(b, "border:0;"...)
			} else {
				b = append(b, "border-width:"...)
				b = uiInt(b, a.Num)
				b = append(b, "px;border-style:solid;"...)
			}
		case "borderTone":
			if css := uiRunColour(a.Word); css != "" {
				b = append(b, "border-color:"...)
				b = append(b, css...)
				b = append(b, ";"...)
			}
		case "radius":
			b = append(b, "border-radius:"...)
			b = uiInt(b, max(a.Num, 0))
			b = append(b, "px;"...)
		case "hover":
			if css := uiRunColour(a.Word); css != "" {
				b = append(b, "--hv:"...)
				b = append(b, css...)
				b = append(b, ";"...)
			}
		}
	}
	// A framed widget in a colour of its own has no stylesheet edge round it
	// unless it asks for one: the edge was there to set it off from the page.
	if uiFramed(kind) {
		_, coloured := uiFind(attrs, "background")
		_, edged := uiFind(attrs, "border")
		_, toned := uiFind(attrs, "borderTone")
		if coloured && !edged && !toned {
			b = append(b, "border-color:transparent;"...)
		}
	}

	if grow, ok := uiFind(attrs, "grow"); ok {
		b = append(b, "flex:"...)
		b = uiInt(b, grow.Num)
		b = append(b, " 1 0;"...)
	} else if _, ok := uiFind(attrs, "width"); ok {
		b = append(b, "flex:0 0 auto;"...)
	}
	return b
}

func uiFlexPos(v string) string {
	switch v {
	case "Start":
		return "flex-start"
	case "End":
		return "flex-end"
	case "Center":
		return "center"
	case "Between":
		return "space-between"
	case "Stretch":
		return "stretch"
	}
	return "flex-start"
}

func uiClasses(b []byte, kind string, attrs []UiAttr) []byte {
	b = append(b, "h-"...)
	b = append(b, kind...)
	for _, a := range attrs {
		switch a.Kind {
		case "tone":
			if _, colour := uiToneColour(a.Word); !colour {
				b = append(b, " h-tone-"...)
				b = uiLower(b, a.Word)
			}
		case "background":
			if _, colour := uiToneColour(a.Word); !colour {
				b = append(b, " h-bg-"...)
				b = uiLower(b, a.Word)
			}
		case "size":
			b = append(b, " h-size-"...)
			b = uiLower(b, a.Word)
		case "busy":
			if a.Flag {
				b = append(b, " h-busy"...)
			}

		case "crosshair":
			if a.Flag {
				b = append(b, " h-aiming"...)
			}
		case "ring":
			if !a.Flag {
				b = append(b, " h-noring"...)
			}
		case "hover":
			b = append(b, " h-hover"...)
		case "scroll":
			if a.Word == "Horizontal" {
				b = append(b, " h-scroll-x"...)
			}
		}
	}
	return b
}

func uiLower(b []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b = append(b, c)
	}
	return b
}

func uiInt(b []byte, n int) []byte {
	return strconv.AppendInt(b, int64(n), 10)
}

// A code editor: the line numbers, then a textarea whose own text is clear over
// a copy of it in the runs' colours, the two scrolled as one by the shell. Each
// pre and the textarea open on a newline the parser drops, so a text that
// starts on an empty line keeps it.
func (r *uiRender) code(v UiView) {
	r.out.WriteString("<div")
	r.common(v.Kind, v.Attrs)
	if size, ok := uiFind(v.Attrs, "fontSize"); ok && size.Num >= 6 && size.Num <= 72 {
		r.out.WriteString(" data-font=\"")
		r.out.WriteString(strconv.Itoa(size.Num))
		r.out.WriteString("\"")
	}
	if c, ok := uiFind(v.Attrs, "caret"); ok && len(c.Nums) == 3 {
		r.out.WriteString(" data-caret=\"")
		r.out.WriteString(strconv.Itoa(int(c.Nums[0])))
		r.out.WriteString(",")
		r.out.WriteString(strconv.Itoa(int(c.Nums[1])))
		r.out.WriteString(",")
		r.out.WriteString(strconv.Itoa(int(c.Nums[2])))
		r.out.WriteString("\"")
	}
	if _, ok := uiFind(v.Attrs, "onInput"); ok {
		r.out.WriteString(" data-whole")
	}
	tab := 4
	if t, ok := uiFind(v.Attrs, "tabSize"); ok && t.Num > 0 {
		tab = t.Num
	}
	r.out.WriteString(">")
	if n, ok := uiFind(v.Attrs, "numbers"); ok && n.Flag {
		r.out.WriteString("<pre class=\"h-gutter\" aria-hidden=\"true\">\n")
		r.gutter(v)
		r.out.WriteString("</pre>")
	}
	sized := " style=\"tab-size:" + strconv.Itoa(tab) + "\""
	r.out.WriteString("<div class=\"h-code-body\"><div class=\"h-curline\"></div><pre class=\"h-runs\" aria-hidden=\"true\"")
	r.out.WriteString(sized)
	r.out.WriteString(">\n")
	for _, m := range v.Marks {
		if m.Kind == "band" && len(m.Nums) == 2 {
			if css := uiRunColour(m.Text); css != "" {
				r.out.WriteString("<span class=\"h-band\" style=\"top:calc(8px + ")
				r.out.WriteString(strconv.Itoa(int(m.Nums[0])))
				r.out.WriteString(" * 1.5em);background:")
				r.out.WriteString(css)
				r.out.WriteString("\"></span>")
			}
		}
	}
	uiRunsTo(&r.out, v.Text, v.Marks)
	r.out.WriteString("\n</pre><textarea class=\"h-mono\" spellcheck=\"false\" wrap=\"off\" autocomplete=\"off\" autocapitalize=\"off\"")
	r.out.WriteString(sized)
	r.out.WriteString(r.disabled(v.Attrs))
	if ro, ok := uiFind(v.Attrs, "readOnly"); ok && ro.Flag {
		r.out.WriteString(" readonly")
	}
	r.out.WriteString(">\n")
	uiEscapeTo(&r.out, v.Text)
	r.out.WriteString("</textarea></div></div>")
}

// One number a line, and a line a marker names carries its tone at the edge.
func (r *uiRender) gutter(v UiView) {
	lines := strings.Count(v.Text, "\n") + 1
	marked := map[int]string{}
	for _, m := range v.Marks {
		if m.Kind == "marker" && len(m.Nums) == 2 {
			if css := uiRunColour(m.Text); css != "" {
				marked[int(m.Nums[0])] = css
			}
		}
	}
	for i := 0; i < lines; i++ {
		if css, ok := marked[i]; ok {
			r.out.WriteString("<span class=\"h-mk\" style=\"box-shadow:inset 3px 0 0 ")
			r.out.WriteString(css)
			r.out.WriteString("\">")
			r.out.WriteString(strconv.Itoa(i + 1))
			r.out.WriteString("</span>\n")
			continue
		}
		r.out.WriteString(strconv.Itoa(i + 1))
		r.out.WriteString("\n")
	}
}

// What a run's tone is to a stylesheet: a colour as written, a role as the
// role's own variable, and nothing for Normal.
func uiRunColour(tone string) string {
	if css, ok := uiToneColour(tone); ok {
		return css
	}
	switch tone {
	case "Muted":
		return "var(--muted)"
	case "Good":
		return "var(--good)"
	case "Warn":
		return "var(--warn)"
	case "Danger":
		return "var(--danger)"
	}
	return ""
}

// The text cut where any run starts or ends, each piece in what covers it: the
// run given last wins where two of a kind overlap, as a later style rule does.
func uiRunsTo(out *strings.Builder, text string, runs []UiRun) {
	type cut struct {
		at    int
		run   int
		start bool
		note  bool
	}
	var cuts []cut
	for i, m := range runs {
		if m.Kind == "note" || m.Kind == "marker" || m.Kind == "band" || len(m.Nums) != 2 || m.Nums[1] <= m.Nums[0] {
			continue
		}
		cuts = append(cuts, cut{int(m.Nums[0]), i, true, false})
		cuts = append(cuts, cut{int(m.Nums[1]), i, false, false})
	}
	// A note goes where its line ends, in characters.
	var notes []int
	for i, m := range runs {
		if m.Kind == "note" && len(m.Nums) == 2 {
			notes = append(notes, i)
		}
	}
	if len(notes) > 0 {
		ends := []int{}
		line, at := 0, 0
		for _, r := range text {
			if r == '\n' {
				ends = append(ends, at)
				line++
			}
			at++
		}
		ends = append(ends, at)
		for _, i := range notes {
			if line := int(runs[i].Nums[0]); line >= 0 && line < len(ends) {
				cuts = append(cuts, cut{ends[line], i, false, true})
			}
		}
	}
	if len(cuts) == 0 {
		uiEscapeTo(out, text)
		return
	}
	sort.SliceStable(cuts, func(i, j int) bool { return cuts[i].at < cuts[j].at })
	live := map[int]bool{}
	chars := []rune(text)
	at := 0
	piece := func(to int) {
		to = max(0, min(to, len(chars)))
		if to <= at {
			return
		}
		ink, shade, wave, line := -1, -1, -1, -1
		for i := range live {
			switch runs[i].Kind {
			case "ink":
				ink = max(ink, i)
			case "shade":
				shade = max(shade, i)
			case "squiggle":
				wave = max(wave, i)
			case "underline":
				line = max(line, i)
			}
		}
		style := ""
		if ink >= 0 {
			if css := uiRunColour(runs[ink].Text); css != "" {
				style += "color:" + css + ";"
			}
		}
		if shade >= 0 {
			if css := uiRunColour(runs[shade].Text); css != "" {
				style += "background:" + css + ";"
			}
		}
		if wave >= 0 {
			css := uiRunColour(runs[wave].Text)
			if css == "" {
				css = "currentColor"
			}
			style += "text-decoration:underline wavy " + css + ";text-decoration-skip-ink:none;"
		} else if line >= 0 {
			css := uiRunColour(runs[line].Text)
			if css == "" {
				css = "currentColor"
			}
			style += "text-decoration:underline " + css + ";"
		}
		if style != "" {
			out.WriteString("<span style=\"")
			out.WriteString(style)
			out.WriteString("\">")
		}
		uiEscapeTo(out, string(chars[at:to]))
		if style != "" {
			out.WriteString("</span>")
		}
		at = to
	}
	for _, c := range cuts {
		piece(c.at)
		if c.note {
			css := uiRunColour(runs[c.run].Text)
			out.WriteString("<span class=\"h-note\" data-note=\"")
			uiEscapeTo(out, uiNoteText(runs[c.run]))
			out.WriteString("\"")
			if css != "" {
				out.WriteString(" style=\"color:")
				out.WriteString(css)
				out.WriteString("\"")
			}
			out.WriteString("></span>")
			continue
		}
		if c.start {
			live[c.run] = true
		} else {
			delete(live, c.run)
		}
	}
	piece(len(chars))
}

func (r *uiRender) common(kind string, attrs []UiAttr) {
	b := r.scratch[:0]
	b = append(b, " class=\""...)
	b = uiClasses(b, kind, attrs)
	b = append(b, '"')

	was := len(b)
	b = append(b, " style=\""...)
	held := len(b)
	b = uiStyle(b, kind, attrs)
	if len(b) == held {
		b = b[:was]
	} else {
		b = append(b, '"')
	}
	if hint, ok := uiFind(attrs, "hint"); ok {
		b = append(b, " title=\""...)
		b = uiEscapeInto(b, hint.Word)
		b = append(b, '"')
	}
	if touch, ok := uiFind(attrs, "touch"); ok {
		b = append(b, " data-touch=\""...)
		b = uiEscapeInto(b, touch.Word)
		b = append(b, '"')
	}
	if title, ok := uiFind(attrs, "title"); ok {
		b = append(b, " data-title=\""...)
		b = uiEscapeInto(b, title.Word)
		b = append(b, '"')
	}
	if keys, ok := uiFind(attrs, "keys"); ok {
		b = append(b, " data-keys=\""...)
		b = uiEscapeInto(b, keys.Word)
		b = append(b, '"')
	}
	if f, ok := uiFind(attrs, "focus"); ok {
		b = append(b, " data-focus=\""...)
		b = strconv.AppendInt(b, int64(f.Num), 10)
		b = append(b, '"')
	}
	if c, ok := uiFind(attrs, "clip"); ok {
		b = append(b, " data-clip=\""...)
		b = strconv.AppendInt(b, int64(c.Num), 10)
		b = append(b, "\" data-clip-text=\""...)
		b = uiEscapeInto(b, c.Word)
		b = append(b, '"')
	}
	if p, ok := uiFind(attrs, "perform"); ok {
		b = append(b, " data-perform=\""...)
		b = uiEscapeInto(b, p.Word)
		b = append(b, ',')
		b = strconv.AppendInt(b, int64(p.Num), 10)
		b = append(b, '"')
	}
	heard := len(b)
	for _, a := range attrs {
		switch a.Kind {
		case "onMenu", "onMiddle", "onDouble", "onDrag", "onHover", "onJump", "onPaste", "onProbe":
			if len(b) == heard {
				b = append(b, " data-ev=\""...)
			} else {
				b = append(b, ' ')
			}
			b = append(b, strings.ToLower(a.Kind[2:])...)
		}
	}
	if len(b) > heard {
		b = append(b, '"')
	}
	if r.handlers != nil && uiHasEvent(attrs) {
		r.next++
		id := "h" + strconv.Itoa(r.next)
		r.handlers[id] = attrs
		if _, ok := uiFind(attrs, "onFrame"); ok {
			r.paced = true
		}
		b = append(b, " data-h=\""...)
		b = append(b, id...)
		b = append(b, '"')
	}
	r.scratch = b
	r.out.Write(b)
}

func (r *uiRender) view(v UiView) {
	switch v.Kind {
	case "none":
		r.out.WriteString("<!--none-->")
	case "spacer":
		r.out.WriteString("<div")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString("></div>")
	case "row", "column":
		r.out.WriteString("<div")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(">")
		for _, c := range v.Children {
			r.view(c)
		}
		r.out.WriteString("</div>")
	case "overlay":
		pinned := ""
		if a, ok := uiFind(v.Attrs, "align"); ok {
			pinned += "align-items:" + uiFlexPos(a.Word) + ";"
		}
		if j, ok := uiFind(v.Attrs, "justify"); ok {
			pinned += "justify-content:" + uiFlexPos(j.Word) + ";"
		}
		shade := "h-backdrop"
		if pinned != "" {
			shade = "h-backdrop h-pinned"
			pinned += "padding:12px;"
		}
		open := "<div class=\"" + shade + "\""
		if pinned != "" {
			open += " style=\"" + pinned + "\""
		}
		if a, ok := uiFind(v.Attrs, "anchor"); ok {
			open = "<div class=\"h-backdrop h-pinned h-anchored\" data-anchor=\"" + a.Word + "\""
		}
		r.out.WriteString(open + r.backdrop(v.Attrs) + ">")
		r.out.WriteString("<div")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(">")
		for _, c := range v.Children {
			r.view(c)
		}
		r.out.WriteString("</div></div>")
	case "text":
		tag := "span"
		if h, ok := uiFind(v.Attrs, "heading"); ok && h.Num >= 1 && h.Num <= 6 {
			tag = "h" + strconv.Itoa(h.Num)
		}
		r.out.WriteString("<")
		r.out.WriteString(tag)
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(">")
		uiEscapeTo(&r.out, v.Text)
		r.out.WriteString("</")
		r.out.WriteString(tag)
		r.out.WriteString(">")
	case "image":
		alt := ""
		if len(v.Options) > 0 {
			alt = v.Options[0]
		}
		r.out.WriteString("<img")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(" src=\"")
		uiEscapeTo(&r.out, v.Text)
		r.out.WriteString("\" alt=\"")
		uiEscapeTo(&r.out, alt)
		r.out.WriteString("\">")
	case "icon":
		r.out.WriteString("<span")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(" aria-hidden=\"true\">")
		r.out.WriteString(uiIconGlyph(v.Text))
		r.out.WriteString("</span>")
	case "link":
		href := ""
		if len(v.Options) > 0 {
			href = v.Options[0]
		}
		r.out.WriteString("<a")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(" href=\"")
		uiEscapeTo(&r.out, uiHref(href))
		r.out.WriteString("\">")
		uiEscapeTo(&r.out, v.Text)
		r.out.WriteString("</a>")
	case "button":
		r.out.WriteString("<button type=\"button\"")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(r.disabled(v.Attrs))
		r.out.WriteString(">")
		uiEscapeTo(&r.out, v.Text)
		r.out.WriteString("</button>")
	case "input":
		kind := "text"
		if k, ok := uiFind(v.Attrs, "kind"); ok {
			kind = uiInputType(k.Word)
		}
		r.out.WriteString("<input type=\"")
		r.out.WriteString(kind)
		r.out.WriteString("\"")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(r.disabled(v.Attrs))
		r.out.WriteString(" value=\"")
		uiEscapeTo(&r.out, v.Text)
		r.out.WriteString("\"")
		if p, ok := uiFind(v.Attrs, "placeholder"); ok {
			r.out.WriteString(" placeholder=\"")
			uiEscapeTo(&r.out, p.Word)
			r.out.WriteString("\"")
		}
		r.out.WriteString(">")
	case "code":
		r.code(v)
	case "textarea":
		r.out.WriteString("<textarea")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(r.disabled(v.Attrs))
		if p, ok := uiFind(v.Attrs, "placeholder"); ok {
			r.out.WriteString(" placeholder=\"")
			uiEscapeTo(&r.out, p.Word)
			r.out.WriteString("\"")
		}
		r.out.WriteString(">")
		uiEscapeTo(&r.out, v.Text)
		r.out.WriteString("</textarea>")
	case "checkbox":
		r.out.WriteString("<label")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString("><input type=\"checkbox\"")
		if v.Flag {
			r.out.WriteString(" checked")
		}
		r.out.WriteString(r.disabled(v.Attrs))
		r.out.WriteString("><span>")
		uiEscapeTo(&r.out, v.Text)
		r.out.WriteString("</span></label>")
	case "select":
		r.out.WriteString("<select")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(r.disabled(v.Attrs))
		r.out.WriteString(">")
		for _, opt := range v.Options {
			sel := ""
			if opt == v.Text {
				sel = " selected"
			}
			r.out.WriteString("<option value=\"")
			uiEscapeTo(&r.out, opt)
			r.out.WriteString("\"")
			r.out.WriteString(sel)
			r.out.WriteString(">")
			uiEscapeTo(&r.out, opt)
			r.out.WriteString("</option>")
		}
		r.out.WriteString("</select>")
	case "scene":
		at := len(r.scenes)
		r.scenes = append(r.scenes, v.Text)
		r.sceneCuts = append(r.sceneCuts, v.Cuts)
		r.out.WriteString("<div")
		r.common(v.Kind, v.Attrs)
		if r.handlers != nil {
			r.out.WriteString(" data-scene=\"")
			r.out.WriteString(strconv.Itoa(at))
			r.out.WriteString("\"")
		}
		r.out.WriteString("></div>")
	case "inset":

		at := len(r.insets)
		r.insets = append(r.insets, v.Text)
		r.out.WriteString("<div")
		r.common(v.Kind, v.Attrs)
		if r.handlers != nil {
			r.out.WriteString(" data-inset=\"")
			r.out.WriteString(strconv.Itoa(at))
			r.out.WriteString("\"")
		}
		r.out.WriteString(">")
		for _, c := range v.Children {
			r.view(c)
		}
		r.out.WriteString("</div>")
	case "canvas":
		r.canvas(v)
	case "table":
		r.table(v)
	case "spinner":
		r.out.WriteString("<div")
		r.common(v.Kind, v.Attrs)
		r.out.WriteString(" role=\"status\"></div>")
	default:
		r.out.WriteString("<!--unknown widget-->")
	}
}

func (r *uiRender) canvas(v UiView) {
	region := uiNumbers(v.Attrs, "region", []float64{0, 0, 100, 100})

	if region[2] <= 0 || region[3] <= 0 {
		region = []float64{0, 0, 100, 100}
	}
	r.out.WriteString("<svg")
	r.common(v.Kind, v.Attrs)
	r.out.WriteString(" viewBox=\"")
	r.out.WriteString(uiNumber(region[0]))
	r.out.WriteString(" ")
	r.out.WriteString(uiNumber(region[1]))
	r.out.WriteString(" ")
	r.out.WriteString(uiNumber(region[2]))
	r.out.WriteString(" ")
	r.out.WriteString(uiNumber(region[3]))
	r.out.WriteString("\" preserveAspectRatio=\"xMidYMid meet\"")
	r.out.WriteString(" xmlns=\"http://www.w3.org/2000/svg\">")
	for _, m := range v.Marks {
		r.mark(m)
	}
	r.out.WriteString("</svg>")
}

func (r *uiRender) mark(m UiMark) {
	colour := uiMarkColour(m.Attrs)
	switch m.Kind {
	case "dot":
		if len(m.Nums) < 3 {
			return
		}
		r.out.WriteString("<circle cx=\"" + uiNumber(m.Nums[0]) + "\" cy=\"" +
			uiNumber(m.Nums[1]) + "\" r=\"" + uiNumber(m.Nums[2]) +
			"\" fill=\"" + colour + "\"/>")
	case "bar":
		if len(m.Nums) < 4 {
			return
		}
		r.out.WriteString("<rect x=\"" + uiNumber(m.Nums[0]-m.Nums[2]/2) +
			"\" y=\"" + uiNumber(m.Nums[1]-m.Nums[3]/2) +
			"\" width=\"" + uiNumber(m.Nums[2]) +
			"\" height=\"" + uiNumber(m.Nums[3]) +
			"\" fill=\"" + colour + "\"/>")
	case "path":
		points := uiPoints(m.Nums)
		if points == "" {
			return
		}
		r.out.WriteString("<polyline points=\"" + points + "\" fill=\"none\" stroke=\"" +
			colour + "\" stroke-width=\"" + uiNumber(uiThick(m.Attrs)) +
			"\" stroke-linecap=\"round\" stroke-linejoin=\"round\"/>")
	case "patch":
		points := uiPoints(m.Nums)
		if points == "" {
			return
		}
		r.out.WriteString("<polygon points=\"" + points + "\" fill=\"" + colour + "\"/>")
	case "words":
		if len(m.Nums) < 3 {
			return
		}

		r.out.WriteString("<text x=\"" + uiNumber(m.Nums[0]) + "\" y=\"" +
			uiNumber(m.Nums[1]) + "\" font-size=\"" + uiNumber(m.Nums[2]) +
			"\" fill=\"" + colour + "\" text-anchor=\"middle\"" +
			" dominant-baseline=\"central\"" +
			" font-family=\"system-ui,-apple-system,sans-serif\">" +
			uiEscape(m.Text) + "</text>")
	}
}

func uiMarkColour(attrs []UiAttr) string {
	paint, ok := uiFind(attrs, "paint")
	if !ok {
		return "currentColor"
	}
	if colour, is := uiToneColour(paint.Word); is {
		return uiEscape(colour)
	}
	switch paint.Word {
	case "Muted":
		return "var(--muted)"
	case "Good":
		return "var(--good)"
	case "Warn":
		return "var(--warn)"
	case "Danger":
		return "var(--danger)"
	}
	return "currentColor"
}

func uiThick(attrs []UiAttr) float64 {
	thick := uiNumbers(attrs, "thick", []float64{1})
	if thick[0] <= 0 {
		return 1
	}
	return thick[0]
}

func uiPoints(nums []float64) string {
	var b strings.Builder
	for i := 0; i+1 < len(nums); i += 2 {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(uiNumber(nums[i]) + "," + uiNumber(nums[i+1]))
	}
	return b.String()
}

func uiNumber(n float64) string {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return "0"
	}
	return strconv.FormatFloat(n, 'f', -1, 32)
}

func uiHref(raw string) string {
	if raw == "" {
		return "#"
	}
	switch raw[0] {
	case '/', '#', '?':
		return raw
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") {
		return raw
	}

	if i := strings.IndexByte(raw, ':'); i >= 0 {
		if j := strings.IndexByte(raw, '/'); j < 0 || i < j {
			return "#"
		}
	}
	return raw
}

func (r *uiRender) backdrop(attrs []UiAttr) string {
	if r.handlers == nil {
		return ""
	}
	if d, ok := uiFind(attrs, "onDismiss"); ok {
		r.next++
		id := "h" + strconv.Itoa(r.next)
		r.handlers[id] = []UiAttr{{Kind: "on", heard: d.heard}}
		return " data-h=\"" + id + "\" data-backdrop=\"1\""
	}
	return ""
}

func (r *uiRender) disabled(attrs []UiAttr) string {
	if d, ok := uiFind(attrs, "disabled"); ok && d.Flag {
		return " disabled"
	}
	if b, ok := uiFind(attrs, "busy"); ok && b.Flag {
		return " disabled"
	}
	return ""
}

func (r *uiRender) table(v UiView) {
	r.out.WriteString("<table")
	r.common(v.Kind, v.Attrs)
	r.out.WriteString(">")
	if len(v.Rows) == 0 {
		r.out.WriteString("</table>")
		return
	}
	sortAttr, sortable := uiFind(v.Attrs, "onSort")
	r.out.WriteString("<thead><tr>")
	for i, head := range v.Rows[0] {
		cell := "<th"
		if r.handlers != nil && sortable {
			r.next++
			id := "h" + strconv.Itoa(r.next)
			r.handlers[id] = []UiAttr{{Kind: "onSort", heard: sortAttr.heard, Num: i}}
			cell += " data-h=\"" + id + "\" data-index=\"" + strconv.Itoa(i) + "\" class=\"h-sortable\""
		}
		r.out.WriteString(cell + ">" + uiEscape(head) + "</th>")
	}
	r.out.WriteString("</tr></thead><tbody>")
	pickAttr, pickable := uiFind(v.Attrs, "onPick")
	for i, row := range v.Rows[1:] {
		line := "<tr"
		if r.handlers != nil && pickable {
			r.next++
			id := "h" + strconv.Itoa(r.next)
			r.handlers[id] = []UiAttr{{Kind: "onPick", heard: pickAttr.heard, Num: i}}
			line += " data-h=\"" + id + "\" data-index=\"" + strconv.Itoa(i) + "\" class=\"h-pickable\""
		}
		r.out.WriteString(line + ">")
		for _, cell := range row {
			r.out.WriteString("<td>" + uiEscape(cell) + "</td>")
		}
		r.out.WriteString("</tr>")
	}
	r.out.WriteString("</tbody></table>")
}

func uiInputType(v string) string {
	switch v {
	case "Number":
		return "number"
	case "Date":
		return "date"
	case "Password":
		return "password"
	case "Search":
		return "search"
	case "Range":
		return "range"
	}
	return "text"
}

func uiIconGlyph(name string) string {
	switch name {
	case "Search":
		return "\u2315"
	case "Close":
		return "\u2715"
	case "Check":
		return "\u2713"
	case "Plus":
		return "\uff0b"
	case "Minus":
		return "\u2212"
	case "Up":
		return "\u2191"
	case "Down":
		return "\u2193"
	case "Left":
		return "\u2190"
	case "Right":
		return "\u2192"
	case "Warning":
		return "\u26a0"
	case "Info":
		return "\u2139"
	case "Star":
		return "\u2605"
	case "Heart":
		return "\u2665"
	case "Reply":
		return "\u21a9"
	case "Repeat":
		return "\u21bb"
	case "Trash":
		return "\u2326"
	case "User":
		return "\u25cf"
	case "Menu":
		return "\u2261"
	}
	return "\u25a1"
}

func UiHtml(v UiView) string {
	r := &uiRender{}
	r.view(v)
	return r.out.String()
}

func UiPage(title string, v UiView) string {
	return "<!doctype html>\n<html><head><meta charset=\"utf-8\">" +
		"<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">" +
		"<title>" + uiEscape(title) + "</title><style>" + uiCSS + "</style></head><body>" +
		"<div class=\"h-root\">" + UiHtml(v) + "</div></body></html>"
}

const uiShell = `<!doctype html>
<html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>__TITLE__</title>
<link rel="icon" href="__ICON__">
<style>__CSS__</style></head>
<body><div class="h-root" id="root"></div>
<script>
var sock = new WebSocket("ws://" + location.host + "/_hive/socket?token=__TOKEN__");

sock.binaryType = "arraybuffer";
var root = document.getElementById("root");
var paintScene = null;
var attachScene = null;
var pendingScenes = [];
var sceneQueued = false;

var hiveOwed = false;

window.hiveSceneReady = function (paint, attach) {
  paintScene = paint;
  attachScene = attach;
  attach();
  flushScene();
};

function flushScene() {
  sceneQueued = false;
  if (!paintScene || !pendingScenes.length) { return; }
  var waiting = pendingScenes;
  pendingScenes = [];
  for (var i = 0; i < waiting.length; i++) { paintScene(waiting[i]); }
}

function keepScene(frame) {
  if (frame.byteLength >= 9 && new DataView(frame).getUint32(5, true) === 0) {
    pendingScenes.length = 0;
  }
  pendingScenes.push(frame);
  if (paintScene && !sceneQueued) {
    sceneQueued = true;
    requestAnimationFrame(flushScene);
  }
}
sock.onmessage = function (e) {
  if (typeof e.data !== "string") { keepScene(e.data); return; }
  var head = e.data.charAt(0);
  if (head === "A") { hiveOwed = false; return; }

  if (head === "P") {
    if (uiApply(e.data.slice(1)) && attachScene) { attachScene(); }
    uiTitle();
    uiCodeSync();
    uiAnchorSync();
    return;
  }
  if (uiPatch(e.data.slice(1)) && attachScene) { attachScene(); }
  uiTitle();
  uiCodeSync();
  uiAnchorSync();
};

function uiTitle() {
  var top = root.firstElementChild;
  var title = top && top.getAttribute("data-title");
  if (title && document.title !== title) { document.title = title; }
}
sock.onclose = function () { document.body.classList.add("h-closed"); };
function send(id, kind, value, index) {
  if (sock.readyState !== 1) { return; }
  sock.send(JSON.stringify({ id: id, kind: kind, value: String(value), index: index | 0 }));
}

function uiScrolled(node, at, into) {
  if (node.scrollTop || node.scrollLeft) {
    into.push([at, node.scrollTop, node.scrollLeft]);
  }
  for (var i = 0; i < node.children.length; i++) {
    uiScrolled(node.children[i], at + "." + i, into);
  }
  return into;
}
function uiPathFrom(from, at) {
  var node = from;
  var parts = at.split(".");
  for (var i = 1; i < parts.length && node; i++) {
    node = node.children[parts[i] | 0];
  }
  return node || null;
}

var draft = document.createElement("div");
var moved = 0;

function uiPatch(html) {
  draft.innerHTML = html;
  moved = 0;
  uiPatchKids(root, draft);
  return moved;
}

function uiApply(text) {
  var edits = JSON.parse(text);
  moved = 0;
  for (var i = 0; i < edits.length; i++) {
    var at = edits[i][0];
    if (at === "") {
      draft.innerHTML = edits[i][1];
      uiPatchKids(root, draft);
      continue;
    }
    var node = uiNodeAt(at);
    if (!node) { continue; }
    var fresh = uiParseIn(node.parentNode, edits[i][1]);
    if (fresh) { uiPatchNode(node.parentNode, node, fresh); }
  }
  return moved;
}

var cut = document.createRange();

function uiParseIn(parent, html) {
  cut.selectNodeContents(parent);
  return cut.createContextualFragment(html).firstChild;
}

function uiNodeAt(at) {
  var node = root, parts = at.split(".");
  for (var i = 0; i < parts.length && node; i++) {
    node = node.childNodes[parts[i] | 0];
  }
  return node || null;
}

function uiPatchKids(live, next) {
  var a = live.firstChild, b = next.firstChild;
  while (a && b) {

    var an = a.nextSibling, bn = b.nextSibling;
    uiPatchNode(live, a, b);
    a = an;
    b = bn;
  }
  while (a) {
    var gone = a.nextSibling;
    live.removeChild(a);
    moved++;
    a = gone;
  }
  while (b) {
    var after = b.nextSibling;
    live.appendChild(b);
    moved++;
    b = after;
  }
}

function uiPatchNode(parent, a, b) {
  if (a.nodeType !== b.nodeType || a.nodeName !== b.nodeName) {
    uiSwapNode(parent, a, b);
    return;
  }
  if (a.nodeType === 3) {
    if (a.nodeValue !== b.nodeValue) { a.nodeValue = b.nodeValue; }
    return;
  }
  if (a.nodeType !== 1) { return; }
  var name = a.nodeName;

  var pick = name === "SELECT" ? uiChosenIn(b) : null;
  var written = name === "TEXTAREA" ? b.textContent : null;
  uiPatchAttrs(a, b);
  uiPatchKids(a, b);
  uiDress(a, b, name, pick, written);
}

function uiPatchAttrs(a, b) {
  var want = b.attributes, have = a.attributes, i;
  for (i = 0; i < want.length; i++) {
    var one = want[i];
    if (a.getAttribute(one.name) !== one.value) { a.setAttribute(one.name, one.value); }
  }

  for (i = have.length - 1; i >= 0; i--) {
    var was = have[i].name;
    if (!b.hasAttribute(was)) { a.removeAttribute(was); }
  }
}

function uiDress(a, b, name, pick, written) {
  if (name === "INPUT") {
    if (a.type === "checkbox" || a.type === "radio") {
      var on = b.hasAttribute("checked");
      if (a.checked !== on) { a.checked = on; }
      return;
    }
    var want = b.getAttribute("value");
    if (want !== null && a.value !== want) { a.value = want; }
    return;
  }
  if (name === "TEXTAREA") {
    if (a.classList.contains("h-mono")) { uiCodeDress(a, written); return; }
    if (a.value !== written) { a.value = written; }
    return;
  }
  if (name === "SELECT" && pick !== null && a.value !== pick) { a.value = pick; }
}

function uiChosenIn(el) {
  var opts = el.getElementsByTagName("option");
  for (var i = 0; i < opts.length; i++) {
    if (opts[i].hasAttribute("selected")) { return opts[i].getAttribute("value"); }
  }
  return null;
}

function uiSwapNode(parent, a, b) {
  var scrolled = a.nodeType === 1 ? uiScrolled(a, "", []) : [];
  var mark = null, start = null;
  var here = document.activeElement;
  if (here && a.nodeType === 1 && a.contains(here)) {
    mark = here.getAttribute ? here.getAttribute("data-h") : null;
    start = here.selectionStart;
  }
  parent.replaceChild(b, a);
  moved++;
  for (var s = 0; s < scrolled.length; s++) {
    var box = uiPathFrom(b, scrolled[s][0]);
    if (box) { box.scrollTop = scrolled[s][1]; box.scrollLeft = scrolled[s][2]; }
  }
  if (mark && b.nodeType === 1) {
    var again = b.querySelector('[data-h="' + mark + '"]');
    if (again && again.focus) {
      again.focus();
      if (start != null && again.setSelectionRange) {
        try { again.setSelectionRange(start, start); } catch (err) {}
      }
    }
  }
}
function holder(el) {
  while (el && el !== root) {
    if (el.getAttribute && el.getAttribute("data-h")) { return el; }
    el = el.parentNode;
  }
  return null;
}
function anchorUnder(el, stop) {
  while (el && el !== stop) {
    if (el.tagName === "A") { return true; }
    el = el.parentNode;
  }
  return false;
}
root.addEventListener("click", function (e) {
  var el = holder(e.target);
  if (!el) { return; }
  if (el.getAttribute("data-backdrop") && e.target !== el) { return; }
  if (el.tagName === "A") {
    e.preventDefault();
  } else if (anchorUnder(e.target, el)) {

    return;
  }
  var idx = parseInt(el.getAttribute("data-index") || "0", 10);
  settled = true;
  send(el.getAttribute("data-h"), "click", "", idx);
});

var pressedOn = "";
var settled = false;
root.addEventListener("mousedown", function (e) {
  var el = holder(e.target);
  pressedOn = el && el.tagName !== "A" ? el.getAttribute("data-h") : "";
});
root.addEventListener("mouseup", function (e) {
  var el = holder(e.target);
  var was = pressedOn;
  pressedOn = "";
  if (!el || el.tagName === "A" || el.disabled) { return; }
  var id = el.getAttribute("data-h");
  if (!id || id !== was) { return; }
  if (el.getAttribute("data-backdrop") && e.target !== el) { return; }
  if (anchorUnder(e.target, el)) { return; }
  var idx = parseInt(el.getAttribute("data-index") || "0", 10);
  settled = false;
  setTimeout(function () {
    if (settled) { return; }
    send(id, "click", "", idx);
  }, 0);
});
root.addEventListener("input", function (e) {
  var el = holder(e.target);
  if (!el) { return; }
  var t = e.target;
  if (t.type === "checkbox") { send(el.getAttribute("data-h"), "toggle", t.checked ? "1" : "", 0); }
  else { send(el.getAttribute("data-h"), "input", t.value, 0); }
});
root.addEventListener("change", function (e) {
  var el = holder(e.target);
  if (!el || e.target.tagName !== "SELECT") { return; }
  send(el.getAttribute("data-h"), "choose", e.target.value, 0);
});

var uiPunct = {Backquote: "\x60", Minus: "-", Equal: "=", BracketLeft: "[", BracketRight: "]",
  Backslash: "\\", Semicolon: ";", Quote: "'", Comma: ",", Period: ".", Slash: "/", IntlBackslash: "\\"};
function uiKeyName(e) {
  var key = e.key;
  if (key === "Control" || key === "Shift" || key === "Alt" || key === "Meta" || key === "AltGraph") { return ""; }
  var held = e.ctrlKey || e.metaKey || e.altKey;
  if (key === " ") { key = "Space"; }
  else if (key === "Esc") { key = "Escape"; }
  else if (key === "Dead" || key === "Unidentified" || (key.length === 1 && held)) {
    var code = e.code || "";
    if (code.indexOf("Key") === 0) { key = code.slice(3); }
    else if (code.indexOf("Digit") === 0) { key = code.slice(5); }
    else if (uiPunct[code]) { key = uiPunct[code]; }
    else if (key.length === 1) { key = key.toUpperCase(); }
    else { return ""; }
  } else if (key.length === 1) { return key; }
  return (e.ctrlKey || e.metaKey ? "Ctrl+" : "") + (e.shiftKey ? "Shift+" : "") + (e.altKey ? "Alt+" : "") + key;
}
document.addEventListener("keydown", function (e) {
  if (e.isComposing) { return; }
  var name = uiKeyName(e);
  if (!name) { return; }
  if (uiClaim(e, name)) { return; }
  var ta = e.target;
  if (ta && ta.classList && ta.classList.contains("h-mono") && !ta.disabled) {
    if (name === "Tab") { e.preventDefault(); uiCodeType(ta, "\t"); }
    else if (name === "Enter") {
      e.preventDefault();
      var start = ta.value.lastIndexOf("\n", ta.selectionStart - 1) + 1;
      var indent = /^[\t ]*/.exec(ta.value.slice(start, ta.selectionStart))[0];
      uiCodeType(ta, "\n" + indent);
    }
  }
}, true);
document.addEventListener("paste", function (e) {
  var el = uiHearer(e.target, "paste");
  if (!el) { return; }
  e.preventDefault();
  send(el.getAttribute("data-h"), "paste", (e.clipboardData || window.clipboardData).getData("text"), 0);
}, true);
function uiClaim(e, name) {
  if ((name === "Ctrl+V" || name === "Shift+Insert") && uiHearer(e.target, "paste")) { return false; }
  var el = e.target && root.contains(e.target) ? e.target : root.firstElementChild;
  for (; el && el !== root; el = el.parentNode) {
    var keys = el.getAttribute && el.getAttribute("data-keys");
    if (!keys || !el.getAttribute("data-h")) { continue; }
    var list = keys.split(" ");
    if (list.indexOf(name) < 0 && list.indexOf("Any") < 0) { continue; }
    e.preventDefault();
    e.stopPropagation();
    send(el.getAttribute("data-h"), "key", name, 0);
    return true;
  }
  return false;
}

var uiSurrogate = /[\uD800-\uDFFF]/;
function uiRunes(s, units) {
  if (!uiSurrogate.test(s)) { return units; }
  var n = 0;
  for (var i = 0; i < units; i++) { var c = s.charCodeAt(i); if (c < 0xDC00 || c > 0xDFFF) { n++; } }
  return n;
}
function uiUnits(s, runes) {
  if (!uiSurrogate.test(s)) { return Math.max(0, Math.min(runes, s.length)); }
  var i = 0, n = 0;
  while (i < s.length && n < runes) { var c = s.charCodeAt(i); i += c >= 0xD800 && c < 0xDC00 ? 2 : 1; n++; }
  return i;
}
function uiCodeState(ta) {
  if (!ta.__code) { ta.__code = { value: ta.value, sent: [], caret: null, told: null, view: null }; }
  return ta.__code;
}
function uiCodeType(ta, text) {
  if (!document.execCommand("insertText", false, text)) {
    ta.setRangeText(text, ta.selectionStart, ta.selectionEnd, "end");
    ta.dispatchEvent(new Event("input", { bubbles: true }));
  }
}
document.addEventListener("input", function (e) {
  var ta = e.target;
  if (!ta || !ta.classList || !ta.classList.contains("h-mono")) { return; }
  e.stopPropagation();
  var box = ta.closest(".h-code"), s = uiCodeState(ta);
  var was = s.value, now = ta.value, a = 0, top = Math.min(was.length, now.length);
  while (a < top && was.charCodeAt(a) === now.charCodeAt(a)) { a++; }
  var zw = was.length, zn = now.length;
  while (zw > a && zn > a && was.charCodeAt(zw - 1) === now.charCodeAt(zn - 1)) { zw--; zn--; }
  if (a > 0 && (was.charCodeAt(a - 1) & 0xFC00) === 0xD800) { a--; }
  if (zw < was.length && (was.charCodeAt(zw) & 0xFC00) === 0xDC00) { zw++; zn++; }
  s.value = now;
  s.sent.push(now);
  if (s.sent.length > 64) { s.sent.shift(); }
  uiCodePaint(box, a, zw, now.slice(a, zn));
  var id = box && box.getAttribute("data-h");
  if (!id) { return; }
  send(id, "edit", uiRunes(was, a) + "," + uiRunes(was, zw) + "," + now.slice(a, zn), 0);
  if (box.hasAttribute("data-whole")) { send(id, "input", now, 0); }
}, true);
function uiCodePaint(box, a, z, text) {
  var pre = box && box.querySelector(".h-runs");
  if (!pre) { return; }
  var walk = document.createTreeWalker(pre, NodeFilter.SHOW_TEXT), node, at = 0, first = null;
  while ((node = walk.nextNode())) {
    var len = node.nodeValue.length, end = at + len;
    if (first === null && a <= end) {
      var v = node.nodeValue;
      node.nodeValue = v.slice(0, a - at) + text + v.slice(Math.min(z, end) - at);
      first = node;
      if (z <= end) { break; }
    } else if (first !== null) {
      node.nodeValue = node.nodeValue.slice(Math.min(z, end) - at);
      if (z <= end) { break; }
    }
    at = end;
  }
}
function uiCodeDress(ta, written) {
  var s = uiCodeState(ta);
  if (ta.value === written) { s.sent = []; s.value = written; return; }
  if (document.activeElement === ta) {
    var seen = s.sent.lastIndexOf(written);
    if (seen >= 0) { s.sent = s.sent.slice(seen + 1); return; }
  }
  var from = ta.selectionStart, to = ta.selectionEnd, top = ta.scrollTop, left = ta.scrollLeft;
  ta.value = written;
  s.value = written;
  s.sent = [];
  ta.setSelectionRange(Math.min(from, written.length), Math.min(to, written.length));
  ta.scrollTop = top;
  ta.scrollLeft = left;
}

function uiCodeSync() {
  var boxes = root.querySelectorAll(".h-code");
  for (var i = 0; i < boxes.length; i++) {
    var box = boxes[i], ta = box.querySelector(".h-mono");
    if (!ta) { continue; }
    var font = box.getAttribute("data-font");
    box.style.fontSize = font ? font + "px" : "";
    var s = uiCodeState(ta), want = box.getAttribute("data-caret");
    if (want !== null && want !== s.caret) {
      s.caret = want;
      var p = want.split(","), v = ta.value;
      s.told = p[0] + "," + p[1];
      var anchor = uiUnits(v, p[0] | 0), head = uiUnits(v, p[1] | 0);
      ta.setSelectionRange(Math.min(anchor, head), Math.max(anchor, head), head < anchor ? "backward" : "forward");
      uiCodeReveal(ta, head);
    }
    uiCodeScrolled(box, ta);
  }
  var clips = root.querySelectorAll("[data-clip]");
  for (var c = 0; c < clips.length; c++) {
    var holder = clips[c], serial = holder.getAttribute("data-clip");
    if (holder.__clip !== undefined && holder.__clip !== serial && navigator.clipboard) {
      navigator.clipboard.writeText(holder.getAttribute("data-clip-text")).catch(function () {});
    }
    holder.__clip = serial;
  }
  var asks = root.querySelectorAll("[data-perform]");
  for (var p = 0; p < asks.length; p++) {
    var field = asks[p], said = field.getAttribute("data-perform");
    var seen = field.__perform;
    field.__perform = said;
    if (seen === undefined || seen === said) { continue; }
    var command = said.slice(0, said.lastIndexOf(","));
    var area = field.matches("input,textarea") ? field : field.querySelector("textarea,input");
    if (!area) { continue; }
    area.focus();
    if (command === "selectAll") { area.select(); }
    else if (command === "paste") {
      if (navigator.clipboard && navigator.clipboard.readText) {
        navigator.clipboard.readText().then(function (text) { document.execCommand("insertText", false, text); }).catch(function () {});
      }
    } else { document.execCommand(command); }
  }
  var asked = root.querySelectorAll("[data-focus]");
  for (var k = 0; k < asked.length; k++) {
    var el = asked[k], want = el.getAttribute("data-focus");
    if (el.__focus === want) { continue; }
    el.__focus = want;
    var target = el.matches("input,textarea,select,button,a") ? el : el.querySelector("textarea,input,select,button,a");
    if (target && target.focus) { target.focus(); }
  }
}
function uiCodeLine(ta) {
  var lh = parseFloat(getComputedStyle(ta).lineHeight);
  return lh > 0 ? lh : 19.5;
}
var uiMeasure = null;
function uiCodeReveal(ta, head) {
  var v = ta.value, lh = uiCodeLine(ta), pad = 8;
  var start = v.lastIndexOf("\n", head - 1) + 1, line = 0;
  for (var i = 0; i < start; i++) { if (v.charCodeAt(i) === 10) { line++; } }
  var y = pad + line * lh;
  if (y < ta.scrollTop + pad) { ta.scrollTop = Math.max(0, y - pad); }
  else if (y + lh > ta.scrollTop + ta.clientHeight - pad) { ta.scrollTop = y + lh - ta.clientHeight + pad; }
  if (!uiMeasure) { uiMeasure = document.createElement("canvas").getContext("2d"); }
  uiMeasure.font = getComputedStyle(ta).font;
  var tab = parseInt(ta.style.tabSize, 10) || 4, cols = 0;
  for (var j = start; j < head; j++) { cols = v.charCodeAt(j) === 9 ? (Math.floor(cols / tab) + 1) * tab : cols + 1; }
  var x = 12 + cols * uiMeasure.measureText("M").width;
  if (x < ta.scrollLeft + 12) { ta.scrollLeft = Math.max(0, x - 40); }
  else if (x > ta.scrollLeft + ta.clientWidth - 12) { ta.scrollLeft = x - ta.clientWidth + 40; }
}
function uiCodeScrolled(box, ta) {
  var pre = box.querySelector(".h-runs"), gutter = box.querySelector(".h-gutter");
  var top = ta.scrollTop, left = ta.scrollLeft, lh = uiCodeLine(ta);
  if (pre) { pre.style.transform = "translate(" + -left + "px," + -top + "px)"; }
  if (gutter) { gutter.style.transform = "translateY(" + -top + "px)"; }
  var cur = box.querySelector(".h-curline");
  if (cur) {
    var v = ta.value, at = ta.selectionStart, line = 0;
    if (document.activeElement === ta && ta.selectionStart === ta.selectionEnd) {
      for (var i = 0; i < at; i++) { if (v.charCodeAt(i) === 10) { line++; } }
      cur.style.display = "block";
      cur.style.height = lh + "px";
      cur.style.transform = "translateY(" + (8 + line * lh - top) + "px)";
    } else {
      cur.style.display = "none";
    }
  }
  var id = box.getAttribute("data-h");
  if (!id) { return; }
  var lines = 1;
  for (var k = 0; k < ta.value.length; k++) { if (ta.value.charCodeAt(k) === 10) { lines++; } }
  var first = Math.max(0, Math.floor((top) / lh));
  var last = Math.min(lines - 1, Math.floor((top + ta.clientHeight - 16) / lh));
  var view = first + "," + Math.max(first, last), s = uiCodeState(ta);
  if (view !== s.view) { s.view = view; send(id, "view", view, 0); }
  var fit = Math.max(1, Math.floor((ta.clientWidth - 24) / uiCodeCell(ta))) + "," + Math.max(1, Math.floor((ta.clientHeight - 16) / lh));
  if (fit !== s.fit) { s.fit = fit; send(id, "fit", fit, 0); }
}
document.addEventListener("scroll", function (e) {
  var ta = e.target;
  if (ta && ta.classList && ta.classList.contains("h-mono")) { uiCodeScrolled(ta.closest(".h-code"), ta); }
}, true);
window.addEventListener("resize", function () { uiCodeSync(); uiAnchorSync(); });

function uiHearer(el, ev) {
  for (; el && el !== root && el.getAttribute; el = el.parentNode) {
    var heard = el.getAttribute("data-ev");
    if (heard && el.getAttribute("data-h") && (" " + heard + " ").indexOf(" " + ev + " ") >= 0) { return el; }
  }
  return null;
}
var uiSpot = { x: 0, y: 0 }, uiDragging = null, uiDragOwed = false;
document.addEventListener("mousedown", function (e) {
  uiSpot = { x: e.clientX, y: e.clientY };
  var open = root.querySelectorAll(".h-anchored[data-h]");
  for (var i = 0; i < open.length; i++) {
    var panel = open[i].firstElementChild;
    if (panel && !panel.contains(e.target)) { send(open[i].getAttribute("data-h"), "click", "", 0); }
  }
  if (e.button === 1 && uiHearer(e.target, "middle")) { e.preventDefault(); }
  if (e.button !== 0) { return; }
  var ta = e.target;
  if ((e.ctrlKey || e.metaKey) && ta.classList && ta.classList.contains("h-mono")) {
    var box = uiHearer(ta, "jump"), at = box ? uiCodeAt(ta, e.clientX, e.clientY) : -1;
    if (at >= 0) { send(box.getAttribute("data-h"), "jump", at, 0); }
  }
  var drag = uiHearer(e.target, "drag");
  if (!drag || (e.target !== drag && e.target.closest("input,textarea,select,button,a"))) { return; }
  e.preventDefault();
  uiDragging = { id: drag.getAttribute("data-h"), x: e.clientX, y: e.clientY, dx: 0, dy: 0 };
}, true);
document.addEventListener("mousemove", function (e) {
  if (uiDragging) {
    uiDragging.dx = Math.round(e.clientX - uiDragging.x);
    uiDragging.dy = Math.round(e.clientY - uiDragging.y);
    if (!uiDragOwed) {
      uiDragOwed = true;
      requestAnimationFrame(function () {
        uiDragOwed = false;
        if (uiDragging) { send(uiDragging.id, "drag", uiDragging.dx + "," + uiDragging.dy + ",0", 0); }
      });
    }
    return;
  }
  var ta = e.target;
  if (!ta.classList || !ta.classList.contains("h-mono")) { return; }
  var box = uiHearer(ta, "hover");
  if (!box) { return; }
  var at = uiCodeAt(ta, e.clientX, e.clientY), s = uiCodeState(ta);
  if (at !== s.hover) { s.hover = at; send(box.getAttribute("data-h"), "hover", at, 0); }
}, true);
var uiProbeAt = null;
function uiProbe(ta, x, y, ctrl) {
  var box = uiHearer(ta, "probe");
  if (!box) { return; }
  var at = ctrl ? uiCodeAt(ta, x, y) : -1, s = uiCodeState(ta);
  ta.closest(".h-code").classList.toggle("h-ctrl", ctrl && at >= 0);
  if (at !== s.probe && (s.probe !== undefined || at >= 0)) { s.probe = at; send(box.getAttribute("data-h"), "probe", at, 0); }
}
document.addEventListener("mousemove", function (e) {
  var ta = e.target;
  if (!ta.classList || !ta.classList.contains("h-mono")) { uiProbeAt = null; return; }
  uiProbeAt = { ta: ta, x: e.clientX, y: e.clientY };
  uiProbe(ta, e.clientX, e.clientY, e.ctrlKey || e.metaKey);
}, true);
["keydown", "keyup"].forEach(function (name) {
  document.addEventListener(name, function (e) {
    if ((e.key === "Control" || e.key === "Meta") && uiProbeAt) {
      uiProbe(uiProbeAt.ta, uiProbeAt.x, uiProbeAt.y, name === "keydown");
    }
  }, true);
});
document.addEventListener("mouseout", function (e) {
  var ta = e.target;
  if (ta && ta.classList && ta.classList.contains("h-mono")) {
    var s = uiCodeState(ta), owner = uiHearer(ta, "probe");
    if (owner && s.probe !== undefined && s.probe !== -1) { s.probe = -1; send(owner.getAttribute("data-h"), "probe", -1, 0); }
  }
  if (!ta.classList || !ta.classList.contains("h-mono")) { return; }
  var box = uiHearer(ta, "hover"), s = uiCodeState(ta);
  if (box && s.hover !== -1 && s.hover !== undefined) { s.hover = -1; send(box.getAttribute("data-h"), "hover", -1, 0); }
}, true);
document.addEventListener("mouseup", function () {
  if (!uiDragging) { return; }
  var d = uiDragging;
  uiDragging = null;
  send(d.id, "drag", d.dx + "," + d.dy + ",1", 0);
}, true);
document.addEventListener("contextmenu", function (e) {
  uiSpot = { x: e.clientX, y: e.clientY };
  var el = uiHearer(e.target, "menu");
  if (!el) { return; }
  e.preventDefault();
  send(el.getAttribute("data-h"), "menu", "", 0);
}, true);
document.addEventListener("auxclick", function (e) {
  var el = e.button === 1 && uiHearer(e.target, "middle");
  if (!el) { return; }
  e.preventDefault();
  send(el.getAttribute("data-h"), "middle", "", 0);
}, true);
document.addEventListener("dblclick", function (e) {
  var el = uiHearer(e.target, "double");
  if (el) { send(el.getAttribute("data-h"), "double", "", 0); }
}, true);

function uiCodeCell(ta) {
  if (!uiMeasure) { uiMeasure = document.createElement("canvas").getContext("2d"); }
  uiMeasure.font = getComputedStyle(ta).font;
  return uiMeasure.measureText("M").width || 8;
}
function uiCodeAt(ta, x, y) {
  var r = ta.getBoundingClientRect(), lh = uiCodeLine(ta), cw = uiCodeCell(ta), v = ta.value;
  var line = Math.floor((y - r.top - 8 + ta.scrollTop) / lh), col = (x - r.left - 12 + ta.scrollLeft) / cw;
  if (line < 0 || col < 0) { return -1; }
  var at = 0;
  for (var l = 0; l < line; l++) { at = v.indexOf("\n", at); if (at < 0) { return -1; } at++; }
  var tab = parseInt(ta.style.tabSize, 10) || 4, c = 0;
  for (var i = at; i < v.length && v.charCodeAt(i) !== 10; i++) {
    var next = v.charCodeAt(i) === 9 ? (Math.floor(c / tab) + 1) * tab : c + 1;
    if (col < next) { return uiRunes(v, i); }
    c = next;
  }
  return -1;
}

function uiAnchorSync() {
  var open = root.querySelectorAll(".h-anchored");
  for (var i = 0; i < open.length; i++) {
    var panel = open[i].firstElementChild;
    if (!panel) { continue; }
    var x = uiSpot.x, y = uiSpot.y, above = y;
    var ta = document.activeElement;
    if (open[i].getAttribute("data-anchor") === "Caret" && ta && ta.classList && ta.classList.contains("h-mono")) {
      var r = ta.getBoundingClientRect(), lh = uiCodeLine(ta), cw = uiCodeCell(ta), v = ta.value, at = ta.selectionEnd;
      var start = v.lastIndexOf("\n", at - 1) + 1, line = 0, tab = parseInt(ta.style.tabSize, 10) || 4, c = 0;
      for (var k = 0; k < start; k++) { if (v.charCodeAt(k) === 10) { line++; } }
      for (var j = start; j < at; j++) { c = v.charCodeAt(j) === 9 ? (Math.floor(c / tab) + 1) * tab : c + 1; }
      x = r.left + 12 + c * cw - ta.scrollLeft;
      above = r.top + 8 + line * lh - ta.scrollTop;
      y = above + lh;
    }
    var w = panel.offsetWidth, h = panel.offsetHeight;
    x = Math.max(4, Math.min(x, window.innerWidth - w - 4));
    if (y + h > window.innerHeight - 4) { y = Math.max(4, Math.min(above - h, window.innerHeight - h - 4)); }
    panel.style.left = x + "px";
    panel.style.top = Math.max(4, y) + "px";
  }
}
var uiCaretOwed = false;
document.addEventListener("selectionchange", function () {
  var ta = document.activeElement;
  if (!ta || !ta.classList || !ta.classList.contains("h-mono") || uiCaretOwed) { return; }
  uiCaretOwed = true;
  requestAnimationFrame(function () {
    uiCaretOwed = false;
    var box = ta.closest(".h-code");
    if (!box) { return; }
    uiCodeScrolled(box, ta);
    var v = ta.value, s = uiCodeState(ta), back = ta.selectionDirection === "backward";
    var anchor = uiRunes(v, back ? ta.selectionEnd : ta.selectionStart);
    var head = uiRunes(v, back ? ta.selectionStart : ta.selectionEnd);
    var said = anchor + "," + head, id = box.getAttribute("data-h");
    if (said !== s.told && id) { s.told = said; send(id, "caret", said, 0); }
    uiAnchorSync();
  });
});
root.addEventListener("keydown", function (e) {
  if (e.key !== "Enter" || e.shiftKey) { return; }
  var el = holder(e.target);
  if (!el || e.target.tagName === "TEXTAREA") { return; }
  send(el.getAttribute("data-h"), "submit", e.target.value, 0);
});
</script>
__EXTRA__
</body></html>`

const uiCSS = `*,*::before,*::after{box-sizing:border-box}
@font-face{font-family:hive-own;font-weight:100 900;font-display:swap;
src:url("/_hive/asset/font.woff2") format("woff2")}
body{margin:0;font:14px/1.5 hive-own,system-ui,-apple-system,"Segoe UI",sans-serif;
background:var(--bg);color:var(--fg);-webkit-text-size-adjust:100%}
:root{--bg:#fff;--fg:#16181c;--muted:#5b6470;--line:#e3e6ea;--accent:#1d9bf0;
--good:#00a870;--warn:#c47d00;--danger:#d7263d;--surface:#f7f9fa}
@media (prefers-color-scheme:dark){:root{--bg:#15181c;--fg:#e7e9ea;--muted:#8b98a5;
--line:#2f3336;--accent:#1d9bf0;--good:#00ba7c;--warn:#ffd400;--danger:#f4212e;
--surface:#1c2026}}
.h-root{display:flex;flex-direction:column;min-height:100vh;min-height:100dvh}
.h-text{white-space:pre-wrap;overflow-wrap:anywhere}
.h-size-title{font-size:20px;font-weight:700}
.h-size-subtitle{font-size:16px;font-weight:600}
.h-size-caption{font-size:12px}
.h-bg-normal{background:var(--bg)}
.h-bg-muted{background:var(--surface)}
.h-bg-good{background:var(--good);color:#fff}
.h-bg-warn{background:var(--warn);color:#15181c}
.h-bg-danger{background:var(--danger);color:#fff}
.h-tone-muted{color:var(--muted)}
.h-tone-good{color:var(--good)}
.h-tone-warn{color:var(--warn)}
.h-tone-danger{color:var(--danger)}
.h-link{color:var(--accent);text-decoration:none}
.h-link:hover{text-decoration:underline}
.h-button{font:inherit;border:1px solid var(--line);background:var(--surface);
color:inherit;border-radius:999px;padding:6px 14px;cursor:pointer;
touch-action:manipulation}
.h-button:hover:not(:disabled){border-color:var(--accent);color:var(--accent)}
.h-button:disabled{opacity:.5;cursor:default}
.h-button.h-tone-good{background:var(--accent);border-color:var(--accent);color:#fff}
.h-button.h-tone-danger{border-color:var(--danger);color:var(--danger)}
.h-input,.h-textarea,.h-select{font:inherit;color:inherit;background:var(--bg);
border:1px solid var(--line);border-radius:8px;padding:8px 10px;min-width:0}
.h-input:focus,.h-textarea:focus{outline:2px solid var(--accent);border-color:transparent}
.h-input[type=range]{padding:0;border:0;background:none;accent-color:var(--accent);cursor:pointer}
.h-textarea{resize:vertical;min-height:64px;width:100%}
.h-checkbox{display:inline-flex;align-items:center;gap:6px;cursor:pointer}
.h-noring:focus,.h-noring:focus-visible,.h-noring:focus-within{outline:none}
.h-code.h-noring:focus-within{border-color:var(--line)}
.h-button[style*="background"]:hover:not(:disabled){border-color:transparent;filter:brightness(1.15)}
.h-hover:hover:not(:disabled){background:var(--hv)!important;filter:none;text-decoration:none}
.h-link.h-hover:hover,.h-hover .h-link:hover{text-decoration:none}
.h-code.h-ctrl .h-mono{cursor:pointer}
.h-scroll-x>*{flex-shrink:0}
.h-code{display:flex;position:relative;min-height:64px;min-width:0;overflow:hidden;
border:1px solid var(--line);border-radius:8px;background:var(--bg);
font:13px/1.5 ui-monospace,"Cascadia Mono","Cascadia Code",Consolas,"Liberation Mono","DejaVu Sans Mono",monospace}
.h-code:focus-within{border-color:var(--accent)}
.h-gutter{margin:0;padding:8px 10px 8px 12px;font:inherit;text-align:right;color:var(--muted);
opacity:.75;user-select:none;white-space:pre;flex:none;will-change:transform}
.h-mk{display:inline-block;width:100%;margin-left:-12px;padding-left:12px}
.h-code-body{position:relative;flex:1;min-width:0;overflow:hidden}
.h-runs,.h-mono{margin:0;padding:8px 12px;font:inherit;white-space:pre;letter-spacing:0}
.h-runs{position:absolute;left:0;top:0;min-width:100%;pointer-events:none;color:inherit;will-change:transform}
.h-mono{position:absolute;inset:0;width:100%;height:100%;resize:none;border:0;outline:0;
background:transparent;color:inherit;-webkit-text-fill-color:transparent;overflow:auto;box-sizing:border-box}
.h-mono::selection{background:color-mix(in srgb,var(--accent) 35%,transparent);-webkit-text-fill-color:transparent}
.h-mono:disabled{cursor:default}
.h-band{position:absolute;left:0;right:0;height:1.5em;pointer-events:none;z-index:-1}
.h-runs{z-index:0;isolation:isolate}
.h-note::after{content:attr(data-note);padding-left:3ch;font-style:italic;opacity:.9}
.h-curline{position:absolute;left:0;right:0;top:0;display:none;pointer-events:none;
background:color-mix(in srgb,var(--fg) 6%,transparent)}
.h-table{border-collapse:collapse;width:100%}
.h-table th,.h-table td{text-align:left;padding:8px 10px;border-bottom:1px solid var(--line);
overflow-wrap:anywhere}
.h-table thead th{position:sticky;top:0;background:var(--bg);font-weight:600}
.h-sortable,.h-pickable{cursor:pointer}
.h-pickable:hover{background:var(--surface)}
.h-backdrop{position:fixed;inset:0;background:rgba(0,0,0,.45);display:flex;
align-items:center;justify-content:center;z-index:10}
.h-pinned{background:none;pointer-events:none}
.h-pinned>.h-overlay{background:none;box-shadow:none;pointer-events:auto;
display:flex;max-height:none;border-radius:0}
[data-touch]{touch-action:none;user-select:none;-webkit-user-select:none}
.h-anchored>.h-overlay{position:fixed;left:0;top:0;display:block;max-height:70vh;overflow:auto;
background:var(--bg);border:1px solid var(--line);border-radius:8px;box-shadow:0 8px 24px rgba(0,0,0,.25)}
.h-overlay{background:var(--bg);border-radius:16px;max-height:85vh;overflow:auto;
max-width:calc(100vw - 24px);box-shadow:0 12px 40px rgba(0,0,0,.3)}
.h-spinner{width:18px;height:18px;border:2px solid var(--line);
border-top-color:var(--accent);border-radius:50%;animation:h-spin .7s linear infinite}
@keyframes h-spin{to{transform:rotate(360deg)}}
.h-busy{opacity:.6}
.h-icon{font-style:normal}
.h-image{max-width:100%;height:auto;display:block}
.h-closed::after{content:"disconnected";position:fixed;bottom:12px;left:12px;
background:var(--danger);color:#fff;padding:4px 10px;border-radius:6px;font-size:12px}
.h-scene{position:relative;overflow:hidden;min-height:0;min-width:0;background:#0b0d10}
.h-inset{position:relative;overflow:hidden;min-height:0;min-width:0;
background:none;pointer-events:none}
.h-world{position:fixed;z-index:2;overflow:hidden}
.h-world>canvas{display:block;width:100%;height:100%}
.h-canvas{display:block;min-width:0;min-height:0}
.h-aiming{cursor:none}
.h-aiming::before,.h-aiming::after{content:"";position:absolute;left:50%;top:50%;
background:#fff;opacity:.8;pointer-events:none}
.h-aiming::before{width:2px;height:15px;margin:-7px 0 0 -1px}
.h-aiming::after{width:15px;height:2px;margin:-1px 0 0 -7px}
@media (pointer:coarse){.h-button,.h-select,.h-input:not([type=range]){min-height:44px}
.h-checkbox{min-height:44px}
.h-input,.h-textarea,.h-select{font-size:16px}}`

type uiDom struct {
	kind byte
	name string
	from int
	open int
	to   int
	kids int32
	head int32
	tail int32
	next int32
}

func uiScan(html string, into []uiDom) []uiDom {
	nodes := into[:0]
	if cap(nodes) < 16 {
		nodes = make([]uiDom, 0, len(html)/40+16)
	}
	nodes = append(nodes, uiDom{kind: 'r', from: 0, open: 0, to: len(html)})
	stack := make([]int32, 1, 32)
	i := 0
	for i < len(html) {
		top := stack[len(stack)-1]
		if html[i] != '<' {
			at := i
			for i < len(html) && html[i] != '<' {
				i++
			}
			nodes = append(nodes, uiDom{kind: 't', from: at, open: i, to: i})
			uiAdopt(nodes, top, int32(len(nodes)-1))
			continue
		}
		if strings.HasPrefix(html[i:], "<!--") {
			shut := strings.Index(html[i:], "-->")
			if shut < 0 {
				break
			}
			end := i + shut + 3
			nodes = append(nodes, uiDom{kind: 'c', from: i, open: end, to: end})
			uiAdopt(nodes, top, int32(len(nodes)-1))
			i = end
			continue
		}
		shut := strings.IndexByte(html[i:], '>')
		if shut < 0 {
			break
		}
		end := i + shut + 1
		if html[i+1] == '/' {
			if len(stack) > 1 {
				nodes[top].to = end
				stack = stack[:len(stack)-1]
			}
			i = end
			continue
		}
		name := uiTagName(html[i+1 : end-1])
		nodes = append(nodes, uiDom{kind: 'e', name: name, from: i, open: end, to: end})
		me := int32(len(nodes) - 1)
		uiAdopt(nodes, top, me)
		if html[end-2] != '/' && uiVoidTag(name) == false {
			stack = append(stack, me)
		}
		i = end
	}
	return nodes
}

func uiAdopt(nodes []uiDom, parent int32, child int32) {
	if nodes[parent].tail == 0 {
		nodes[parent].head = child
	} else {
		nodes[nodes[parent].tail].next = child
	}
	nodes[parent].tail = child
	nodes[parent].kids++
}

func uiTagName(open string) string {
	for i := 0; i < len(open); i++ {
		c := open[i]
		if c == ' ' || c == '/' {
			return open[:i]
		}
	}
	return open
}

func uiVoidTag(name string) bool {
	return name == "img" || name == "input"
}

type uiEdit struct {
	path string
	html string
}

func uiDocEdits(was string, old []uiDom, now string, fresh []uiDom) []uiEdit {
	var edits []uiEdit
	uiDiffKids(was, old, 0, now, fresh, 0, "", &edits)
	return edits
}

func uiDiffKids(was string, old []uiDom, a int32, now string, fresh []uiDom,
	b int32, path string, edits *[]uiEdit) {
	if old[a].kids != fresh[b].kids {
		*edits = append(*edits, uiEdit{path: path, html: now[fresh[b].from:fresh[b].to]})
		return
	}
	x := old[a].head
	y := fresh[b].head
	for i := 0; x != 0 && y != 0; i, x, y = i+1, old[x].next, fresh[y].next {
		if was[old[x].from:old[x].to] == now[fresh[y].from:fresh[y].to] {
			continue
		}

		if old[x].kind != 'e' || fresh[y].kind != 'e' || old[x].name != fresh[y].name {
			*edits = append(*edits, uiEdit{path: path, html: now[fresh[b].from:fresh[b].to]})
			return
		}
		here := strconv.Itoa(i)
		if path != "" {
			here = path + "." + here
		}
		if was[old[x].from:old[x].open] != now[fresh[y].from:fresh[y].open] {
			*edits = append(*edits, uiEdit{path: here, html: now[fresh[y].from:fresh[y].to]})
			continue
		}
		uiDiffKids(was, old, x, now, fresh, y, here, edits)
	}
}

func uiEditsFrame(edits []uiEdit) string {
	var b strings.Builder
	b.WriteString("P[")
	for i, e := range edits {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('[')
		uiQuoteInto(&b, e.path)
		b.WriteByte(',')
		uiQuoteInto(&b, e.html)
		b.WriteByte(']')
	}
	b.WriteByte(']')
	return b.String()
}

func uiQuoteInto(b *strings.Builder, s string) {
	b.WriteByte(uiQuoteMark)
	at := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != uiQuoteMark && c != uiBackslash && c >= 0x20 {
			continue
		}
		b.WriteString(s[at:i])
		at = i + 1
		b.WriteByte(uiBackslash)
		if c == uiQuoteMark || c == uiBackslash {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('u')
		b.WriteByte('0')
		b.WriteByte('0')
		b.WriteByte(uiHexDigit(c >> 4))
		b.WriteByte(uiHexDigit(c & 15))
	}
	b.WriteString(s[at:])
	b.WriteByte(uiQuoteMark)
}

const uiQuoteMark = byte(34)
const uiBackslash = byte(92)

func uiHexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}

type uiWindow struct {
	mu   sync.Mutex
	html string
	dom   []uiDom
	spare []uiDom
	scene    string
	cuts     []int32
	insets   string
	worldGen uint32
	docGen   uint32
	handlers map[string][]UiAttr
	paced    bool

	watchers map[NetWsConnection]uint32
	holding  map[NetWsConnection]uint32
	token    string
	addr     SyslinkAddress
	listener net.Listener
	shut     bool
	painter  uiPainter
}

type uiPainter interface {
	show(v UiView)
	open(title string, addr SyslinkAddress) bool
	last() UiView
}

var uiNativePainter func() uiPainter

type uiWatcher struct {
	conn NetWsConnection
	near bool
	read bool
}

func (w *uiWindow) publish(v UiView) {
	w.mu.Lock()
	painter := w.painter
	w.mu.Unlock()
	if painter != nil {
		painter.show(v)
		return
	}
	r := &uiRender{handlers: map[string][]UiAttr{}}

w.mu.Lock()
room := len(w.html)
spare := w.spare
w.spare = nil
w.mu.Unlock()
if room > 0 {
	r.out.Grow(room + room/8)
}
r.view(v)
scene, cuts := uiSceneWorld(r.scenes, r.sceneCuts)
insets := uiSceneLooks(r.insets)
w.mu.Lock()
html := r.out.String()
document := html != w.html
world := scene != "" && (scene != w.scene || insets != w.insets)
was, wasCuts := w.scene, w.cuts
wasHtml, wasDom := w.html, w.dom
w.html = html
w.scene = scene
w.cuts = cuts
w.insets = insets
w.handlers = r.handlers
w.paced = r.paced
if world {
	w.worldGen++
}

if document {
	w.docGen++
	if len(w.watchers) > 0 {
		w.dom = uiScan(html, spare)
	} else {
		w.dom = nil
	}
}
seq := w.worldGen
papers := w.docGen
dom := w.dom
watchers := make([]uiWatcher, 0, len(w.watchers))
for c, held := range w.watchers {
	watchers = append(watchers, uiWatcher{
		conn: c,
		near: held == seq-1,
		read: w.holding[c] == papers-1,
	})
	if world {
		w.watchers[c] = seq
	}
	if document {
		w.holding[c] = papers
	}
}
w.mu.Unlock()
var edit, whole []byte
if world {
	edit = uiSceneFrame(scene, cuts, insets, was, wasCuts, seq, seq-1)
}

paper := ""
if document && wasDom != nil && dom != nil {
	edits := uiDocEdits(wasHtml, wasDom, html, dom)
	if len(edits) > 0 {
		paper = uiEditsFrame(edits)
	}
}
for _, c := range watchers {
	NetWsSend(c.conn, "A")
	if document {
		if c.read && paper != "" {
			NetWsSend(c.conn, paper)
		} else {
			NetWsSend(c.conn, "H"+html)
		}
	}
	if !world {
		continue
	}

	if c.near {
		uiWsSend(c.conn, edit)
		continue
	}
	if whole == nil {
		whole = uiSceneFrame(scene, cuts, insets, "", nil, seq, 0)
	}
	uiWsSend(c.conn, whole)
}

	w.mu.Lock()
	if document {
		w.spare = wasDom
	} else {
		w.spare = spare
	}
	w.mu.Unlock()
}

func uiSceneWorld(scenes []string, cuts [][]int32) (string, []int32) {
	if len(scenes) == 0 {
		return "", nil
	}

	if len(scenes) > 1 {
		uiSceneCrowd.Do(func() {
			fmt.Fprintln(os.Stderr, "hive: a window draws one scene, and this view has " +
				strconv.Itoa(len(scenes)) + " — the first one is the one on screen")
		})
	}
	return scenes[0], cuts[0]
}

func uiSceneLooks(insets []string) string {
	many := len(insets)
	if many > 255 {
		many = 255
	}
	var b strings.Builder
	uiPackU8(&b, many)
	for i := 0; i < many; i++ {
		b.WriteString(insets[i])
	}
	return b.String()
}

func uiSceneFrame(scene string, cuts []int32, insets string,
	was string, wasCuts []int32, seq, prev uint32) []byte {
	head := len(scene)
	if len(cuts) > 0 {
		head = int(cuts[0])
	}
	shapes := len(cuts) - 1
	if shapes < 0 {
		shapes = 0
	}
	held := 0
	if prev != 0 && len(wasCuts) > 0 {
		held = len(wasCuts) - 1
	}
	moves, changed := uiSceneDiff(scene, cuts, was, wasCuts, shapes, held)

size := 9 + head + len(insets) + 4 + len(moves)*4 + 4
for _, at := range changed {
	size += 4 + int(cuts[at+1]-cuts[at])
}
out := make([]byte, 0, size)
out = append(out, 'S')
out = uiAppendU32(out, int(seq))
out = uiAppendU32(out, int(prev))

	out = append(out, scene[:head]...)
	out = append(out, insets...)
	out = uiAppendU32(out, len(moves)/3)
	for _, n := range moves {
		out = uiAppendU32(out, int(n))
	}
	out = uiAppendU32(out, len(changed))
	for _, at := range changed {
		out = uiAppendU32(out, int(at))
		out = append(out, scene[cuts[at]:cuts[at+1]]...)
	}
	return out
}

func uiAppendU32(out []byte, v int) []byte {
	return append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

var uiSceneSeed = maphash.MakeSeed()

func uiSceneDiff(scene string, cuts []int32, was string, wasCuts []int32, shapes, held int) ([]int32, []int32) {
	rec := func(i int) string { return scene[cuts[i]:cuts[i+1]] }
	old := func(i int) string { return was[wasCuts[i]:wasCuts[i+1]] }
	same := func(i int) bool { return i < held && i < shapes && rec(i) == old(i) }
	changed := make([]int32, 0, shapes)
	var moves []int32
	var index map[uint64]int32
	var taken []bool
	for i := 0; i < shapes; i++ {
		if same(i) {
			continue
		}
		if held > 0 {
			if index == nil {
				index = make(map[uint64]int32, held)
				for j := held - 1; j >= 0; j-- {
					index[maphash.String(uiSceneSeed, old(j))] = int32(j)
				}
				taken = make([]bool, held)
			}
			if at, ok := index[maphash.String(uiSceneSeed, rec(i))]; ok {
				src := int(at)
				n := 0
				for i+n < shapes && src+n < held && !taken[src+n] && !same(i+n) && !same(src+n) && rec(i+n) == old(src+n) {
					taken[src+n] = true
					n++
				}
				if n > 0 {
					moves = append(moves, int32(i), int32(src), int32(n))
					i += n - 1
					continue
				}
			}
		}
		changed = append(changed, int32(i))
	}
	return moves, changed
}

func uiWsSend(c NetWsConnection, payload []byte) {
	if c.ws == nil {
		return
	}
	_ = c.ws.writeFrame(wsBinary, payload)
}

func uiPackU8(b *strings.Builder, v int) { b.WriteByte(byte(v)) }

func uiPackU16(b *strings.Builder, v int) {
	b.WriteByte(byte(v))
	b.WriteByte(byte(v >> 8))
}

func uiPackU32(b *strings.Builder, v int) {
	b.WriteByte(byte(v))
	b.WriteByte(byte(v >> 8))
	b.WriteByte(byte(v >> 16))
	b.WriteByte(byte(v >> 24))
}

func uiPackF32(b *strings.Builder, v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	uiPackU32(b, int(math.Float32bits(float32(v))))
}

func uiPackWord(b *strings.Builder, s string) {
	if len(s) > 255 {
		s = uiPackCut(s, 255)
	}
	b.WriteByte(byte(len(s)))
	b.WriteString(s)
}

func uiPackLong(b *strings.Builder, s string) {
	if len(s) > 65535 {
		s = uiPackCut(s, 65535)
	}
	uiPackU16(b, len(s))
	b.WriteString(s)
}

func uiPackCut(s string, most int) string {
	for most > 0 && s[most]&0xC0 == 0x80 {
		most--
	}
	return s[:most]
}

var uiSceneCrowd sync.Once

func uiPair(text string) (float64, float64) {
	at := strings.IndexByte(text, ',')
	if at < 0 {
		return 0, 0
	}
	x, errX := strconv.ParseFloat(strings.TrimSpace(text[:at]), 64)
	y, errY := strconv.ParseFloat(strings.TrimSpace(text[at+1:]), 64)
	if errX != nil || errY != nil {
		return 0, 0
	}
	return x, y
}

func uiControl(text string) (string, float64) {
	at := strings.IndexByte(text, '|')
	if at < 0 {
		return "", 0
	}
	position, err := strconv.ParseFloat(strings.TrimSpace(text[at+1:]), 64)
	if err != nil {
		return "", 0
	}
	return text[:at], position
}

func (w *uiWindow) linger() {
	// Where a host owns the window, it is the one that ends the program, and
	// a sleeping device can hold a socket quiet for minutes. Exiting on a
	// quiet socket there would read as a crash.
	if runtime.GOOS == "android" {
		return
	}
	time.Sleep(1500 * time.Millisecond)
	w.mu.Lock()
	gone := len(w.watchers) == 0 && !w.shut
	if gone {
		w.shut = true
	}
	addr, listener := w.addr, w.listener
	w.mu.Unlock()
	if !gone {
		return
	}
	SyslinkKill(addr)
	if listener != nil {
		listener.Close()
	}
	if Release() == 0 {
		fmt.Fprintln(os.Stderr, "hive: the window was closed")
		os.Exit(0)
	}
}

func (w *uiWindow) message(id string, kind string, value string, index int) (any, bool) {
	w.mu.Lock()
	attrs, ok := w.handlers[id]
	w.mu.Unlock()
	if !ok {
		return nil, false
	}
	return uiMeaning(attrs, kind, value, index)
}

// "12,30,rest" as 12, 30 and whatever follows the second comma, commas and all:
// an edit's text, a selection's or a view's nothing.
func uiCut(value string) (int, int, string) {
	a, rest, _ := strings.Cut(value, ",")
	b, text, _ := strings.Cut(rest, ",")
	x, _ := strconv.Atoi(a)
	y, _ := strconv.Atoi(b)
	return x, y, text
}

func uiMeaning(attrs []UiAttr, kind string, value string, index int) (any, bool) {
	for _, a := range attrs {
		h := a.heard
		if h == nil {
			continue
		}
		switch {
		case kind == "click" && a.Kind == "on":
			return h.Msg, true
		case kind == "click" && a.Kind == "onPick" && h.FnInt != nil:
			return h.FnInt(index), true
		case kind == "click" && a.Kind == "onSort" && h.FnInt != nil:
			return h.FnInt(index), true
		case kind == "input" && a.Kind == "onInput" && h.FnStr != nil:
			return h.FnStr(value), true
		case kind == "submit" && a.Kind == "onSubmit" && h.FnStr != nil:
			return h.FnStr(value), true
		case kind == "key" && a.Kind == "onKey" && h.FnStr != nil:
			return h.FnStr(value), true
		case kind == "paste" && a.Kind == "onPaste" && h.FnStr != nil:
			return h.FnStr(value), true
		case kind == "caret" && a.Kind == "onCaret" && h.FnSize != nil:
			anchor, head, _ := uiCut(value)
			return h.FnSize(anchor, head), true
		case kind == "view" && a.Kind == "onView" && h.FnSize != nil:
			first, last, _ := uiCut(value)
			return h.FnSize(first, last), true
		case kind == "fit" && a.Kind == "onFit" && h.FnSize != nil:
			cols, rows, _ := uiCut(value)
			return h.FnSize(cols, rows), true
		case kind == "edit" && a.Kind == "onEdit" && h.FnEdit != nil:
			from, to, text := uiCut(value)
			return h.FnEdit(from, to, text), true
		case kind == "menu" && a.Kind == "onMenu", kind == "middle" && a.Kind == "onMiddle",
			kind == "double" && a.Kind == "onDouble":
			return h.Msg, true
		case kind == "drag" && a.Kind == "onDrag" && h.FnDrag != nil:
			x, y, done := uiCut(value)
			return h.FnDrag(x, y, done == "1"), true
		case kind == "hover" && a.Kind == "onHover" && h.FnInt != nil,
			kind == "jump" && a.Kind == "onJump" && h.FnInt != nil,
			kind == "probe" && a.Kind == "onProbe" && h.FnInt != nil:
			at, _ := strconv.Atoi(value)
			return h.FnInt(at), true
		case kind == "choose" && a.Kind == "onChoose" && h.FnStr != nil:
			return h.FnStr(value), true
		case kind == "toggle" && a.Kind == "onToggle" && h.FnBool != nil:
			return h.FnBool(value != ""), true
		case kind == "frame" && a.Kind == "onFrame" && h.FnInt != nil:
			return h.FnInt(index), true
		case kind == "rate" && a.Kind == "onRate" && h.FnInt != nil:
			return h.FnInt(index), true
		case kind == "keydown" && a.Kind == "onKeyDown" && h.FnStr != nil:
			return h.FnStr(value), true
		case kind == "keyup" && a.Kind == "onKeyUp" && h.FnStr != nil:
			return h.FnStr(value), true
		case kind == "look" && a.Kind == "onLook" && h.FnLook != nil:
			dx, dy := uiPair(value)
			return h.FnLook(dx, dy), true
		case kind == "grab" && a.Kind == "onGrab" && h.FnBool != nil:
			return h.FnBool(value != ""), true

		case kind == "pad" && a.Kind == "onPad" && h.FnPad != nil:
			control, position := uiControl(value)
			return h.FnPad(index, control, position), true
		case kind == "size" && a.Kind == "onSize" && h.FnSize != nil:
			wide, tall := uiPair(value)
			return h.FnSize(int(wide), int(tall)), true
		}
	}
	return nil, false
}

func UiWindow[S any, M any](title string, view func(S) UiView, update func(*S, M) M) func(*S, M) M {
	handler := func(state *S, msg M) M { return update(state, msg) }
	startsAs(funcKey(handler), uiStarter[S, M](title, view, false))
	return handler
}

func UiWindowAddressed[S any, M any](title string, view func(S) UiView, update func(*S, M, SyslinkAddress) M) func(*S, M, SyslinkAddress) M {
	handler := func(state *S, msg M, self SyslinkAddress) M { return update(state, msg, self) }
	startsAs(funcKey(handler), uiStarter[S, M](title, view, false))
	return handler
}

func UiWebview[S any, M any](title string, view func(S) UiView, update func(*S, M) M) func(*S, M) M {
	handler := func(state *S, msg M) M { return update(state, msg) }
	startsAs(funcKey(handler), uiStarter[S, M](title, view, true))
	return handler
}

func UiWebviewAddressed[S any, M any](title string, view func(S) UiView, update func(*S, M, SyslinkAddress) M) func(*S, M, SyslinkAddress) M {
	handler := func(state *S, msg M, self SyslinkAddress) M { return update(state, msg, self) }
	startsAs(funcKey(handler), uiStarter[S, M](title, view, true))
	return handler
}

func uiStarter[S any, M any](title string, view func(S) UiView, page bool) syslinkStarter {
	return func(given any, first any) (any, func(SyslinkAddress)) {
		update := given.(func(*S, M, syslinkTurn) M)
		w := &uiWindow{
			watchers: map[NetWsConnection]uint32{},
			holding:  map[NetWsConnection]uint32{},
			token:    CryptoRandomHex(16),
		}
		if uiNativePainter != nil && !page && os.Getenv("HIVE_WINDOW") != "print" {
			w.painter = uiNativePainter()
			uiCrashLog()
		}
		tally := uiFoldTally()
		folded := func(state *S, msg M, t syslinkTurn) M {
			began := time.Now()
			out := update(state, msg, t)
			w.mu.Lock()
			paced := w.paced
			w.mu.Unlock()
			if paced && !t.drawn {
				tally.fold(began, time.Time{}, time.Time{})
				return out
			}
			viewed := time.Now()
			v := view(*state)
			drawn := time.Now()
			w.publish(v)
			tally.fold(began, viewed, drawn)
			return out
		}

		w.publish(view(first.(S)))
		return folded, func(addr SyslinkAddress) { uiShow(w, title, addr) }
	}
}

func uiShow(w *uiWindow, title string, named SyslinkAddress) {
	addr := SyslinkAddress{Name: atomNone, Id: named.Id, box: named.box}
	w.mu.Lock()
	w.addr = addr
	painter := w.painter
	w.mu.Unlock()
	if painter != nil {
		if painter.open(title, addr) {
			return
		}
		w.mu.Lock()
		w.painter = nil
		w.mu.Unlock()
		w.publish(painter.last())
	}

	page := strings.NewReplacer(
		"__TITLE__", uiEscape(title),
		"__CSS__", uiCSS,
		"__TOKEN__", w.token,
		"__EXTRA__", uiExtraScript+strings.Join(hivePageScripts, ""),
		"__ICON__", uiIconHref(),
	).Replace(uiShell)

	ready := make(chan int, 1)
	go uiServe(w, addr, page, ready)
	bound := <-ready
	if bound == 0 {
		fmt.Fprintln(os.Stderr, "hive: the window could not be opened — no port could be bound")
		return
	}
	Hold()
	url := "http://127.0.0.1:" + strconv.Itoa(bound) + "/?token=" + w.token
	if err := uiOpen(url); err != nil {

		fmt.Fprintln(os.Stderr, "hive: open " + url)
	}
}

type uiTally struct {
	on      bool
	mark    time.Time
	folds   int
	drawn   int
	gcs     uint32
	paused  uint64
	update  []float64
	view    []float64
	publish []float64
}

func uiFoldTally() *uiTally {
	return &uiTally{on: os.Getenv("HIVE_FOLDS") != "", mark: time.Now()}
}

func (t *uiTally) fold(began, viewed, drawn time.Time) {
	if !t.on {
		return
	}
	now := time.Now()
	t.folds++
	if viewed.IsZero() {
		t.update = append(t.update, uiMs(now.Sub(began)))
	} else {
		t.drawn++
		t.update = append(t.update, uiMs(viewed.Sub(began)))
		t.view = append(t.view, uiMs(drawn.Sub(viewed)))
		t.publish = append(t.publish, uiMs(now.Sub(drawn)))
	}
	if now.Sub(t.mark) < time.Second {
		return
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(os.Stderr, "hive: folds %d/s drawn %d/s | update %s | view %s | publish %s | gc %d, %.1fms\n",
		t.folds, t.drawn, uiSpread(t.update), uiSpread(t.view), uiSpread(t.publish),
		m.NumGC-t.gcs, float64(m.PauseTotalNs-t.paused)/1e6)
	t.gcs, t.paused = m.NumGC, m.PauseTotalNs
	t.mark, t.folds, t.drawn = now, 0, 0
	t.update, t.view, t.publish = t.update[:0], t.view[:0], t.publish[:0]
}

func uiMs(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func uiSpread(xs []float64) string {
	if len(xs) == 0 {
		return "-"
	}
	s := append([]float64(nil), xs...)
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return fmt.Sprintf("%.1f/%.1f/%.1fms", s[len(s)/2], s[len(s)*9/10], s[len(s)-1])
}

var uiExtraScript string
var uiExtraRoutes = map[string]func(http.ResponseWriter, *http.Request){}

var uiAssets *embed.FS

func uiAssetsAre(files embed.FS) { uiAssets = &files }

func uiStored(rw http.ResponseWriter, req *http.Request) {
	if uiAssets == nil {
		http.Error(rw, "not found", http.StatusNotFound)
		return
	}
	name := strings.TrimPrefix(req.URL.Path, "/_hive/asset/")
	if name == "" || strings.Contains(name, "..") {
		http.Error(rw, "not found", http.StatusNotFound)
		return
	}
	body, err := uiAssets.ReadFile("assets/" + name)
	if err != nil {
		http.Error(rw, "not found", http.StatusNotFound)
		return
	}
	rw.Header().Set("Content-Type", uiMime(name))

	rw.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	rw.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = rw.Write(body)
}

func uiMime(name string) string {
	cut := strings.LastIndex(name, ".")
	if cut < 0 {
		return "application/octet-stream"
	}
	switch strings.ToLower(name[cut:]) {
	case ".ogg", ".opus":
		return "audio/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a", ".aac":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".woff2":
		return "font/woff2"
	}
	return "application/octet-stream"
}

func init() { uiExtraRoutes["/_hive/asset/"] = uiStored }

func uiServe(w *uiWindow, addr SyslinkAddress, page string, ready chan int) {
	mux := http.NewServeMux()
	for path, handler := range uiExtraRoutes {
		mux.HandleFunc(path, handler)
	}
	for path, handler := range hivePageRoutes {
		if route, ok := handler.(func(http.ResponseWriter, *http.Request)); ok {
			mux.HandleFunc(path, route)
		}
	}
	if len(uiWindowIcon) > 0 {
		mux.HandleFunc("/_hive/icon", uiIconRoute)
	}
	mux.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("token") != w.token {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(rw, page)
	})
	mux.HandleFunc("/_hive/socket", func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("token") != w.token {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		socket, err := wsAccept(rw, req)
		if err != nil {
			return
		}
		conn := NetWsConnection{ws: socket}
		defer socket.close()
		w.mu.Lock()
		w.watchers[conn] = w.worldGen
		w.holding[conn] = w.docGen
		current := w.html
		w.mu.Unlock()
		NetWsSend(conn, "H"+current)
		w.reseed(conn)
		uiPump(w, addr, conn)
		w.mu.Lock()
		delete(w.watchers, conn)
		delete(w.holding, conn)
		empty := len(w.watchers) == 0
		w.mu.Unlock()

		if empty {
			go w.linger()
		}
	})
	listener, bound := uiListen()
	if listener == nil {
		ready <- 0
		return
	}
	w.mu.Lock()
	w.listener = listener
	w.mu.Unlock()
	ready <- bound
	_ = http.Serve(listener, mux)
}

func (w *uiWindow) reseed(conn NetWsConnection) {
	w.mu.Lock()
	scene, cuts, insets, seq := w.scene, w.cuts, w.insets, w.worldGen
	if _, watching := w.watchers[conn]; watching {
		w.watchers[conn] = seq
	}
	w.mu.Unlock()
	if scene == "" {
		return
	}
	uiWsSend(conn, uiSceneFrame(scene, cuts, insets, "", nil, seq, 0))
}

func uiPump(w *uiWindow, addr SyslinkAddress, conn NetWsConnection) {
	for {
		got := NetWsReceive(conn)
		if got.IsError() {
			return
		}
		id, kind, value, index := uiEvent(got.Ok())
		if id == "" {
			continue
		}

		if kind == "resync" {
			w.reseed(conn)
			continue
		}
		msg, ok := w.message(id, kind, value, index)
		switch {
		case ok && uiDrawnAfter(kind):
			syslinkPostDrawn(addr, msg)
		case ok:
			SyslinkPost(addr, msg)
		case kind == "frame":
			NetWsSend(conn, "A")
		}
	}
}

// A window drawn by the program itself usually has no console, so what a
// crash prints would go nowhere: it is also kept, appended, in
// <temp>/<program>-crash.log.
var uiCrashOnce sync.Once

func uiCrashLog() {
	uiCrashOnce.Do(func() {
		name := strings.TrimSuffix(filepath.Base(os.Args[0]), filepath.Ext(os.Args[0]))
		path := filepath.Join(os.TempDir(), name+"-crash.log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		debug.SetCrashOutput(f, debug.CrashOptions{})
		f.Close()
	})
}

func uiDrawnAfter(kind string) bool {
	switch kind {
	case "frame", "click", "input", "submit", "choose", "toggle", "key", "edit", "menu", "middle", "double", "jump":
		return true
	}
	return false
}

func uiEvent(text string) (string, string, string, int) {
	id := uiField(text, "id")
	kind := uiField(text, "kind")
	value := uiField(text, "value")
	index, _ := strconv.Atoi(uiField(text, "index"))
	return id, kind, value, index
}

func uiField(text string, name string) string {
	key := "\"" + name + "\":"
	at := strings.Index(text, key)
	if at < 0 {
		return ""
	}
	rest := strings.TrimLeft(text[at+len(key):], " ")
	if rest == "" {
		return ""
	}
	if rest[0] != '"' {
		end := strings.IndexAny(rest, ",}")
		if end < 0 {
			end = len(rest)
		}
		return strings.TrimSpace(rest[:end])
	}
	var b strings.Builder
	for i := 1; i < len(rest); i++ {
		if rest[i] == '\\' && i+1 < len(rest) {
			i++
			switch rest[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'u':
				if i+4 < len(rest) {
					if n, err := strconv.ParseInt(rest[i+1:i+5], 16, 32); err == nil {
						b.WriteRune(rune(n))
						i += 4
					}
				}
			default:
				b.WriteByte(rest[i])
			}
			continue
		}
		if rest[i] == '"' {
			break
		}
		b.WriteByte(rest[i])
	}
	return b.String()
}

var uiWindowIcon []byte

func uiWindowIconIs(body []byte) { uiWindowIcon = body }

func uiIconHref() string {
	if len(uiWindowIcon) > 0 {
		return "/_hive/icon"
	}
	return "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'" +
		"%3E%3Cpath fill='%23f0b429' d='M12 1.5 21 6.75v10.5L12 22.5 3 17.25V6.75z'/%3E%3C/svg%3E"
}

func uiIconRoute(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "image/png")
	rw.Header().Set("Cache-Control", "max-age=31536000, immutable")
	rw.Write(uiWindowIcon)
}

func uiListen() (net.Listener, int) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "hive: ui.window could not bind a local port: " + err.Error())
		return nil, 0
	}
	return listener, listener.Addr().(*net.TCPAddr).Port
}

func uiAppArguments(url string) []string {
	arguments := []string{"--app=" + url, "--window-size=1100,780"}
	profile := uiProfileDir()
	if profile == "" {
		return arguments
	}
	arguments = append(arguments,
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check")

	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		arguments = append(arguments, "--class="+uiAppName())
	}
	return arguments
}

func uiProfileDir() string {
	root, err := os.UserCacheDir()
	if err != nil {
		root = os.TempDir()
	}

	sum := fnv.New32a()
	if path, err := os.Executable(); err == nil {
		sum.Write([]byte(path))
	} else {
		sum.Write([]byte(os.Args[0]))
	}
	dir := filepath.Join(root, "hive", "windows", uiAppName()+"-"+strconv.FormatUint(uint64(sum.Sum32()), 16))
	if os.MkdirAll(dir, 0o700) != nil {
		return ""
	}
	return dir
}

func uiAppName() string {
	name := filepath.Base(os.Args[0])
	name = strings.TrimSuffix(name, ".exe")
	kept := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		}
		return -1
	}, name)
	if kept == "" {
		return "hive"
	}
	return kept
}

func uiOpen(url string) error {
	// A handset has no browser to launch and a host in front of it that
	// opens the window itself. Said in a shape meant to be read rather than
	// left to be picked out of prose.
	if runtime.GOOS == "android" || os.Getenv("HIVE_WINDOW") == "print" {
		fmt.Println("hive-window " + url)
		return nil
	}
	arguments := uiAppArguments(url)
	for _, candidate := range uiBrowsers() {
		path, err := exec.LookPath(candidate)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, arguments...)
		if err := cmd.Start(); err == nil {
			return nil
		}
	}
	if flatpak, err := exec.LookPath("flatpak"); err == nil {
		for _, id := range uiFlatpakBrowsers() {

			if exec.Command(flatpak, "info", id).Run() != nil {
				continue
			}
			cmd := exec.Command(flatpak, append([]string{"run", id}, arguments...)...)
			if cmd.Start() == nil {
				return nil
			}
		}
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func uiFlatpakBrowsers() []string {
	return []string{
		"com.google.Chrome",
		"org.chromium.Chromium",
		"com.microsoft.Edge",
		"com.brave.Browser",
		"com.vivaldi.Vivaldi",
		"io.github.ungoogled_software.ungoogled_chromium",
	}
}

func uiBrowsers() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			"chrome", "msedge", "chromium",
			"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe",
		}
	case "darwin":
		return []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}
	default:
		return []string{
			"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
			"microsoft-edge", "brave-browser", "vivaldi",
		}
	}
}
