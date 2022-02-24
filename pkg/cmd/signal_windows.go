package cmd

import (
	"os"
)

var shutdownSignals = []os.Signal{os.Interrupt}
