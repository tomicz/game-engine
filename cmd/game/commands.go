package main

import (
	"flag"
	"fmt"
	"game-engine/internal/download"
	"game-engine/internal/fonts"
	"game-engine/internal/googlefonts"
	"game-engine/internal/mapgen"
	"game-engine/internal/scene"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func registerCommands(app *App) {
	reg := app.Registry
	scn := app.Scene
	log := app.Log

	// grid: --show / --hide to show or hide the editor grid
	var showGrid, hideGrid bool
	gridFS := flag.NewFlagSet("grid", flag.ContinueOnError)
	gridFS.BoolVar(&showGrid, "show", false, "show grid")
	gridFS.BoolVar(&hideGrid, "hide", false, "hide grid")
	reg.Register("grid", gridFS, func() error {
		s, h := showGrid, hideGrid
		showGrid, hideGrid = false, false
		if s {
			scn.SetGridVisible(true)
		}
		if h {
			scn.SetGridVisible(false)
		}
		app.SaveEnginePrefs()
		return nil
	})

	// fps: --show / --hide to show or hide the FPS counter
	var showFPS, hideFPS bool
	fpsFS := flag.NewFlagSet("fps", flag.ContinueOnError)
	fpsFS.BoolVar(&showFPS, "show", false, "show FPS")
	fpsFS.BoolVar(&hideFPS, "hide", false, "hide FPS")
	reg.Register("fps", fpsFS, func() error {
		s, h := showFPS, hideFPS
		showFPS, hideFPS = false, false
		if s {
			app.Debug.SetShowFPS(true)
		}
		if h {
			app.Debug.SetShowFPS(false)
		}
		app.SaveEnginePrefs()
		return nil
	})

	// memalloc: --show / --hide to show or hide the memory counter
	var showMemAlloc, hideMemAlloc bool
	memallocFS := flag.NewFlagSet("memalloc", flag.ContinueOnError)
	memallocFS.BoolVar(&showMemAlloc, "show", false, "show memory allocation")
	memallocFS.BoolVar(&hideMemAlloc, "hide", false, "hide memory allocation")
	reg.Register("memalloc", memallocFS, func() error {
		s, h := showMemAlloc, hideMemAlloc
		showMemAlloc, hideMemAlloc = false, false
		if s {
			app.Debug.SetShowMemAlloc(true)
		}
		if h {
			app.Debug.SetShowMemAlloc(false)
		}
		app.SaveEnginePrefs()
		return nil
	})

	// window: --fullscreen / --windowed to switch display mode
	registerWindowCmd(app)

	// spawn: add a primitive at a position. Usage: cmd spawn <type> <x> <y> <z> [sx sy sz]
	registerSpawnCmd(app)

	// save: write current scene to YAML
	saveFS := flag.NewFlagSet("save", flag.ContinueOnError)
	reg.Register("save", saveFS, func() error {
		return scn.SaveScene()
	})

	// newscene: clear all primitives and save an empty scene
	newsceneFS := flag.NewFlagSet("newscene", flag.ContinueOnError)
	reg.Register("newscene", newsceneFS, func() error {
		return scn.NewScene()
	})

	// model: set AI model for natural-language commands
	registerModelCmd(app)

	// physics: enable or disable falling/collision for the selected object
	physicsFS := flag.NewFlagSet("physics", flag.ContinueOnError)
	reg.Register("physics", physicsFS, func() error {
		args := physicsFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd physics on | cmd physics off (select an object first)")
		}
		switch args[0] {
		case "on":
			return scn.SetSelectedPhysics(true)
		case "off":
			return scn.SetSelectedPhysics(false)
		default:
			return fmt.Errorf("use on or off (e.g. cmd physics off)")
		}
	})

	// delete, select, look: view-based object commands using shared argument parser
	registerDeleteCmd(app)
	registerSelectCmd(app)
	registerLookCmd(app)

	// inspect: print details about an object
	inspectFS := flag.NewFlagSet("inspect", flag.ContinueOnError)
	reg.Register("inspect", inspectFS, func() error {
		args := inspectFS.Args()
		if len(args) != 0 {
			return fmt.Errorf("usage: cmd inspect (no arguments)")
		}
		if obj, ok := scn.SelectedObject(); ok {
			log.Log(formatObjectInfo("Selected", obj))
			return nil
		}
		visible := scn.ObjectsInView()
		if len(visible) == 0 {
			return fmt.Errorf("no objects in view")
		}
		log.Log(formatObjectInfo("Closest in view", visible[0].Object))
		return nil
	})

	// download, texture, skybox
	registerDownloadCmd(app)
	registerTextureCmd(app)
	registerSkyboxCmd(app)

	// color: set RGB (0-1) on selected object
	colorFS := flag.NewFlagSet("color", flag.ContinueOnError)
	reg.Register("color", colorFS, func() error {
		args := colorFS.Args()
		if len(args) < 3 {
			return fmt.Errorf("usage: cmd color <r> <g> <b> (0-1, e.g. cmd color 1 0 0)")
		}
		var c [3]float32
		for i := 0; i < 3; i++ {
			f, err := strconv.ParseFloat(args[i], 32)
			if err != nil || f < 0 || f > 1 {
				return fmt.Errorf("color components must be 0-1")
			}
			c[i] = float32(f)
		}
		return scn.SetSelectedColor(c)
	})

	// duplicate: clone selected object N times with offset
	duplicateFS := flag.NewFlagSet("duplicate", flag.ContinueOnError)
	reg.Register("duplicate", duplicateFS, func() error {
		n := 1
		if args := duplicateFS.Args(); len(args) >= 1 {
			if v, err := strconv.Atoi(args[0]); err == nil && v >= 1 {
				n = v
			}
		}
		offset := [3]float32{2, 0, 0}
		count, err := scn.DuplicateSelected(n, offset)
		if err != nil {
			return err
		}
		log.Log(fmt.Sprintf("Duplicated %d time(s).", count))
		return nil
	})

	// screenshot: capture current view to screenshot.png
	registerScreenshotCmd(app)

	// lighting: set time-of-day profile
	lightingFS := flag.NewFlagSet("lighting", flag.ContinueOnError)
	reg.Register("lighting", lightingFS, func() error {
		args := lightingFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd lighting noon | sunset | night")
		}
		scn.SetLighting(args[0])
		return nil
	})

	// name: set name on selected object
	nameFS := flag.NewFlagSet("name", flag.ContinueOnError)
	reg.Register("name", nameFS, func() error {
		args := nameFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd name <name>")
		}
		return scn.SetSelectedName(args[0])
	})

	// motion: set motion on selected
	motionFS := flag.NewFlagSet("motion", flag.ContinueOnError)
	reg.Register("motion", motionFS, func() error {
		args := motionFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd motion off | bob")
		}
		m := args[0]
		if m == "off" {
			m = ""
		}
		return scn.SetSelectedMotion(m)
	})

	// undo: revert last add or delete
	undoFS := flag.NewFlagSet("undo", flag.ContinueOnError)
	reg.Register("undo", undoFS, func() error {
		return scn.Undo()
	})

	// focus: point camera at selected object
	focusFS := flag.NewFlagSet("focus", flag.ContinueOnError)
	reg.Register("focus", focusFS, func() error {
		return scn.FocusOnSelected()
	})

	// view: list objects currently visible to the camera
	viewFS := flag.NewFlagSet("view", flag.ContinueOnError)
	reg.Register("view", viewFS, func() error {
		visible := scn.ObjectsInView()
		if len(visible) == 0 {
			log.Log("No objects in view. Move the camera to look at primitives.")
			return nil
		}
		log.Log(fmt.Sprintf("%d object(s) in view (closest first):", len(visible)))
		for _, v := range visible {
			name := v.Object.Name
			if name == "" {
				name = fmt.Sprintf("#%d", v.Index)
			}
			log.Log(fmt.Sprintf("  %s — %s — distance %.2f — screen (%.0f, %.0f)",
				name, v.Object.Type, v.Distance, v.ScreenPos.X, v.ScreenPos.Y))
		}
		return nil
	})

	// gravity: set gravity strength/direction
	gravityFS := flag.NewFlagSet("gravity", flag.ContinueOnError)
	reg.Register("gravity", gravityFS, func() error {
		args := gravityFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd gravity <y> (e.g. cmd gravity -9.8 or 0 for zero-g)")
		}
		f, err := strconv.ParseFloat(args[0], 32)
		if err != nil {
			return fmt.Errorf("gravity must be a number")
		}
		scn.SetGravity([3]float32{0, float32(f), 0})
		return nil
	})

	// heightmap: procedurally generate a random height map
	registerHeightmapCmd(app)

	// terrain_repeat: set texture repeat for heightmap terrain
	registerTerrainRepeatCmd(app)

	// template: spawn a preset (e.g. tree)
	registerTemplateCmd(app)

	// font: set or show active UI font
	registerFontCmd(app)
}

// --- Individual command registration helpers (for commands with more complex logic) ---

func registerWindowCmd(app *App) {
	var wantFullscreen, wantWindowed bool
	windowFS := flag.NewFlagSet("window", flag.ContinueOnError)
	windowFS.BoolVar(&wantFullscreen, "fullscreen", false, "switch to fullscreen")
	windowFS.BoolVar(&wantWindowed, "windowed", false, "switch to windowed")
	app.Registry.Register("window", windowFS, func() error {
		f, w := wantFullscreen, wantWindowed
		wantFullscreen, wantWindowed = false, false
		if f == w {
			return nil
		}
		isFull := rl.IsWindowFullscreen()
		if f && !isFull {
			rl.ToggleFullscreen()
		}
		if w && isFull {
			rl.ToggleFullscreen()
		}
		return nil
	})
}

func registerSpawnCmd(app *App) {
	spawnFS := flag.NewFlagSet("spawn", flag.ContinueOnError)
	app.Registry.Register("spawn", spawnFS, func() error {
		args := spawnFS.Args()
		if len(args) != 4 && len(args) != 7 {
			return fmt.Errorf("usage: cmd spawn <type> <x> <y> <z> [sx sy sz]")
		}
		typ := args[0]
		if !primTypes[typ] {
			return fmt.Errorf("unknown type %q (use: cube, sphere, cylinder, plane)", typ)
		}
		var pos [3]float32
		for i := 0; i < 3; i++ {
			f, err := strconv.ParseFloat(args[1+i], 32)
			if err != nil {
				return fmt.Errorf("invalid position %q: %w", args[1+i], err)
			}
			pos[i] = float32(f)
		}
		scale := [3]float32{1, 1, 1}
		if len(args) == 7 {
			for i := 0; i < 3; i++ {
				f, err := strconv.ParseFloat(args[4+i], 32)
				if err != nil {
					return fmt.Errorf("invalid scale %q: %w", args[4+i], err)
				}
				scale[i] = float32(f)
			}
		}
		app.Scene.AddPrimitive(typ, pos, scale)
		app.Scene.RecordAdd(1)
		return nil
	})
}

func registerModelCmd(app *App) {
	modelFS := flag.NewFlagSet("model", flag.ContinueOnError)
	app.Registry.Register("model", modelFS, func() error {
		if app.IsOllama {
			return fmt.Errorf("cannot change model when using Ollama (disabled to prevent LLM from switching by accident)")
		}
		args := modelFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd model <name> (e.g. cmd model gpt-4o-mini)")
		}
		app.CurrentAIModel = args[0]
		app.SaveEnginePrefs()
		return nil
	})
}

func registerDeleteCmd(app *App) {
	deleteFS := flag.NewFlagSet("delete", flag.ContinueOnError)
	app.Registry.Register("delete", deleteFS, func() error {
		args := deleteFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd delete selected | look | random | name <name> | left|right|top|bottom | [color] <type> [position] | all [type|name]")
		}
		scn := app.Scene

		switch args[0] {
		case "selected":
			return scn.DeleteSelected()
		case "look", "camera":
			return scn.DeleteAtCameraLook()
		case "random":
			return scn.DeleteRandom()
		case "name":
			if len(args) < 2 {
				return fmt.Errorf("usage: cmd delete name <name>")
			}
			_, err := scn.DeleteByName(args[1])
			return err
		case "all":
			return deleteAll(scn, args[1:])
		}

		q := parseObjectArgs(args)
		if q.Position != "" && q.Type == "" && q.Name == "" {
			return scn.DeleteVisibleByPosition(q.Position)
		}
		if q.Type != "" && q.Position == "" {
			return scn.DeleteVisibleByDescription(q.Type, q.Color)
		}
		if q.Type != "" && q.Position != "" {
			return scn.DeleteVisibleByDescriptionAndPosition(q.Type, q.Color, "", q.Position)
		}
		if q.Name != "" && q.Position != "" {
			return scn.DeleteVisibleByDescriptionAndPosition("", nil, q.Name, q.Position)
		}
		if q.Name != "" {
			return scn.DeleteVisibleByDescription(q.Name, nil)
		}

		return fmt.Errorf("use selected, look, random, name <name>, left|right|top|bottom, [color] <type> [position], or all [type|name]")
	})
}

func deleteAll(scn *scene.Scene, args []string) error {
	if len(args) == 0 {
		n, err := scn.DeleteAllVisibleByDescription("", nil, "")
		if err != nil {
			return err
		}
		fmt.Printf("Deleted %d object(s) in view.\n", n)
		return nil
	}
	if len(args) == 1 {
		a := strings.ToLower(args[0])
		if primTypes[a] {
			n, err := scn.DeleteAllVisibleByDescription(a, nil, "")
			if err != nil {
				return err
			}
			fmt.Printf("Deleted %d %s(s) in view.\n", n, a)
			return nil
		}
		n, err := scn.DeleteAllVisibleByDescription("", nil, a)
		if err != nil {
			return err
		}
		fmt.Printf("Deleted %d object(s) matching %q in view.\n", n, a)
		return nil
	}
	if len(args) == 2 {
		colorName := strings.ToLower(args[0])
		typ := strings.ToLower(args[1])
		if primTypes[typ] {
			if c, ok := colorNames[colorName]; ok {
				n, err := scn.DeleteAllVisibleByDescription(typ, &c, "")
				if err != nil {
					return err
				}
				fmt.Printf("Deleted %d %s %s(s) in view.\n", n, colorName, typ)
				return nil
			}
		}
	}
	return fmt.Errorf("usage: cmd delete all [type] or delete all [color] [type] or delete all <name_substring>")
}

func registerSelectCmd(app *App) {
	selectFS := flag.NewFlagSet("select", flag.ContinueOnError)
	app.Registry.Register("select", selectFS, func() error {
		args := selectFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd select none | left|right|... | [color] <type> [position] | <name> [position]")
		}
		scn := app.Scene

		if strings.ToLower(args[0]) == "none" {
			scn.ClearSelection()
			return nil
		}

		q := parseObjectArgs(args)
		if q.Position != "" && q.Type == "" && q.Name == "" {
			return scn.SelectVisibleByPosition(q.Position)
		}
		return scn.SelectVisibleByDescriptionAndPosition(q.Type, q.Color, q.Name, q.Position)
	})
}

func registerLookCmd(app *App) {
	lookFS := flag.NewFlagSet("look", flag.ContinueOnError)
	app.Registry.Register("look", lookFS, func() error {
		args := lookFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd look left|right|... | [color] <type> [position] | <name> [position]")
		}
		scn := app.Scene

		q := parseObjectArgs(args)
		if q.Position != "" && q.Type == "" && q.Name == "" {
			return scn.FocusOnVisibleByPosition(q.Position)
		}
		return scn.FocusOnVisibleByDescriptionAndPosition(q.Type, q.Color, q.Name, q.Position)
	})
}

func registerDownloadCmd(app *App) {
	downloadFS := flag.NewFlagSet("download", flag.ContinueOnError)
	app.Registry.Register("download", downloadFS, func() error {
		args := downloadFS.Args()
		if len(args) < 2 {
			return fmt.Errorf("usage: cmd download image <url> (select an object first)")
		}
		if args[0] != "image" {
			return fmt.Errorf("usage: cmd download image <url>")
		}
		url := args[1]
		if url == "" {
			return fmt.Errorf("url is required")
		}
		idx := app.Scene.SelectedIndex()
		if idx < 0 {
			return fmt.Errorf("no object selected (click an object with terminal open)")
		}
		go func() {
			relPath, err := download.Download(url, "assets/textures/downloaded")
			app.DownloadDone <- &downloadResult{Index: idx, Path: relPath, Err: err}
		}()
		return nil
	})
}

func registerTextureCmd(app *App) {
	textureFS := flag.NewFlagSet("texture", flag.ContinueOnError)
	app.Registry.Register("texture", textureFS, func() error {
		args := textureFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd texture <path>")
		}
		path := args[0]
		if path == "" {
			return fmt.Errorf("path is required")
		}
		if app.Scene.SelectedIndex() < 0 {
			return fmt.Errorf("no object selected (click an object with terminal open)")
		}
		return app.Scene.SetSelectedTexture(path)
	})
}

func registerSkyboxCmd(app *App) {
	skyboxFS := flag.NewFlagSet("skybox", flag.ContinueOnError)
	app.Registry.Register("skybox", skyboxFS, func() error {
		args := skyboxFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd skybox <url>")
		}
		url := args[0]
		if url == "" {
			return fmt.Errorf("url is required")
		}
		go func() {
			relPath, err := downloadImage(url, "assets/skybox/downloaded")
			app.SkyboxDone <- &skyboxResult{Path: relPath, Err: err}
		}()
		return nil
	})
}

func registerScreenshotCmd(app *App) {
	screenshotFS := flag.NewFlagSet("screenshot", flag.ContinueOnError)
	app.Registry.Register("screenshot", screenshotFS, func() error {
		rl.TakeScreenshot("screenshot.png")
		app.Log.Log("Screenshot saved: screenshot.png")
		return nil
	})
}

func registerHeightmapCmd(app *App) {
	var hmWidth, hmDepth int
	var hmTileSize, hmMaxHeight float64
	var hmSeed int64
	heightmapFS := flag.NewFlagSet("heightmap", flag.ContinueOnError)
	heightmapFS.IntVar(&hmWidth, "w", 0, "width in tiles")
	heightmapFS.IntVar(&hmDepth, "d", 0, "depth in tiles")
	heightmapFS.Float64Var(&hmTileSize, "tile", 0, "tile size on X/Z")
	heightmapFS.Float64Var(&hmMaxHeight, "h", 0, "max height")
	heightmapFS.Int64Var(&hmSeed, "seed", 0, "random seed (0 = random)")
	app.Registry.Register("heightmap", heightmapFS, func() error {
		opts := mapgen.DefaultHeightMapOptions()
		if hmWidth > 0 {
			opts.Width = hmWidth
		}
		if hmDepth > 0 {
			opts.Depth = hmDepth
		}
		if hmTileSize > 0 {
			opts.TileSize = float32(hmTileSize)
		}
		if hmMaxHeight > 0 {
			opts.HeightScale = float32(hmMaxHeight)
		}
		if hmSeed != 0 {
			opts.Seed = hmSeed
		}
		if err := mapgen.ApplyHeightmapTerrain(app.Scene, opts); err != nil {
			return err
		}
		app.Log.Log(fmt.Sprintf("Heightmap generated (terrain mesh %dx%d).", opts.Width, opts.Depth))
		return nil
	})
}

func registerTerrainRepeatCmd(app *App) {
	terrainRepeatFS := flag.NewFlagSet("terrain_repeat", flag.ContinueOnError)
	app.Registry.Register("terrain_repeat", terrainRepeatFS, func() error {
		args := terrainRepeatFS.Args()
		if len(args) < 2 {
			return fmt.Errorf("usage: cmd terrain_repeat <u> <v> (e.g. cmd terrain_repeat 4 4)")
		}
		u, err1 := strconv.ParseFloat(args[0], 32)
		v, err2 := strconv.ParseFloat(args[1], 32)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("terrain_repeat: u and v must be numbers")
		}
		if u <= 0 || v <= 0 {
			return fmt.Errorf("terrain_repeat: u and v must be > 0")
		}
		app.Scene.SetTerrainTextureRepeat(float32(u), float32(v))
		app.Log.Log(fmt.Sprintf("Terrain texture repeat set to %.2fx, %.2fy.", u, v))
		return nil
	})
}

func registerTemplateCmd(app *App) {
	templateFS := flag.NewFlagSet("template", flag.ContinueOnError)
	app.Registry.Register("template", templateFS, func() error {
		args := templateFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd template tree [x y z]")
		}
		x, y, z := 0.0, 0.0, 0.0
		if len(args) >= 4 {
			for i, s := range []*float64{&x, &y, &z} {
				if f, err := strconv.ParseFloat(args[1+i], 32); err == nil {
					*s = f
				}
			}
		}
		switch args[0] {
		case "tree":
			_ = app.Registry.Execute([]string{"spawn", "cylinder", strconv.FormatFloat(x, 'f', -1, 32), strconv.FormatFloat(y, 'f', -1, 32), strconv.FormatFloat(z, 'f', -1, 32), "0.3", "2", "0.3"})
			_ = app.Registry.Execute([]string{"spawn", "sphere", strconv.FormatFloat(x, 'f', -1, 32), strconv.FormatFloat(y+1.5, 'f', -1, 32), strconv.FormatFloat(z, 'f', -1, 32), "1.2", "1.2", "1.2"})
			app.Log.Log("Spawned tree.")
		default:
			return fmt.Errorf("unknown template (use tree)")
		}
		return nil
	})
}

func registerFontCmd(app *App) {
	fontFS := flag.NewFlagSet("font", flag.ContinueOnError)
	app.Registry.Register("font", fontFS, func() error {
		args := fontFS.Args()
		if len(args) < 1 {
			app.Log.Log("Current font: " + app.CurrentFont)
			return nil
		}
		rel := args[0]
		rel = fonts.StripAssetsFontsPrefix(rel)
		// Try direct path first
		for _, p := range []string{"assets/fonts/" + rel, "../../assets/fonts/" + rel} {
			if err := app.UI.LoadFont(p); err == nil {
				app.CurrentFont = rel
				app.Terminal.SetFont(app.UI.Font())
				app.Debug.SetFont(app.UI.Font())
				app.SaveEnginePrefs()
				app.Log.Log("Font set: " + rel)
				return nil
			}
		}
		// Search assets/fonts for a matching file
		for _, search := range fonts.SearchCandidates(rel) {
			if foundRel, fullPath, findErr := fonts.FindFont(search); findErr == nil {
				if err := app.UI.LoadFont(fullPath); err == nil {
					app.CurrentFont = foundRel
					app.Terminal.SetFont(app.UI.Font())
					app.Debug.SetFont(app.UI.Font())
					app.SaveEnginePrefs()
					app.Log.Log("Font set: " + foundRel)
					return nil
				}
			}
		}
		// Not found locally: download from Google Fonts
		go func() {
			res := &fontDownloadResult{}
			defer func() { app.FontDownloadDone <- res }()
			downloadURL, err := googlefonts.FetchDownloadURLByFamily(rel)
			if err != nil {
				res.Err = err
				return
			}
			var baseDir string
			for _, d := range []string{"assets/fonts", "../../assets/fonts"} {
				if err := os.MkdirAll(filepath.Join(d, "downloaded"), 0755); err == nil {
					baseDir = d
					break
				}
			}
			if baseDir == "" {
				res.Err = fmt.Errorf("cannot create assets/fonts/downloaded")
				return
			}
			folder := googlefonts.NormalizeFamily(rel)[0]
			downloadDir := filepath.Join(baseDir, "downloaded", folder)
			if err := os.MkdirAll(downloadDir, 0755); err != nil {
				res.Err = err
				return
			}
			savedPath, err := download.Download(downloadURL, downloadDir)
			if err != nil {
				res.Err = err
				return
			}
			res.RelPath = filepath.ToSlash("downloaded/" + folder + "/" + filepath.Base(savedPath))
			res.FullPath = savedPath
		}()
		app.Log.Log("Downloading font from Google Fonts…")
		return nil
	})
}

func formatObjectInfo(label string, obj scene.ObjectInstance) string {
	return fmt.Sprintf("%s: type=%s name=%q pos=[%.2f,%.2f,%.2f] scale=[%.2f,%.2f,%.2f] color=[%.2f,%.2f,%.2f] physics=%v motion=%q texture=%q",
		label,
		obj.Type, obj.Name,
		obj.Position[0], obj.Position[1], obj.Position[2],
		obj.Scale[0], obj.Scale[1], obj.Scale[2],
		obj.Color[0], obj.Color[1], obj.Color[2],
		scene.PhysicsEnabledForObject(obj), obj.Motion, obj.Texture)
}
