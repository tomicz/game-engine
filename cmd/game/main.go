package main

import (
	"context"
	"game-engine/internal/agent"
	"game-engine/internal/commands"
	"game-engine/internal/debug"
	"game-engine/internal/engineconfig"
	"game-engine/internal/env"
	"game-engine/internal/graphics"
	"game-engine/internal/llm"
	"game-engine/internal/logger"
	"game-engine/internal/scene"
	"game-engine/internal/terminal"
	"game-engine/internal/ui"
	"net/http"
	_ "net/http/pprof"
	"os"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	_ = env.Load(".env")
	_ = env.Load("../../.env")

	if os.Getenv("DEBUG_PPROF") == "1" {
		go func() { _ = http.ListenAndServe("localhost:6060", nil) }()
	}

	log := logger.New()
	rl.SetTraceLogCallback(log.LogEngine)

	scn := scene.New()
	dbg := debug.New()
	reg := commands.NewRegistry()

	if os.Getenv("CAMERA_AWARENESS") == "1" {
		scn.EnableViewAwareness(scene.NewViewAwarenessWithLogging())
	}

	// Apply persisted engine prefs.
	prefs, _ := engineconfig.Load()
	dbg.SetShowFPS(prefs.ShowFPS)
	dbg.SetShowMemAlloc(prefs.ShowMemAlloc)
	scn.SetGridVisible(prefs.GridVisible)

	currentAIModel := prefs.AIModel
	if currentAIModel == "" {
		currentAIModel = "gpt-4o-mini"
	}
	currentFont := prefs.Font
	if currentFont == "" {
		currentFont = "Roboto/static/Roboto-Regular.ttf"
	}

	app := &App{
		Log:              log,
		Scene:            scn,
		Debug:            dbg,
		Registry:         reg,
		UI:               ui.New(),
		Inspector:        ui.NewInspector(),
		CurrentAIModel:   currentAIModel,
		CurrentFont:      currentFont,
		DownloadDone:     make(chan *downloadResult, 8),
		SkyboxDone:       make(chan *skyboxResult, 4),
		FontDownloadDone: make(chan *fontDownloadResult, 2),
		PendingRunCmd:    make(chan []string, 64),
		baseNodes:        []*ui.Node{},
	}

	// If only Groq is configured, default to a Groq model.
	if os.Getenv("GROQ_API_KEY") != "" && (app.CurrentAIModel == "" || app.CurrentAIModel == "gpt-4o-mini") {
		app.CurrentAIModel = "llama-3.3-70b-versatile"
		app.SaveEnginePrefs()
	}

	registerCommands(app)

	// LLM client: Groq (free) > Cursor (+ OpenAI fallback) > OpenAI > Ollama (local).
	groqKey := os.Getenv("GROQ_API_KEY")
	cursorKey := os.Getenv("CURSOR_API_KEY")
	openAIKey := os.Getenv("OPENAI_API_KEY")
	ollamaBase := os.Getenv("OLLAMA_BASE_URL")
	switch {
	case groqKey != "":
		app.Client = llm.NewGroq(groqKey)
	case cursorKey != "" && openAIKey != "":
		app.Client = &llm.Fallback{Primary: llm.NewCursor(cursorKey), Secondary: llm.NewOpenAI(openAIKey)}
	case cursorKey != "":
		app.Client = llm.NewCursor(cursorKey)
	case openAIKey != "":
		app.Client = llm.NewOpenAI(openAIKey)
	default:
		app.Client = llm.NewOllama(ollamaBase)
		app.IsOllama = true
		if app.CurrentAIModel == "" || app.CurrentAIModel == "gpt-4o-mini" || app.CurrentAIModel == "llama-3.3-70b-versatile" {
			app.CurrentAIModel = "qwen3-coder:30b"
			app.SaveEnginePrefs()
		}
	}

	if app.Client != nil {
		app.Agent = agent.New(app.Client, func() string { return app.CurrentAIModel })
		agent.RegisterSceneHandlers(app.Agent, scn, reg, app.PendingRunCmd)
	}

	app.Terminal = terminal.New(log, reg)
	if app.Agent != nil {
		app.Terminal.GetViewContext = func() string { return scn.GetViewContextSummary() }
		app.Terminal.OnNaturalLanguage = func(line string, viewContext string) {
			log.Log("Thinking…")
			summary, err := app.Agent.Run(context.Background(), line, viewContext)
			if err != nil {
				log.Log(err.Error())
			} else {
				log.Log(summary)
			}
		}
	}

	// UI: CSS-driven overlay.
	for _, path := range []string{"assets/ui/default.css", "../../assets/ui/default.css"} {
		if err := app.UI.LoadCSS(path); err == nil {
			break
		}
	}

	// Resolve engine font paths for lazy loading in Draw.
	app.engineFontPaths = []string{
		"assets/fonts/" + app.CurrentFont,
		"../../assets/fonts/" + app.CurrentFont,
	}

	graphics.Run(app.Update, app.Draw)
}
