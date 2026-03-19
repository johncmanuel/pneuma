package main

import (
	"embed"

	"pneuma/internal/desktop"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := desktop.NewApp()

	err := wails.Run(&options.App{
		Title:       "pneuma",
		Width:       1280,
		Height:      800,
		MinWidth:    960,
		MinHeight:   600,
		Frameless:   false,
		StartHidden: false,
		//
		// HideWindowOnClose: true,
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 15, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		// prevent multiple instances of the app from running at the same time
		// https://wails.io/docs/guides/single-instance-lock/
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "c0d0b46b-2b5c-43bb-9f1c-c4b0b5c3eaff",
			OnSecondInstanceLaunch: app.SecondInstanceLaunch,
		},
		Bind: []interface{}{
			app,
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     false,
			DisableWebViewDrop: true,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
