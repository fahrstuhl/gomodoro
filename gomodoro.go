package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getlantern/systray"
	"github.com/godbus/dbus"
	"github.com/veandco/go-sdl2/sdl"
)

var work *time.Timer
var pause *time.Timer
var workdur time.Duration
var pausedur time.Duration
var windows []*sdl.Window

func main() {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		panic(err)
	}
	defer sdl.Quit()
	systray.Run(onReady, onExit)
}

func onReady() {
	workdur = 50 * time.Minute
	pausedur = 10 * time.Second

	systray.SetTitle("Gomodoro")
	systray.SetTooltip("Gomodoro Timer")

	mStart := systray.AddMenuItem("Start Session", "Start Pomodoro Session")
	go func() {
		for {
			select {
			case <-mStart.ClickedCh:
				startSession()
			}
		}
	}()

	mStop := systray.AddMenuItem("Stop Session", "Stop Pomodoro Session")
	go func() {
		for {
			select {
			case <-mStop.ClickedCh:
				stopSession()
			}
		}
	}()

	mQuit := systray.AddMenuItem("Quit", "Quit Gomodoro")
	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()
}

func onExit() {
}

func pauseScreen() {
	num_screens, _ := sdl.GetNumVideoDisplays()
	notify("found "+fmt.Sprint(num_screens)+"screens", "")
	for idx := 0; idx < num_screens; idx++ {
		bounds, _ := sdl.GetDisplayBounds(idx)
		mode, _ := sdl.GetDesktopDisplayMode(idx)
		fmt.Printf("Creating window on screen %d with bounds %dx%d and size %dx%d\n", idx, bounds.X, bounds.Y, mode.W, mode.H)
		window, _ := sdl.CreateWindow("Gomodoro", bounds.X, bounds.Y, mode.W, mode.H, sdl.WINDOW_BORDERLESS)
		window.SetPosition(bounds.X, bounds.Y)
		window.Show()
		windows = append(windows, window)
	}
	for idx := 0; idx < num_screens; idx++ {
		window := windows[idx]
		surface, _ := window.GetSurface()
		surface.FillRect(nil, 0)
		window.UpdateSurface()
	}
}

func unpauseScreen() {
	for _, win := range windows {
		win.Destroy()
	}
	windows = nil
}

func startSession() {
	work = time.AfterFunc(workdur, startPause)
	notify("Work Started", "")
	playMusic()
	unpauseScreen()
}

func stopSession() {
	notify("Session Stopped", "")
	if work != nil && !work.Stop() {
		<-work.C
	}
	if pause != nil && !pause.Stop() {
		<-pause.C
	}
	unpauseScreen()
}

func startPause() {
	notify("Pause Started", "")
	pause = time.AfterFunc(pausedur, startSession)
	pauseMusic()
	pauseScreen()
}

func pauseMusic() {
	controlMusic("Pause")
}

func playMusic() {
	controlMusic("Play")
}

func controlMusic(control string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}
	call := conn.Object("org.mpris.MediaPlayer2.spotify", "/org/mpris/MediaPlayer2").Call("org.mpris.MediaPlayer2.Player."+control, 0)
	if call.Err != nil {
		fmt.Fprintln(os.Stderr, "Failed to control music: ", call.Err)
	}
}

func notify(title string, message string) {
	conn, err := dbus.SessionBus()
	if err != nil {
		panic(err)
	}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0),
		"", title, message, []string{},
		map[string]dbus.Variant{}, int32(5000))
	if call.Err != nil {
		panic(call.Err)
	}
}
