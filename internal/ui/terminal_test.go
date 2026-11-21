package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/root-ali/tstui/internal/session"
	"github.com/root-ali/tstui/internal/vterm"
)

// termFixture is a TerminalView showing a session backed by session.Fake,
// drawn on a simulation screen, without the rest of the app.
type termFixture struct {
	t      *testing.T
	screen tcell.SimulationScreen
	view   *TerminalView
	sess   *session.Session
	proc   *session.FakeProcess
	mgr    *session.Manager
}

// The view's rect is 42x12, so the inner terminal is 40x10.
func newTermFixture(t *testing.T) *termFixture {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(60, 20)
	sp := &session.Fake{}
	mgr := session.NewManager(sp, session.Options{Cols: 80, Rows: 24})
	t.Cleanup(func() {
		mgr.CloseAll()
		screen.Fini()
	})
	s, _, err := mgr.Open(session.Spec{Proxy: "p", Host: "web-01", Login: "ali"}, false)
	if err != nil {
		t.Fatal(err)
	}
	v := NewTerminalView(DefaultTheme())
	v.SetRect(0, 0, 42, 12)
	v.SetSession(s)
	v.Focus(func(tview.Primitive) {})
	return &termFixture{t: t, screen: screen, view: v, sess: s, proc: sp.Proc(0), mgr: mgr}
}

// output feeds the program's output and waits until the terminal shows want.
func (f *termFixture) output(s, want string) {
	f.t.Helper()
	f.proc.Output(s)
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(f.sess.Terminal().Text(), want) {
		if time.Now().After(deadline) {
			f.t.Fatalf("terminal never showed %q:\n%s", want, f.sess.Terminal().Text())
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// mode feeds output that only sets a mode, and waits until on reports it
// (output has no text to wait for).
func (f *termFixture) mode(s string, on func(vterm.Modes) bool) {
	f.t.Helper()
	f.proc.Output(s)
	deadline := time.Now().Add(5 * time.Second)
	for !on(f.sess.Terminal().Modes()) {
		if time.Now().After(deadline) {
			f.t.Fatalf("mode %q never took effect", s)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func (f *termFixture) draw() {
	f.screen.Clear()
	f.view.Draw(f.screen)
	f.screen.Show()
}

// row returns screen row y between columns x0 and x1 as text.
func (f *termFixture) row(y, x0, x1 int) string {
	var b strings.Builder
	for x := x0; x < x1; x++ {
		str, _, _ := f.screen.Get(x, y)
		if str == "" {
			str = " "
		}
		b.WriteString(str)
	}
	return b.String()
}

func (f *termFixture) waitInput(want string) {
	f.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(string(f.proc.Input()), want) {
		if time.Now().After(deadline) {
			f.t.Fatalf("input %q lacks %q", f.proc.Input(), want)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestTerminalViewDrawAndResize(t *testing.T) {
	f := newTermFixture(t)
	f.draw()
	// Drawing resized the session to the inner rect, and told the program.
	if c, r := f.sess.Terminal().Size(); c != 40 || r != 10 {
		t.Fatalf("terminal size %dx%d, want 40x10", c, r)
	}
	if c, r := f.proc.Size(); c != 40 || r != 10 {
		t.Fatalf("pty size %dx%d, want 40x10", c, r)
	}

	f.output("\x1b[31mred\x1b[0m 日本 ok\r\n$ ", "ok")
	f.draw()
	if got := f.row(1, 1, 12); got != "red 日 本 ok" && !strings.HasPrefix(got, "red 日") {
		t.Fatalf("row 1 = %q", got)
	}
	fg, _, _ := f.cellStyle(1, 1).Decompose()
	if fg != tcell.PaletteColor(1) {
		t.Errorf("red text drawn with %v", fg)
	}
	// The wide character takes two cells; the next text starts after it.
	if str, _, w := f.screen.Get(5, 1); str != "日" || w != 2 {
		t.Errorf("wide char at col 5 = %q width %d", str, w)
	}
	if str, _, _ := f.screen.Get(7, 1); str != "本" {
		t.Errorf("second wide char at col 7 = %q", str)
	}
	// Cursor after the prompt.
	if cx, cy, visible := f.screen.GetCursor(); !visible || cx != 3 || cy != 2 {
		t.Errorf("cursor at %d,%d visible=%v, want 3,2", cx, cy, visible)
	}

	f.view.SetRect(0, 0, 32, 8)
	f.draw()
	if c, r := f.proc.Size(); c != 30 || r != 6 {
		t.Fatalf("pty size after pane resize %dx%d, want 30x6", c, r)
	}
}

func (f *termFixture) cellStyle(x, y int) tcell.Style {
	_, st, _ := f.screen.Get(x, y)
	return st
}

func TestTerminalViewKeys(t *testing.T) {
	f := newTermFixture(t)
	f.draw()
	f.mode("\x1b[?1h", func(m vterm.Modes) bool { return m.AppCursor }) // application cursor keys
	// output with "" returns at once: wait until the emulator has the mode,
	// or Up may be encoded before it (a flake under load).
	for deadline := time.Now().Add(5 * time.Second); !f.sess.Terminal().Modes().AppCursor; {
		if time.Now().After(deadline) {
			t.Fatal("application cursor mode never set")
		}
		time.Sleep(2 * time.Millisecond)
	}
	keys := []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, 'l', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl),
		tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyF5, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModAlt),
	}
	for _, k := range keys {
		f.view.HandleKey(k, nil, nil)
	}
	f.waitInput("ls\r\x03\x1bOA\x1b[15~\x1bx")
}

func TestTerminalViewScrollMode(t *testing.T) {
	f := newTermFixture(t)
	f.draw()
	var out strings.Builder
	for i := range 30 {
		fmt.Fprintf(&out, "line %02d\r\n", i)
	}
	f.output(out.String(), "line 29")
	f.draw()
	if got := strings.TrimSpace(f.row(1, 1, 41)); got != "line 21" {
		t.Fatalf("live top row = %q", got)
	}
	f.view.EnterScroll()
	f.view.HandleKey(tcell.NewEventKey(tcell.KeyPgUp, 0, tcell.ModNone), nil, nil)
	f.draw()
	if got := strings.TrimSpace(f.row(1, 1, 10)); got != "line 12" {
		t.Fatalf("after page up, top row = %q", got)
	}
	if !strings.Contains(f.row(1, 1, 41), "scroll 9/") {
		t.Errorf("no scroll indicator: %q", f.row(1, 1, 41))
	}
	// Keys do not reach the program in scroll mode.
	f.view.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone), nil, nil)
	f.draw()
	if got := strings.TrimSpace(f.row(1, 1, 10)); got != "line 00" {
		t.Fatalf("after g, top row = %q", got)
	}
	f.view.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone), nil, nil)
	if f.view.Scrolling() {
		t.Fatal("q did not leave scroll mode")
	}
	f.draw()
	if got := strings.TrimSpace(f.row(1, 1, 41)); got != "line 21" {
		t.Fatalf("after q, top row = %q", got)
	}
	if len(f.proc.Input()) != 0 {
		t.Errorf("scroll keys reached the program: %q", f.proc.Input())
	}
}

func TestTerminalViewPaste(t *testing.T) {
	f := newTermFixture(t)
	paste := f.view.PasteHandler()
	paste("echo one\n", nil)
	f.waitInput("echo one\r")
	f.mode("\x1b[?2004h", func(m vterm.Modes) bool { return m.BracketedPaste })
	paste("echo two\n", nil)
	f.waitInput("\x1b[200~echo two\r\x1b[201~")
}

func (f *termFixture) mouse(action tview.MouseAction, x, y int, buttons tcell.ButtonMask, mods tcell.ModMask) {
	f.view.MouseHandler()(action, tcell.NewEventMouse(x, y, buttons, mods), func(tview.Primitive) {})
}

func TestTerminalViewSelectionCopies(t *testing.T) {
	f := newTermFixture(t)
	var copied int
	f.view.onCopy = func(n int) { copied = n }
	f.draw()
	f.output("hello world\r\nsecond line\r\n", "second line")
	f.draw()
	// Drag from "world" on the first row to "second" on the next.
	f.mouse(tview.MouseLeftDown, 7, 1, tcell.Button1, 0)
	f.mouse(tview.MouseMove, 6, 2, tcell.Button1, 0)
	f.mouse(tview.MouseLeftUp, 6, 2, 0, 0)
	want := "world\nsecond"
	if got := f.view.SelectionText(); got != want {
		t.Fatalf("selection %q, want %q", got, want)
	}
	if got := string(f.screen.GetClipboardData()); got != want {
		t.Fatalf("clipboard %q", got)
	}
	if copied != len(want) {
		t.Errorf("onCopy got %d", copied)
	}
	f.draw()
	if _, _, attrs := f.cellStyle(7, 1).Decompose(); attrs&tcell.AttrReverse == 0 {
		t.Error("selection not highlighted")
	}
}

// A program that asked for mouse reports gets them; Shift selects instead.
func TestTerminalViewMouseForwarding(t *testing.T) {
	f := newTermFixture(t)
	f.draw()
	f.mode("\x1b[?1000h\x1b[?1006h", func(m vterm.Modes) bool { return m.MouseNormal && m.MouseSGR })
	f.mouse(tview.MouseLeftDown, 3, 2, tcell.Button1, 0)
	f.mouse(tview.MouseLeftUp, 3, 2, 0, 0)
	f.mouse(tview.MouseScrollUp, 3, 2, tcell.WheelUp, 0)
	f.waitInput("\x1b[<0;3;2M\x1b[<0;3;2m\x1b[<64;3;2M")

	before := len(f.proc.Input())
	f.mouse(tview.MouseLeftDown, 3, 2, tcell.Button1, tcell.ModShift)
	if len(f.proc.Input()) != before {
		t.Error("shift-click was forwarded")
	}
}

func TestTerminalViewBanner(t *testing.T) {
	f := newTermFixture(t)
	f.draw()
	f.proc.Exit(0)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if st, _, _ := f.sess.State(); st == session.Disconnected {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("session did not end")
		}
		time.Sleep(2 * time.Millisecond)
	}
	f.draw()
	if got := f.row(10, 1, 41); !strings.Contains(got, "Session closed (exit 0). r reconnect, c") {
		t.Fatalf("banner row %q", got)
	}
	var reconnected, closed bool
	f.view.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'x', 0), func() { reconnected = true }, func() { closed = true })
	if reconnected || closed || len(f.proc.Input()) != 0 {
		t.Fatal("x did something on a closed session")
	}
	f.view.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'r', 0), func() { reconnected = true }, func() { closed = true })
	f.view.HandleKey(tcell.NewEventKey(tcell.KeyRune, 'c', 0), func() { reconnected = true }, func() { closed = true })
	if !reconnected || !closed {
		t.Fatalf("r/c: reconnected=%v closed=%v", reconnected, closed)
	}
}

func TestParsePrefix(t *testing.T) {
	tests := []struct {
		spec  string
		ev    *tcell.EventKey
		match bool
		label string
	}{
		{"Ctrl-B", tcell.NewEventKey(tcell.KeyCtrlB, 'b', tcell.ModCtrl), true, "ctrl-b"},
		{"Ctrl-B", tcell.NewEventKey(tcell.KeyRune, 'b', tcell.ModNone), false, "ctrl-b"},
		{"Ctrl-a", tcell.NewEventKey(tcell.KeyCtrlA, 'a', tcell.ModCtrl), true, "ctrl-a"},
		{"Alt-X", tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModAlt), true, "alt-x"},
		{"Alt-X", tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone), false, "alt-x"},
		{"bogus", tcell.NewEventKey(tcell.KeyCtrlB, 'b', tcell.ModCtrl), true, "ctrl-b"},
	}
	for _, tt := range tests {
		p := parsePrefix(tt.spec)
		if p.match(tt.ev) != tt.match || p.label != tt.label {
			t.Errorf("parsePrefix(%q): match(%s)=%v label %q", tt.spec, tt.ev.Name(), p.match(tt.ev), p.label)
		}
		if !p.match(p.event()) {
			t.Errorf("parsePrefix(%q): its own event does not match", tt.spec)
		}
	}
}

// click sends what tview sends for one click: down, up, then click or,
// for clickAction MouseLeftDoubleClick, the double-click.
func (f *termFixture) click(x, y int, clickAction tview.MouseAction, mods tcell.ModMask) {
	f.mouse(tview.MouseLeftDown, x, y, tcell.Button1, mods)
	f.mouse(tview.MouseLeftUp, x, y, 0, mods)
	f.mouse(clickAction, x, y, 0, mods)
}

func TestTerminalViewDoubleClickSelectsWord(t *testing.T) {
	// The pane's inner area starts at (1, 1): column c is x = c+1.
	tests := []struct {
		name string
		x    int
		want string
	}{
		{"a word", 2, "ssh"},
		{"user@host, from its last letter", 19, "ali@web-01.prod"},
		{"a path, from the middle", 25, "/var/log/x.log"},
		{"a blank", 4, ""},
		{"a lone symbol", 36, ""},
		{"past the text", 40, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTermFixture(t)
			f.draw()
			f.output("ssh ali@web-01.prod /var/log/x.log | g\r\n", "| g")
			f.draw()
			f.click(tt.x, 1, tview.MouseLeftClick, 0)
			f.click(tt.x, 1, tview.MouseLeftDoubleClick, 0)
			if got := f.view.SelectionText(); got != tt.want {
				t.Fatalf("selection %q, want %q", got, tt.want)
			}
			if got := string(f.screen.GetClipboardData()); got != tt.want {
				t.Fatalf("clipboard %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTerminalViewTripleClickSelectsLine(t *testing.T) {
	f := newTermFixture(t)
	f.draw()
	f.output("first line here\r\nsecond line\r\n", "second line")
	f.draw()
	f.click(8, 1, tview.MouseLeftClick, 0)
	f.click(8, 1, tview.MouseLeftDoubleClick, 0)
	if got := f.view.SelectionText(); got != "line" {
		t.Fatalf("double-click selected %q", got)
	}
	f.click(8, 1, tview.MouseLeftClick, 0)
	if got := f.view.SelectionText(); got != "first line here" {
		t.Fatalf("triple-click selected %q", got)
	}
	if got := string(f.screen.GetClipboardData()); got != "first line here" {
		t.Fatalf("clipboard %q", got)
	}
	// A fourth click is a plain click again: the selection goes.
	f.click(8, 1, tview.MouseLeftClick, 0)
	if f.view.SelectionText() != "" {
		t.Fatal("a later click kept the selection")
	}
	// A click on another line right after a double-click is not a triple-click.
	f.click(3, 2, tview.MouseLeftClick, 0)
	f.click(3, 2, tview.MouseLeftDoubleClick, 0)
	f.click(3, 1, tview.MouseLeftClick, 0)
	if got := f.view.SelectionText(); got != "" {
		t.Fatalf("click on another line selected %q", got)
	}
}

// With a program that reads the mouse, Shift+double-click still selects.
func TestTerminalViewShiftDoubleClick(t *testing.T) {
	f := newTermFixture(t)
	f.draw()
	f.output("hello world\r\n", "hello world")
	f.mode("\x1b[?1000h\x1b[?1006h", func(m vterm.Modes) bool { return m.MouseNormal && m.MouseSGR })
	f.draw()
	f.click(9, 1, tview.MouseLeftClick, 0)
	f.click(9, 1, tview.MouseLeftDoubleClick, 0)
	if got := f.view.SelectionText(); got != "" {
		t.Fatalf("a forwarded double-click selected %q", got)
	}
	f.click(9, 1, tview.MouseLeftClick, tcell.ModShift)
	f.click(9, 1, tview.MouseLeftDoubleClick, tcell.ModShift)
	if got := f.view.SelectionText(); got != "world" {
		t.Fatalf("shift+double-click selected %q", got)
	}
}
