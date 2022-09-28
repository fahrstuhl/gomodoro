package main

import (
	"fmt"
	"os"
	"time"
	_ "embed"

	"github.com/getlantern/systray"
	"github.com/godbus/dbus"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/ttf"
)

var work *time.Timer
var pause *time.Timer
var tick *time.Timer
var announce *time.Timer
var workdur time.Duration
var pausedur time.Duration
var tickdur time.Duration
var announcedur time.Duration
var time_left time.Duration
var windows []*sdl.Window

//go:embed "Kenney High Square.ttf"
var font []byte

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
	announcedur = 2 * time.Minute
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
		stopSession()
		systray.Quit()
	}()
}

func onExit() {
}

func announcePause() {
	notify(fmt.Sprintf("Session ends in %02.0f minutes.", time_left.Minutes()), "")
}

func announceSession() {
	notify(fmt.Sprintf("Session starts in %02.0f minutes.", time_left.Minutes()), "")
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
	clearAllTimers()
	time_left = workdur
	work = time.AfterFunc(workdur, startPause)
	tick = time.AfterFunc(tickdur, ticked)
	announce = time.AfterFunc(workdur - announcedur, announcePause)
	notify("Work Started", "")
	playMusic()
	unpauseScreen()
	updateIcon(time_left_fmt())
}

func stopSession() {
	time_left = 0
	clearAllTimers()
	notify("Session Stopped", "")
	updateIcon("off")
	unpauseScreen()
}

func clearTimer(timer *time.Timer) {
	if timer != nil && !timer.Stop() {
		<-timer.C
	}
}

func clearAllTimers() {
	clearTimer(work)
	work = nil
	clearTimer(pause)
	pause = nil
	clearTimer(tick)
	tick = nil
	clearTimer(announce)
	announce = nil
}

func startPause() {
	work = nil
	time_left = pausedur
	notify("Pause Started", "")
	pause = time.AfterFunc(pausedur, startSession)
	announce = time.AfterFunc(pausedur - announcedur, announceSession)
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

func isWork() bool {
	return work != nil
}

func isPause() bool {
	return work != nil
}

func isRunning() bool {
	return isPause() || isWork()
}

func isStopped() bool {
	return !isRunning()
}

func updateIcon(status string) {
	var color sdl.Color
	if isWork() {
		color.R = 0
		color.G = 255
		color.B = 0
		color.A = 255
	} else if isPause() {
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
	font_rwops, err := sdl.RWFromMem(font)
	font, _ := ttf.OpenFontRW(font_rwops, 0, 22)
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

