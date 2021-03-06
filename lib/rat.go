package rat

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/nsf/termbox-go"
)

var (
	events         chan termbox.Event
	done           chan bool
	eventListeners map[keyEvent]func()
	modes          map[string]Mode
	cfg            Configurer

	widgets WidgetStack
	pagers  PagerStack
	prompt  ConfirmPrompt
)

func Init() error {
	var err error

	if err = termbox.Init(); err != nil {
		return err
	}

	termbox.SetInputMode(termbox.InputAlt)
	termbox.SetOutputMode(termbox.Output256)

	events = make(chan termbox.Event)
	widgets = NewWidgetStack()
	pagers = NewPagerStack()
	done = make(chan bool)
	eventListeners = make(map[keyEvent]func())
	modes = make(map[string]Mode)
	cfg = NewConfigurer()

	widgets.Push(pagers)
	prompt = NewConfirmPrompt()

	AddEventListener("q", PopPager)
	AddEventListener("S-q", Quit)
	AddEventListener("1", func() { pagers.Show(1) })
	AddEventListener("2", func() { pagers.Show(2) })
	AddEventListener("3", func() { pagers.Show(3) })

	w, h := termbox.Size()
	layout(w, h)

	return nil
}

func LoadConfig(rd io.Reader) {
	cfg.Process(rd)
}

func Close() {
	termbox.Close()
}

func Quit() {
	close(done)
}

func Run() {
	go func() {
		for {
			events <- termbox.PollEvent()
		}
	}()

loop:
	for {
		render()

		select {
		case <-done:
			break loop
		case e := <-events:
			switch e.Type {
			case termbox.EventKey:
				ke := KeyEventFromTBEvent(&e)
				handleEvent(ke)
			case termbox.EventResize:
				layout(e.Width, e.Height)
			}
		case <-time.After(time.Second / 10):
		}
	}

	widgets.Destroy()
}

func AddChildPager(parent, child Pager, creatingKey string) {
	pagers.AddChild(parent, child, creatingKey)
}

func PushPager(p Pager) {
	pagers.Push(p)
}

func PopPager() {
	pagers.Pop()

	if pagers.Size() == 0 {
		Quit()
	}
}

func Confirm(message string, callback func()) {
	prompt.Confirm(message, callback)
}

func ConfirmExec(cmd string, ctx Context, callback func()) {
	Confirm(fmt.Sprintf("Run `%s`", InterpolateContext(cmd, ctx)), func() {
		Exec(cmd, ctx)
		callback()
	})
}

func Exec(cmd string, ctx Context) {
	exec.Command(os.Getenv("SHELL"), "-c", InterpolateContext(cmd, ctx)).Run()
}

func RegisterMode(name string, mode Mode) {
	modes[name] = mode
}

func layout(width, height int) {
	widgets.SetBox(NewBox(0, 0, width, height-1))
	prompt.SetBox(NewBox(0, height-1, width, 1))
}

func render() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	widgets.Render()
	prompt.Render()
	termbox.Flush()
}

func AddEventListener(keyStr string, handler func()) {
	eventListeners[KeyEventFromString(keyStr)] = handler
}

func handleEvent(ke keyEvent) bool {
	if prompt.HandleEvent(ke) {
		return true
	}

	if widgets.HandleEvent(ke) {
		return true
	}

	if handler, ok := eventListeners[ke]; ok {
		handler()
		return true
	}

	return false
}
