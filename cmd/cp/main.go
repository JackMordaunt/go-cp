package main

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/jackmordaunt/cp"
)

func oops(f string, v ...interface{}) {
	fmt.Printf(fmt.Sprintf("%s %s", color.New(color.FgBlue).Sprintf("oops:"), f), v...)
	os.Exit(0)
}

func fatal(f string, v ...interface{}) {
	fmt.Printf(fmt.Sprintf("%s %s", color.New(color.FgRed).Sprintf("error:"), f), v...)
	os.Exit(0)
}

func main() {
	args := os.Args[1:]
	if len(args) < 2 {
		oops("not enough arguments\n")
	}
	from, to := args[0], args[1]
	copier := cp.Copier{
		Clobber: true,
	}
	if err := copier.Copy(from, to); err != nil {
		fatal("copying files: %v\n", err)
	}
}
