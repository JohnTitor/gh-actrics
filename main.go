package main

import (
	"context"
	"fmt"
	"os"

	"github.com/JohnTitor/gh-actrics/cmd"
)

func main() {
	if err := cmd.Execute(context.Background(), os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
