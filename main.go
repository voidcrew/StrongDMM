package main

import (
	"os"

	"sdmm/internal/app"
	"sdmm/internal/startup"
)

func main() {
	os.Exit(startup.Run(app.Start))
}
