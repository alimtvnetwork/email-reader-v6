// size.go isolates the fyne.NewSize call so the rest of main.go imports the
// fyne packages it actually uses without grabbing the whole namespace.
package main

import "fyne.io/fyne/v2"

func fyneSize(w, h float32) fyne.Size { return fyne.NewSize(w, h) }
