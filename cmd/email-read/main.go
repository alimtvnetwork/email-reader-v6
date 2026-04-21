package main

import (
	"fmt"
	"os"
)

// Version is the CLI version. Bumped per release.
const Version = "0.1.0"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "version" || os.Args[1] == "-v") {
		fmt.Printf("email-read %s\n", Version)
		return
	}
	fmt.Printf("email-read %s\n", Version)
	fmt.Println("scaffold ok — commands wired in later steps")
}
