package main

import "strings"

// Shared maps and parsing logic for commands that accept [color] <type> [position] arguments.

var primTypes = map[string]bool{
	"cube": true, "sphere": true, "cylinder": true, "plane": true, "terrain": true,
}

var positionWords = map[string]bool{
	"left": true, "right": true, "top": true, "bottom": true, "closest": true, "farthest": true,
}

var colorNames = map[string][3]float32{
	"red": {1, 0, 0}, "green": {0, 1, 0}, "blue": {0, 0, 1}, "yellow": {1, 1, 0},
	"orange": {1, 0.5, 0}, "purple": {0.5, 0, 0.5}, "pink": {1, 0.75, 0.8},
	"white": {1, 1, 1}, "black": {0, 0, 0}, "gray": {0.5, 0.5, 0.5}, "grey": {0.5, 0.5, 0.5},
}

// objectQuery represents a parsed [color] <type> [position] or <name> [position] query.
type objectQuery struct {
	Type     string       // primitive type ("cube", etc.) or empty
	Color    *[3]float32  // optional color filter
	Name     string       // name substring or empty
	Position string       // position word ("left", etc.) or empty
}

// parseObjectArgs parses args like: left | cube | red cube | cube left | red cube left | building right
func parseObjectArgs(args []string) objectQuery {
	switch len(args) {
	case 1:
		a0 := strings.ToLower(args[0])
		if positionWords[a0] {
			return objectQuery{Position: a0}
		}
		if primTypes[a0] {
			return objectQuery{Type: a0}
		}
		return objectQuery{Name: a0}

	case 2:
		a0, a1 := strings.ToLower(args[0]), strings.ToLower(args[1])
		if primTypes[a0] && positionWords[a1] {
			return objectQuery{Type: a0, Position: a1}
		}
		if c, ok := colorNames[a0]; ok && primTypes[a1] {
			return objectQuery{Type: a1, Color: &c}
		}
		if positionWords[a1] {
			return objectQuery{Name: a0, Position: a1}
		}

	case 3:
		a0, a1, a2 := strings.ToLower(args[0]), strings.ToLower(args[1]), strings.ToLower(args[2])
		if c, ok := colorNames[a0]; ok && primTypes[a1] && positionWords[a2] {
			return objectQuery{Type: a1, Color: &c, Position: a2}
		}
	}

	return objectQuery{}
}
