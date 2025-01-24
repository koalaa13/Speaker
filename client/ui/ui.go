package ui

import (
	"github.com/gotk3/gotk3/gtk"
)

func CreateWindow(muteButtonCallback func()) *gtk.Window {
	gtk.Init(nil)
	win, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	win.SetTitle("SilverSpeak")
	win.Connect("destroy", func() {
		gtk.MainQuit()
	})
	win.SetDefaultSize(500, 500)

	muteButton, _ := gtk.ButtonNew()
	muteButton.Connect("clicked", muteButtonCallback)
	buttonLabel, _ := gtk.LabelNew("Mute/Unmute")
	muteButton.Add(buttonLabel)
	win.Add(muteButton)

	win.ShowAll()
	gtk.Main()
	return win
}
