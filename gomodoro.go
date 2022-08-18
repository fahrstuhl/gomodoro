package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getlantern/systray"
	"github.com/godbus/dbus"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/ttf"
)

var work *time.Timer
var pause *time.Timer
var tick *time.Timer
var workdur time.Duration
var pausedur time.Duration
var tickdur time.Duration
var time_left time.Duration
var windows []*sdl.Window

func main() {
	if err := sdl.Init(sdl.INIT_EVERYTHING); err != nil {
		panic(err)
	}
	defer sdl.Quit()
	if err := ttf.Init(); err != nil {
		panic(err)
	}
	defer ttf.Quit()
	updateIcon("off")
	systray.Run(onReady, onExit)
}

func onReady() {
	workdur = 50 * time.Minute
	pausedur = 10 * time.Minute
	tickdur = time.Minute

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
	pause = nil
	time_left = workdur
	work = time.AfterFunc(workdur, startPause)
	tick = time.AfterFunc(tickdur, ticked)
	notify("Work Started", "")
	playMusic()
	unpauseScreen()
	updateIcon(time_left_fmt())
}

func stopSession() {
	time_left = 0
	notify("Session Stopped", "")
	if work != nil && !work.Stop() {
		<-work.C
	}
	if pause != nil && !pause.Stop() {
		<-pause.C
	}
	if tick != nil && !tick.Stop() {
		<-tick.C
	}
	work = nil
	pause = nil
	tick = nil
	unpauseScreen()
}

func startPause() {
	work = nil
	time_left = pausedur
	notify("Pause Started", "")
	pause = time.AfterFunc(pausedur, startSession)
	pauseMusic()
	pauseScreen()
	updateIcon(time_left_fmt())
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

func time_left_fmt() string {
	status := fmt.Sprintf("%02.0f", time_left.Minutes())
	return status
}

func ticked() {
	time_left -= tickdur
	tick.Reset(tickdur)
	updateIcon(time_left_fmt())
}

func updateIcon(status string) {
	var color sdl.Color
	if work != nil {
		color.R = 0
		color.G = 255
		color.B = 0
		color.A = 255
	} else if pause != nil {
		color.R = 255
		color.G = 0
		color.B = 0
		color.A = 255
	} else {
		color.R = 128
		color.G = 128
		color.B = 128
		color.A = 255
	}
	font, _ := ttf.OpenFont("./Kenney High Square.ttf", 22)
	text, _ := font.RenderUTF8Blended(status, color)

	square, _ := sdl.CreateRGBSurface(0, 22, 22, 32, 0xFF000000, 0x00FF0000, 0x0000FF00, 0x000000FF)
	text.BlitScaled(&text.ClipRect, square, &square.ClipRect)

	dynamic_icon := make([]byte, 16384)
	rwops, err := sdl.RWFromMem(dynamic_icon)
	if err != nil {
		fmt.Printf("error while creating rwops: %s\n", err)
	}
	err = img.SavePNGRW(square, rwops, 0)
	if err != nil {
		fmt.Printf("error while writing: %s\n", err)
	}
	systray.SetIcon(dynamic_icon)
}

