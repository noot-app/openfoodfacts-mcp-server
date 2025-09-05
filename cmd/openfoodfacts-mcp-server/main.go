package main

import (
	"fmt"
	"os"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/cmd"
)

func main() {
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
