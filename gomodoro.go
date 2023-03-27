package main

import (
	"fmt"
	"os"
	"path"
	"time"
	"encoding/json"
	_ "embed"

	"github.com/getlantern/systray"
	"github.com/godbus/dbus"
	"github.com/veandco/go-sdl2/sdl"
	"github.com/veandco/go-sdl2/img"
	"github.com/veandco/go-sdl2/ttf"
	"github.com/badkaktus/gorocket"
	"github.com/adrg/xdg"
)

type State int

const (
	STOPPED State = iota
	WORKING
	PAUSED
)

type Configuration struct {
	User	string
	Token   string
}

var state State
var tick *time.Timer
var workdur time.Duration
var pausedur time.Duration
var tickdur time.Duration
var announcedur time.Duration
var time_left time.Duration
var windows []*sdl.Window
var rocket_client *gorocket.Client
var conf Configuration

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

	config_dir := path.Join(xdg.ConfigHome, "gomodoro")
	config_path := path.Join(config_dir, "config.json")

	err := os.MkdirAll(config_dir, 0700)
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(config_path); err != nil {
		config_message := fmt.Sprintf("No Rocketchat credentials in \n%s\n", config_path)
		notify(config_message, "")
		fmt.Printf(config_message)
	} else {
		file, _ := os.Open(config_path)
		defer file.Close()
		decoder := json.NewDecoder(file)
		conf := Configuration{}
		err := decoder.Decode(&conf)
		if err != nil {
			msg := fmt.Sprintf("Error %s, can't decode config at\n%s: %s\n", err, config_path)
			fmt.Println(msg)
			notify(msg, "")
		} else {
			rocket_client = gorocket.NewWithOptions("https://chat.avm.de",
				gorocket.WithUserID(conf.User),
				gorocket.WithXToken(conf.Token),
				gorocket.WithTimeout(1 * time.Second),
			)
		}
	}

	systray.Run(onReady, onExit)
}

func onReady() {
	workdur = 50 * time.Minute
	pausedur = 10 * time.Minute
	announcedur = 5 * time.Minute
	tickdur = time.Second
	state = STOPPED

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

	mPause := systray.AddMenuItem("Start Pause", "Pause Pomodoro Session")
	go func() {
		for {
			select {
			case <-mPause.ClickedCh:
				startPause()
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
	unpauseScreen()
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

func set_chat_status(status string) {
	if rocket_client != nil {
		new_status := gorocket.SetStatus{Message: "", Status: status}
		rocket_client.UsersSetStatus(&new_status)
	}
}

func startSession() {
	state = WORKING
	set_chat_status("busy")
	clearTickTimer()
	time_left = workdur
	tick = time.AfterFunc(tickdur, ticked)
	unpauseScreen()
	updateIcon(time_left_fmt())
	playMusic()
	notify("Work Started", "")
}

func stopSession() {
	state = STOPPED
	set_chat_status("online")
	time_left = 0
	clearTickTimer()
	updateIcon("off")
	unpauseScreen()
	notify("Session Stopped", "")
}

func startPause() {
	state = PAUSED
	set_chat_status("online")
	clearTickTimer()
	time_left = pausedur
	tick = time.AfterFunc(tickdur, ticked)
	pauseScreen()
	updateIcon(time_left_fmt())
	pauseMusic()
	notify("Pause Started", "")
}

func clearTickTimer() {
	clearTimer(tick)
	tick = nil
}

func clearTimer(timer *time.Timer) {
	if timer != nil && !timer.Stop() {
		<-timer.C
	}
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
	if time_left == 0 {
		if state == PAUSED {
			startSession()
		} else if state == WORKING {
			startPause()
		}
	}
	if time_left == announcedur {
		if state == PAUSED {
			announceSession()
		} else if state == WORKING {
			announcePause()
		}
	}
}

func isWork() bool {
	return state == WORKING
}

func isPause() bool {
	return state == PAUSED
}

func isRunning() bool {
	return isPause() || isWork()
}

func isStopped() bool {
	return state == STOPPED
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

