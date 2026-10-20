//go:build !windows

package main

import (
	"fmt"

	termbox "github.com/nsf/termbox-go"
	log "github.com/sirupsen/logrus"
)

// pollKeyboard reads raw terminal input; normal termbox events don't include escape sequences
func pollKeyboard() {
	for {
		var data [64]byte

		switch ev := termbox.PollRawEvent(data[:]); ev.Type {
		case termbox.EventRaw:
			switch fmt.Sprintf("%q", data[:ev.N]) {
			case `"\x12"`, `"r"`: // Ctrl+R, r
				handleReloadKey()
			case `"\x1b"`, `"q"`, `"\x03"`: // ESC, q, Ctrl+C
				handleExitKey()
			}
		case termbox.EventError:
			panic(ev.Err)
		default:
			log.Tracef("unhandled default case %v", ev)
		}
	}
}
