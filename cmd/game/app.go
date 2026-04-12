package main

import (
	"game-engine/internal/agent"
	"game-engine/internal/commands"
	"game-engine/internal/debug"
	"game-engine/internal/engineconfig"
	"game-engine/internal/llm"
	"game-engine/internal/logger"
	"game-engine/internal/scene"
	"game-engine/internal/terminal"
	"game-engine/internal/ui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// App holds all engine subsystems and shared state wired together in main().
type App struct {
	Log       *logger.Logger
	Scene     *scene.Scene
	Debug     *debug.Debug
	Registry  *commands.Registry
	Terminal  *terminal.Terminal
	UI        *ui.Engine
	Inspector *ui.Inspector
	Agent     *agent.Agent
	Client    llm.Client

	// Config state
	CurrentAIModel string
	CurrentFont    string
	IsOllama       bool

	// Async result channels
	DownloadDone     chan *downloadResult
	SkyboxDone       chan *skyboxResult
	FontDownloadDone chan *fontDownloadResult
	PendingRunCmd    chan []string

	// Internal draw state
	baseNodes      []*ui.Node
	uiFontTried    bool
	engineFontPaths []string
}

type downloadResult struct {
	Index int
	Path  string
	Err   error
}

type skyboxResult struct {
	Path string
	Err  error
}

type fontDownloadResult struct {
	RelPath  string
	FullPath string
	Err      error
}

func (app *App) SaveEnginePrefs() {
	_ = engineconfig.Save(engineconfig.EnginePrefs{
		ShowFPS:      app.Debug.ShowFPS,
		ShowMemAlloc: app.Debug.ShowMemAlloc,
		GridVisible:  app.Scene.GridVisible,
		AIModel:      app.CurrentAIModel,
		Font:         app.CurrentFont,
	})
}

func (app *App) Update() {
	drainChan(app.PendingRunCmd, func(args []string) {
		if err := app.Registry.Execute(args); err != nil {
			app.Log.Log(err.Error())
		}
	})

	drainChan(app.DownloadDone, func(res *downloadResult) {
		if res.Err != nil {
			app.Log.Log(res.Err.Error())
		} else if err := app.Scene.SetObjectTexture(res.Index, res.Path); err != nil {
			app.Log.Log(err.Error())
		} else {
			app.Log.Log("Texture applied: " + res.Path)
		}
	})

	drainChan(app.SkyboxDone, func(res *skyboxResult) {
		if res.Err != nil {
			app.Log.Log(res.Err.Error())
		} else {
			app.Scene.SetSkyboxPath(res.Path)
			app.Log.Log("Skybox set: " + res.Path)
		}
	})

	drainChan(app.FontDownloadDone, func(res *fontDownloadResult) {
		if res.Err != nil {
			app.Log.Log(res.Err.Error())
		} else if err := app.UI.LoadFont(res.FullPath); err != nil {
			app.Log.Log(err.Error())
		} else {
			app.CurrentFont = res.RelPath
			app.Terminal.SetFont(app.UI.Font())
			app.Debug.SetFont(app.UI.Font())
			app.SaveEnginePrefs()
			app.Log.Log("Font set: " + res.RelPath)
		}
	})

	app.Terminal.Update()

	if app.Terminal.IsOpen() {
		app.Scene.UpdateEditor(true, terminal.BarHeight)
		if obj, ok := app.Scene.SelectedObject(); ok {
			if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				screenH := int32(rl.GetScreenHeight())
				mouseY := rl.GetMouseY()
				if mouseY < screenH-int32(terminal.BarHeight) {
					hitNode, hit := app.UI.HitTest(rl.GetMouseX(), mouseY)
					if hit && hitNode != nil && hitNode.Class == "inspector-physics" {
						_ = app.Scene.SetSelectedPhysics(!scene.PhysicsEnabledForObject(obj))
					}
				}
			}
		}
	} else {
		app.Scene.Update()
	}
}

func (app *App) Draw() {
	app.Scene.Draw(app.Terminal.IsOpen())
	app.Debug.Draw()

	obj, ok := app.Scene.SelectedObject()
	nodes := app.Inspector.AppendNodes(app.baseNodes, app.Terminal.IsOpen() && ok, ui.Selection{
		Name:     obj.Type,
		Position: obj.Position,
		Scale:    obj.Scale,
		Physics:  scene.PhysicsEnabledForObject(obj),
		Texture:  obj.Texture,
	})

	if !app.uiFontTried {
		app.uiFontTried = true
		for _, p := range app.engineFontPaths {
			if err := app.UI.LoadFont(p); err == nil {
				app.Terminal.SetFont(app.UI.Font())
				app.Debug.SetFont(app.UI.Font())
				break
			}
		}
	}

	app.UI.SetNodes(nodes)
	app.UI.Draw()
	app.Terminal.Draw()
}

// drainChan reads all pending values from a channel and calls fn for each.
func drainChan[T any](ch chan T, fn func(T)) {
	for {
		select {
		case v := <-ch:
			fn(v)
		default:
			return
		}
	}
}
