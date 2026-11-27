package main

import (
	"os"

	"lplug/internal/lplug"
)

func main() { os.Exit(lplug.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
