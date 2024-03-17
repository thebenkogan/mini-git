package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/thebenkogan/git/internal/git"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("No command provided")
		os.Exit(1)
	}

	git := git.Git{Root: ".bgit", Output: os.Stdout}

	var err error
	switch command := os.Args[1]; command {
	case "init":
		err = git.Init()
	case "cat-file":
		catFileCmd := flag.NewFlagSet("cat-file", flag.ExitOnError)
		shaPtr := catFileCmd.String("p", "", "sha hash of the blob")
		_ = catFileCmd.Parse(os.Args[2:])
		err = git.CatFile(*shaPtr)
	case "hash-object":
		hashObjectCmd := flag.NewFlagSet("cat-file", flag.ExitOnError)
		writePtr := hashObjectCmd.Bool("w", false, "write the blob to the objects store")
		_ = hashObjectCmd.Parse(os.Args[2:])
		err = git.HashObject(hashObjectCmd.Args()[0], *writePtr)
	default:
		err = fmt.Errorf("Unknown command %s", command)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
