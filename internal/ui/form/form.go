package form

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/TE0dollary/chatlog-bot/internal/ui/style"
)

const (
	// DialogPadding dialog inner padding.
	DialogPadding = 3

	// DialogHelpHeight dialog help text height.
	DialogHelpHeight = 1

	// DialogMinWidth dialog min width.
	DialogMinWidth = 40

	// FormHeightOffset form height offset for border.
	FormHeightOffset = 3

	// extra width padding, similar to submenu's cmdWidthOffset
	formWidthOffset = 10
)

// Form is a modal form component with a title, form fields, and help text.
type Form struct {
	*tview.Box
	title         string
	layout        *tview.Flex
	form          *tview.Form
	helpText      *tview.TextView
	width         int
	height        int
	cancelHandler func()
	fields        []formField // stored for width recalculation
}

type formField struct {
	label      string
	value      string
	fieldWidth int
}

// NewForm creates a new form with the given title.
func NewForm(title string) *Form {
	f := &Form{
		Box:    tview.NewBox(),
		title:  title,
		layout: tview.NewFlex().SetDirection(tview.FlexRow),
		form:   tview.NewForm(),
		fields: make([]formField, 0),
	}

	f.form.SetBorderPadding(1, 1, 1, 1)
	f.form.SetBackgroundColor(style.DialogBgColor)
	f.form.SetFieldBackgroundColor(style.BgColor)
	f.form.SetFieldTextColor(style.FgColor)
	f.form.SetButtonBackgroundColor(style.ButtonBgColor)
	f.form.SetButtonTextColor(style.FgColor)
	f.form.SetLabelColor(style.DialogFgColor)
	f.form.SetButtonsAlign(tview.AlignCenter)

	f.helpText = tview.NewTextView()
	f.helpText.SetDynamicColors(true)
	f.helpText.SetTextAlign(tview.AlignCenter)
	f.helpText.SetTextColor(style.DialogFgColor)
	f.helpText.SetBackgroundColor(style.DialogBgColor)
	fmt.Fprintf(f.helpText,
		"[%s::b]Tab[%s::b]: 导航  [%s::b]Enter[%s::b]: 选择  [%s::b]ESC[%s::b]: 返回",
		style.GetColorHex(style.MenuBgColor), style.GetColorHex(style.PageHeaderFgColor),
		style.GetColorHex(style.MenuBgColor), style.GetColorHex(style.PageHeaderFgColor),
		style.GetColorHex(style.MenuBgColor), style.GetColorHex(style.PageHeaderFgColor),
	)

	formLayout := tview.NewFlex().SetDirection(tview.FlexColumn)
	formLayout.AddItem(EmptyBoxSpace(style.DialogBgColor), 1, 0, false)
	formLayout.AddItem(f.form, 0, 1, true)
	formLayout.AddItem(EmptyBoxSpace(style.DialogBgColor), 1, 0, false)

	f.layout.SetTitle(fmt.Sprintf("[::b]%s", f.title))
	f.layout.SetTitleColor(style.DialogFgColor)
	f.layout.SetTitleAlign(tview.AlignCenter)
	f.layout.SetBorder(true)
	f.layout.SetBorderColor(style.DialogBorderColor)
	f.layout.SetBackgroundColor(style.DialogBgColor)

	f.layout.AddItem(formLayout, 0, 1, true)
	f.layout.AddItem(f.helpText, DialogHelpHeight, 0, false)

	return f
}

// AddInputField adds an input field to the form.
func (f *Form) AddInputField(label, value string, fieldWidth int, accept func(textToCheck string, lastChar rune) bool, changed func(text string)) *Form {
	f.fields = append(f.fields, formField{
		label:      label,
		value:      value,
		fieldWidth: fieldWidth,
	})

	f.form.AddInputField(label, value, fieldWidth, accept, changed)
	f.recalculateSize()

	return f
}

// AddButton adds a button to the form.
func (f *Form) AddButton(label string, selected func()) *Form {
	f.form.AddButton(label, selected)
	f.recalculateSize()
	return f
}

// AddCheckbox adds a checkbox to the form.
func (f *Form) AddCheckbox(label string, checked bool, changed func(checked bool)) *Form {
	f.form.AddCheckbox(label, checked, changed)
	f.recalculateSize()
	return f
}

// SetCancelFunc sets the function to be called when the form is cancelled.
func (f *Form) SetCancelFunc(handler func()) *Form {
	f.cancelHandler = handler
	return f
}

// recalculateSize recomputes form dimensions based on current fields and buttons.
func (f *Form) recalculateSize() {
	itemCount := f.form.GetFormItemCount()

	// each field takes 2 rows; add button row, border, and help text
	f.height = (itemCount * 2) + 2 + FormHeightOffset + DialogHelpHeight

	maxLabelWidth := 0
	maxValueWidth := 0

	for _, field := range f.fields {
		if len(field.label) > maxLabelWidth {
			maxLabelWidth = len(field.label)
		}

		valueWidth := field.fieldWidth
		if len(field.value) > valueWidth {
			valueWidth = len(field.value)
		}

		if valueWidth > maxValueWidth {
			maxValueWidth = valueWidth
		}
	}

	f.width = maxLabelWidth + maxValueWidth + formWidthOffset

	if f.width < DialogMinWidth {
		f.width = DialogMinWidth
	}
}

// Draw draws the form on the screen.
func (f *Form) Draw(screen tcell.Screen) {
	f.recalculateSize()
	f.Box.DrawForSubclass(screen, f)
	f.layout.Draw(screen)
}

// SetRect sets the position and size of the form.
func (f *Form) SetRect(x, y, width, height int) {
	f.recalculateSize()

	ws := (width - f.width) / 2
	hs := (height - f.height) / 2

	if f.width > width {
		ws = 0
		f.width = width - 1
	}

	if f.height > height {
		hs = 0
		f.height = height - 1
	}

	f.Box.SetRect(x+ws, y+hs, f.width, f.height)

	x, y, width, height = f.Box.GetInnerRect()
	f.layout.SetRect(x, y, width, height)
}

// Focus is called when this primitive receives focus.
func (f *Form) Focus(delegate func(p tview.Primitive)) {
	if f.form != nil {
		delegate(f.form)
	} else {
		delegate(f.Box)
	}
}

// HasFocus returns whether or not this primitive has focus.
func (f *Form) HasFocus() bool {
	return f.form.HasFocus()
}

// InputHandler returns the handler for this primitive.
func (f *Form) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return f.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if event.Key() == tcell.KeyEscape && f.cancelHandler != nil {
			f.cancelHandler()
			return
		}

		if handler := f.form.InputHandler(); handler != nil {
			handler(event, setFocus)
		}
	})
}

// EmptyBoxSpace creates an empty box with the specified background color.
func EmptyBoxSpace(bgColor tcell.Color) *tview.Box {
	box := tview.NewBox()
	box.SetBackgroundColor(bgColor)
	box.SetBorder(false)
	return box
}
