//go:build windows

package main

import (
	termbox "github.com/nsf/termbox-go"
	log "github.com/sirupsen/logrus"
)

// pollKeyboard reads key events; termbox does not support raw input on Windows
func pollKeyboard() {
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch {
			case ev.Key == termbox.KeyCtrlR || ev.Ch == 'r':
				handleReloadKey()
			case ev.Key == termbox.KeyEsc || ev.Key == termbox.KeyCtrlC || ev.Ch == 'q':
				handleExitKey()
			}
		case termbox.EventError:
			panic(ev.Err)
		default:
			log.Tracef("unhandled default case %v", ev)
		}
	}
}
