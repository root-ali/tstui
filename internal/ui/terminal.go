package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/x/vt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/root-ali/tstui/internal/session"
	"github.com/root-ali/tstui/internal/vterm"
)

// TerminalView is the embedded terminal: it draws the active session's
// virtual terminal into the pane, sends keys, pastes and mouse events to
// it, keeps the session's size equal to the pane's, and implements scroll
// mode and mouse selection.
//
// Keys reach it through HandleKey: the application's input capture
// decides what is a tstui key (the prefix) and what belongs to the
// terminal. All methods run on the UI goroutine.
type TerminalView struct {
	*tview.Box
	theme Theme

	sess        *session.Session
	placeholder string
	// banner returns the line shown at the bottom of a session that is
	// not running; nil uses the default.
	banner func(*session.Session) string
	// onSize is told the inner size on every draw (for new sessions).
	onSize func(cols, rows int)
	// onFocus asks the application to focus the terminal (a click).
	onFocus func()
	// onCopy reports a copy to the clipboard (for a footer message).
	onCopy func(chars int)

	scrolling bool // scroll mode (Ctrl+B [ or the mouse wheel)
	offset    int  // lines scrolled up from the live screen

	sel struct {
		on, dragging bool
		from, to     cellPos
		// wordAt and wordLine are when and on which line the last
		// double-click selected a word: a click right after it on the
		// same line (a triple-click) selects the line.
		wordAt   time.Time
		wordLine int
	}
	buttons tcell.ButtonMask // mouse buttons held, for forwarding releases

	screen tcell.Screen // from the last Draw, for SetClipboard
	rows   int          // inner height at the last Draw
}

type cellPos struct{ line, col int }

func (p cellPos) before(q cellPos) bool {
	return p.line < q.line || (p.line == q.line && p.col < q.col)
}

// NewTerminalView returns an empty terminal pane.
func NewTerminalView(theme Theme) *TerminalView {
	v := &TerminalView{Box: tview.NewBox(), theme: theme}
	v.SetBorder(true).SetBorderColor(theme.Border).SetTitleColor(theme.Title)
	return v
}

// SetSession shows s (nil for none). Scroll mode and the selection end.
func (v *TerminalView) SetSession(s *session.Session) {
	if v.sess != s {
		v.exitScroll()
		v.sel.on, v.sel.dragging = false, false
	}
	v.sess = s
}

// Session is the session shown.
func (v *TerminalView) Session() *session.Session { return v.sess }

// SetPlaceholder sets the text shown when there is no session.
func (v *TerminalView) SetPlaceholder(text string) { v.placeholder = text }

// Scrolling reports whether scroll mode is on.
func (v *TerminalView) Scrolling() bool { return v.scrolling }

// EnterScroll turns scroll mode on.
func (v *TerminalView) EnterScroll() {
	if v.sess != nil {
		v.scrolling = true
	}
}

func (v *TerminalView) exitScroll() { v.scrolling, v.offset = false, 0 }

func (v *TerminalView) running() bool {
	if v.sess == nil {
		return false
	}
	st, _, _ := v.sess.State()
	return st.Running()
}

// Draw implements tview.Primitive.
func (v *TerminalView) Draw(screen tcell.Screen) {
	v.DrawForSubclass(screen, v)
	v.screen = screen
	x, y, w, h := v.GetInnerRect()
	v.rows = h
	if w <= 0 || h <= 0 {
		return
	}
	if v.onSize != nil {
		v.onSize(w, h)
	}
	if v.sess == nil {
		v.drawPlaceholder(screen, x, y, w, h)
		return
	}
	v.sess.Resize(w, h)
	if v.HasFocus() {
		v.sess.ClearActivity()
	}

	running := v.running()
	v.sess.Terminal().View(func(s *vterm.Screen) {
		sb := s.ScrollbackLen()
		v.offset = min(v.offset, sb)
		top := sb - v.offset
		cols, _ := s.Size()
		for row := range h {
			line := top + row
			if line >= s.Lines() {
				break
			}
			for col := 0; col < min(cols, w); {
				c := s.Cell(line, col)
				if c.Width == 0 {
					col++
					continue
				}
				main, comb := c.Runes()
				style := c.Style
				if v.selected(cellPos{line, col}) {
					style = style.Reverse(true)
				}
				screen.SetContent(x+col, y+row, main, comb, style)
				col += max(c.Width, 1)
			}
		}
		cx, cy, visible := s.Cursor()
		if v.offset == 0 && visible && running && v.HasFocus() && !v.scrolling && cx < w && cy < h {
			screen.ShowCursor(x+cx, y+cy)
			screen.SetCursorStyle(cursorStyle(s.CursorStyle()))
		}
		if v.scrolling || v.offset > 0 {
			v.drawCorner(screen, x, y, w, fmt.Sprintf(" scroll %d/%d · q exit ", v.offset, sb))
		}
	})
	if !running {
		v.drawBanner(screen, x, y+h-1, w)
	}
}

func cursorStyle(style vt.CursorStyle, blink bool) tcell.CursorStyle {
	switch {
	case style == vt.CursorUnderline && blink:
		return tcell.CursorStyleBlinkingUnderline
	case style == vt.CursorUnderline:
		return tcell.CursorStyleSteadyUnderline
	case style == vt.CursorBar && blink:
		return tcell.CursorStyleBlinkingBar
	case style == vt.CursorBar:
		return tcell.CursorStyleSteadyBar
	case blink:
		return tcell.CursorStyleBlinkingBlock
	}
	return tcell.CursorStyleSteadyBlock
}

func (v *TerminalView) drawPlaceholder(screen tcell.Screen, x, y, w, h int) {
	lines := strings.Split(v.placeholder, "\n")
	top := y + max(0, (h-len(lines))/2)
	for i, l := range lines {
		tview.Print(screen, l, x, top+i, w, tview.AlignCenter, v.theme.DialogPlaceholder)
	}
}

func (v *TerminalView) drawCorner(screen tcell.Screen, x, y, w int, text string) {
	n := tview.TaggedStringWidth(text)
	style := tcell.StyleDefault.Foreground(v.theme.TableCursorFg).Background(v.theme.TableCursorBg)
	for i, r := range []rune(text) {
		if i >= w {
			break
		}
		screen.SetContent(x+max(0, w-n)+i, y, r, nil, style)
	}
}

// DefaultBanner is the bottom line of a session that is not running.
func DefaultBanner(s *session.Session) string {
	st, code, err := s.State()
	if st == session.Failed && err != nil {
		return fmt.Sprintf("✘ %s · r reconnect, c close", err)
	}
	return fmt.Sprintf("Session closed (exit %d). r reconnect, c close", code)
}

func (v *TerminalView) drawBanner(screen tcell.Screen, x, y, w int) {
	text := DefaultBanner(v.sess)
	if v.banner != nil {
		text = v.banner(v.sess)
	}
	st, _, _ := v.sess.State()
	bg := v.theme.Warn
	if st == session.Failed {
		bg = v.theme.Err
	}
	style := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(bg).Bold(true)
	for col := range w {
		screen.SetContent(x+col, y, ' ', nil, style)
	}
	tview.Print(screen, fmt.Sprintf("[black:%s:b]%s", tag(bg), tview.Escape(" "+text+" ")), x, y, w, tview.AlignLeft, tcell.ColorBlack)
}

// HandleKey handles a key for the terminal: scroll mode keys, r/c on a
// closed session, or input for the program.
func (v *TerminalView) HandleKey(ev *tcell.EventKey, reconnect, closeTab func()) {
	if v.sess == nil {
		return
	}
	if v.scrolling {
		v.scrollKey(ev)
		return
	}
	if !v.running() {
		switch ev.Rune() {
		case 'r':
			reconnect()
		case 'c':
			closeTab()
		}
		return
	}
	v.sel.on = false
	v.offset = 0
	v.sess.Send(vterm.EncodeKey(ev, v.sess.Terminal().Modes()))
}

func (v *TerminalView) scrollKey(ev *tcell.EventKey) {
	page := max(v.rows-1, 1)
	switch ev.Key() {
	case tcell.KeyPgUp:
		v.offset += page
	case tcell.KeyPgDn, tcell.KeyCtrlF:
		v.offset -= page
	case tcell.KeyCtrlU:
		v.offset += page / 2
	case tcell.KeyCtrlD:
		v.offset -= page / 2
	case tcell.KeyUp:
		v.offset++
	case tcell.KeyDown:
		v.offset--
	case tcell.KeyHome:
		v.offset = 1 << 30
	case tcell.KeyEnd:
		v.offset = 0
	case tcell.KeyEsc:
		v.exitScroll()
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'k':
			v.offset++
		case 'j':
			v.offset--
		case 'g':
			v.offset = 1 << 30
		case 'G':
			v.offset = 0
		case 'q':
			v.exitScroll()
		}
	}
	v.offset = max(v.offset, 0) // Draw clamps the top
}

// PasteHandler implements tview.Primitive: the pasted text goes to the
// program, bracketed if it asked for that.
func (v *TerminalView) PasteHandler() func(string, func(tview.Primitive)) {
	return v.WrapPasteHandler(func(text string, _ func(tview.Primitive)) {
		if !v.running() || v.scrolling {
			return
		}
		v.offset = 0
		v.sess.Send(vterm.EncodePaste(text, v.sess.Terminal().Modes()))
	})
}

// MouseHandler implements tview.Primitive. A program that asked for mouse
// reports gets them (hold Shift to select instead); otherwise the wheel
// scrolls the scrollback, and dragging, a double-click (a word) or a
// triple-click (a line) selects text, copied to the clipboard with
// OSC 52.
func (v *TerminalView) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return v.WrapMouseHandler(func(action tview.MouseAction, ev *tcell.EventMouse, _ func(tview.Primitive)) (bool, tview.Primitive) {
		mx, my := ev.Position()
		if !v.InRect(mx, my) && !v.sel.dragging && v.buttons == 0 {
			return false, nil
		}
		if action == tview.MouseLeftDown || action == tview.MouseRightDown || action == tview.MouseMiddleDown {
			if v.onFocus != nil {
				v.onFocus()
			}
		}
		if v.sess == nil {
			return true, nil
		}
		x, y, w, h := v.GetInnerRect()
		col, row := min(max(mx-x, 0), w-1), min(max(my-y, 0), h-1)
		modes := v.sess.Terminal().Modes()
		if modes.MouseReporting() && v.running() && v.offset == 0 && ev.Modifiers()&tcell.ModShift == 0 {
			return v.forwardMouse(action, ev, col, row, modes)
		}
		return v.localMouse(action, col, row, modes)
	})
}

func (v *TerminalView) forwardMouse(action tview.MouseAction, ev *tcell.EventMouse, col, row int, modes vterm.Modes) (bool, tview.Primitive) {
	me := vterm.MouseEvent{X: col, Y: row, Alt: ev.Modifiers()&tcell.ModAlt != 0, Ctrl: ev.Modifiers()&tcell.ModCtrl != 0}
	held := ev.Buttons() & (tcell.Button1 | tcell.Button2 | tcell.Button3)
	switch action {
	case tview.MouseLeftDown:
		me.Button = vterm.MouseLeft
	case tview.MouseMiddleDown:
		me.Button = vterm.MouseMiddle
	case tview.MouseRightDown:
		me.Button = vterm.MouseRight
	case tview.MouseLeftUp:
		me.Button, me.Action = vterm.MouseLeft, vterm.MouseRelease
	case tview.MouseMiddleUp:
		me.Button, me.Action = vterm.MouseMiddle, vterm.MouseRelease
	case tview.MouseRightUp:
		me.Button, me.Action = vterm.MouseRight, vterm.MouseRelease
	case tview.MouseScrollUp:
		me.Button = vterm.MouseWheelUp
	case tview.MouseScrollDown:
		me.Button = vterm.MouseWheelDown
	case tview.MouseMove:
		me.Action = vterm.MouseMotion
		switch {
		case held&tcell.Button1 != 0:
			me.Button = vterm.MouseLeft
		case held&tcell.Button3 != 0:
			me.Button = vterm.MouseMiddle
		case held&tcell.Button2 != 0:
			me.Button = vterm.MouseRight
		default:
			me.Button = vterm.MouseNone
		}
	default:
		return true, nil // clicks are sent as down + up
	}
	v.buttons = held
	v.sess.Send(vterm.EncodeMouse(me, modes))
	if held != 0 {
		return true, v // keep getting events while a button is held
	}
	return true, nil
}

func (v *TerminalView) localMouse(action tview.MouseAction, col, row int, modes vterm.Modes) (bool, tview.Primitive) {
	var sb int
	v.sess.Terminal().View(func(s *vterm.Screen) { sb = s.ScrollbackLen() })
	pos := cellPos{line: sb - v.offset + row, col: col}
	switch action {
	case tview.MouseScrollUp:
		if modes.AltScreen && v.running() {
			// Like xterm's alternateScroll: the wheel moves the cursor in
			// full-screen programs (less, vim) that do not use the mouse.
			v.sess.Send([]byte(strings.Repeat(string(vterm.EncodeKey(tcell.NewEventKey(tcell.KeyUp, 0, 0), modes)), 3)))
			return true, nil
		}
		v.offset = min(v.offset+3, sb)
	case tview.MouseScrollDown:
		if modes.AltScreen && v.running() {
			v.sess.Send([]byte(strings.Repeat(string(vterm.EncodeKey(tcell.NewEventKey(tcell.KeyDown, 0, 0), modes)), 3)))
			return true, nil
		}
		v.offset = max(v.offset-3, 0)
	case tview.MouseLeftDown:
		v.sel.on, v.sel.dragging = false, true
		v.sel.from, v.sel.to = pos, pos
		return true, v
	case tview.MouseMove:
		if v.sel.dragging {
			v.sel.to = pos
			v.sel.on = v.sel.from != v.sel.to
			return true, v
		}
	case tview.MouseLeftUp:
		if v.sel.dragging {
			v.sel.dragging = false
			v.sel.to = pos
			v.sel.on = v.sel.from != v.sel.to
			if v.sel.on {
				v.copySelection()
			}
		}
	case tview.MouseLeftDoubleClick:
		if v.selectWord(pos) {
			v.sel.wordAt, v.sel.wordLine = time.Now(), pos.line
			v.copySelection()
		}
	case tview.MouseLeftClick:
		if !v.sel.wordAt.IsZero() && time.Since(v.sel.wordAt) <= tview.DoubleClickInterval && pos.line == v.sel.wordLine {
			v.sel.wordAt = time.Time{}
			if v.selectLine(pos.line) {
				v.copySelection()
			}
		}
	}
	return true, nil
}

// selectWord selects the word under p, like a terminal's double-click.
// It reports false (and selects nothing) on a blank or a lone symbol.
func (v *TerminalView) selectWord(p cellPos) bool {
	from, to, ok := p, p, false
	v.sess.Terminal().View(func(s *vterm.Screen) {
		cols, _ := s.Size()
		if p.line < 0 || p.line >= s.Lines() || p.col < 0 || p.col >= cols {
			return
		}
		at := p.col
		for at > 0 && s.Cell(p.line, at).Width == 0 {
			at-- // the right half of a wide character
		}
		if !isWordCell(s.Cell(p.line, at)) {
			return
		}
		lo, hi := at, at
		for lo > 0 {
			c := s.Cell(p.line, lo-1)
			if c.Width == 0 && lo-2 >= 0 {
				c = s.Cell(p.line, lo-2)
			}
			if !isWordCell(c) {
				break
			}
			lo--
		}
		for hi+1 < cols {
			c := s.Cell(p.line, hi+1)
			if c.Width != 0 && !isWordCell(c) {
				break
			}
			hi++
		}
		from, to, ok = cellPos{p.line, lo}, cellPos{p.line, hi}, true
	})
	if ok {
		v.sel.from, v.sel.to, v.sel.on = from, to, true
	}
	return ok
}

// selectLine selects all of line; false when the line is blank.
func (v *TerminalView) selectLine(line int) bool {
	var cols int
	var blank bool
	v.sess.Terminal().View(func(s *vterm.Screen) {
		cols, _ = s.Size()
		blank = line < 0 || line >= s.Lines() || s.LineText(line, 0, -1) == ""
	})
	if blank {
		return false
	}
	v.sel.from, v.sel.to, v.sel.on = cellPos{line, 0}, cellPos{line, cols - 1}, true
	return true
}

// wordSymbols are the characters, besides letters and digits, that a
// double-click keeps in a word, so paths, hostnames, IPs and user@host
// come out whole.
const wordSymbols = "_-./~+@%\\"

func isWordCell(c vterm.Cell) bool {
	r := []rune(c.Content)
	if len(r) == 0 {
		return false
	}
	return unicode.IsLetter(r[0]) || unicode.IsDigit(r[0]) || strings.ContainsRune(wordSymbols, r[0])
}

func (v *TerminalView) selected(p cellPos) bool {
	if !v.sel.on {
		return false
	}
	a, b := v.sel.from, v.sel.to
	if b.before(a) {
		a, b = b, a
	}
	return !p.before(a) && !b.before(p)
}

// SelectionText returns the selected text, lines joined with newlines.
func (v *TerminalView) SelectionText() string {
	if !v.sel.on || v.sess == nil {
		return ""
	}
	a, b := v.sel.from, v.sel.to
	if b.before(a) {
		a, b = b, a
	}
	var lines []string
	v.sess.Terminal().View(func(s *vterm.Screen) {
		for line := a.line; line <= b.line; line++ {
			x0, x1 := 0, -1
			if line == a.line {
				x0 = a.col
			}
			if line == b.line {
				x1 = b.col + 1
			}
			lines = append(lines, s.LineText(line, x0, x1))
		}
	})
	return strings.Join(lines, "\n")
}

func (v *TerminalView) copySelection() {
	text := v.SelectionText()
	if text == "" || v.screen == nil {
		return
	}
	v.screen.SetClipboard([]byte(text))
	if v.onCopy != nil {
		v.onCopy(len([]rune(text)))
	}
}
