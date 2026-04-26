package theme

import "image/color"

// paletteLight mirrors paletteDark with the Light-mode RGB values from
// spec/24-app-design-system-and-ui/01-tokens.md §2. Both maps MUST stay
// key-parallel (palette_test.go enforces it).
var paletteLight = map[ColorName]color.NRGBA{
	// §2.1 surface/foreground
	ColorBackground:         {250, 250, 252, 255},
	ColorSurface:            {255, 255, 255, 255},
	ColorSurfaceMuted:       {244, 245, 248, 255},
	ColorBorder:             {224, 226, 232, 255},
	ColorForeground:         {15, 17, 21, 255},
	ColorForegroundMuted:    {90, 96, 110, 255},
	ColorForegroundDisabled: {170, 175, 185, 255},

	// §2.2 brand
	ColorPrimary:           {42, 100, 245, 255},
	ColorPrimaryForeground: {255, 255, 255, 255},
	ColorAccent:            {122, 82, 220, 255},

	// §2.3 status
	ColorSuccess: {34, 160, 90, 255},
	ColorWarning: {200, 140, 20, 255},
	ColorError:   {200, 40, 60, 255},
	ColorInfo:    {30, 130, 210, 255},

	// §2.4 sidebar
	ColorSidebar:                     {247, 248, 251, 255},
	ColorSidebarForeground:           {60, 66, 78, 255},
	ColorSidebarItemHover:            {230, 233, 240, 255},
	ColorSidebarItemActive:           {220, 230, 252, 255},
	ColorSidebarItemActiveForeground: {42, 100, 245, 255},
	ColorSidebarBorder:               {224, 226, 232, 255},

	// §2.5 watch dots
	ColorWatchDotIdle:         {140, 145, 155, 255},
	ColorWatchDotStarting:     {60, 140, 220, 255},
	ColorWatchDotWatching:     {34, 160, 90, 255},
	ColorWatchDotReconnecting: {200, 140, 20, 255},
	ColorWatchDotStopping:     {90, 170, 230, 255},
	ColorWatchDotError:        {200, 40, 60, 255},
}
