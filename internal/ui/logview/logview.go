package logview

import (
	"sync"

	"github.com/TE0dollary/chatlog-bot/internal/ui/style"

	"github.com/rivo/tview"
)

const (
	Title    = " Log "
	MaxBytes = 64 * 1024 // 64 KB; clear on overflow
)

// LogView is a scrollable log panel implementing io.Writer.
// Logs are appended in real time and automatically scrolled to the end.
type LogView struct {
	*tview.TextView

	mu     sync.Mutex
	redraw func() // injected by App; triggers redraw and scroll-to-end
}

// New creates and returns a LogView.
func New() *LogView {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetBorder(true)
	tv.SetBorderColor(style.BorderColor)
	tv.SetTitle(Title)
	tv.SetTitleAlign(tview.AlignLeft)
	tv.SetWrap(true)

	return &LogView{TextView: tv}
}

// SetRedrawFunc injects the redraw callback (e.g. app.QueueUpdateDraw + ScrollToEnd).
// Call this after App starts to ensure thread-safe UI refresh.
func (lv *LogView) SetRedrawFunc(f func()) {
	lv.mu.Lock()
	defer lv.mu.Unlock()
	lv.redraw = f
}

// Write implements io.Writer, appending to the text view and scrolling to end.
// Safe to call from any goroutine.
func (lv *LogView) Write(p []byte) (n int, err error) {
	if len(lv.TextView.GetText(false))+len(p) > MaxBytes {
		lv.TextView.Clear()
	}
	n, err = lv.TextView.Write(p)

	lv.mu.Lock()
	redraw := lv.redraw
	lv.mu.Unlock()

	if redraw != nil {
		redraw()
	}
	return n, err
}
