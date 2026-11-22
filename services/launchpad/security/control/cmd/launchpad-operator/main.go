package main

import "os"

func main() {
	os.Exit(Run(os.Args[1:], os.Environ(), os.Stdin, os.Stdout))
}
