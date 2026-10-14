package main

import (
	"os"

	"golang.org/x/sys/windows"
)

func enableVirtualTerminal() bool {
	console := windows.Handle(os.Stdout.Fd())

	var consoleMode uint32
	if err := windows.GetConsoleMode(console, &consoleMode); err != nil {
		return false
	}

	return windows.SetConsoleMode(console, consoleMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
