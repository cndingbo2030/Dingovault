//go:build bindings

package main

import (
	"embed"

	"github.com/cndingbo2030/dingovault/internal/bridge"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// Minimal entry for `wails build` binding generation (-tags bindings). Does not open the DB or scan files.
func main() {
	app := bridge.NewApp(nil, nil, ".")
	_ = wails.Run(&options.App{
		Title:  "Dingovault",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 18, G: 18, B: 22, A: 1},
		Bind: []interface{}{
			app,
		},
	})
}
