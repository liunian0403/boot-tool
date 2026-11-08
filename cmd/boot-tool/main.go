package main

import (
	"github.com/liunian0403/boot-tool/internal/app"
	"github.com/liunian0403/boot-tool/internal/clock"
	"github.com/liunian0403/boot-tool/internal/gui"
	"github.com/liunian0403/boot-tool/internal/locale"
)

func main() {
	gui.Run(app.New(clock.New(), locale.Detect()))
}
