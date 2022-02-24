package main

import (
	"fmt"
	"github.com/inconshreveable/mousetrap"
	"github.com/rifflock/lfshook"
	"github.com/shiena/ansicolor"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"math/rand"
	"nexus-sync/pkg/cmd"
	"nexus-sync/pkg/log"
	"os"
	"runtime"
	"strings"
	"time"
)

func init() {
	// Init logger
	formatter := prefixed.TextFormatter{
		ForceColors:     true,
		ForceFormatting: true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	}
	var writer = ansicolor.NewAnsiColorWriter(os.Stdout)
	wm := make(lfshook.WriterMap)
	for _, level := range logrus.AllLevels {
		wm[level] = writer
	}
	hooks := make(logrus.LevelHooks)
	hooks.Add(lfshook.NewHook(wm, &formatter))
	log.SetHooks(hooks)

	if runtime.GOOS == "windows" {
		if mousetrap.StartedByExplorer() {
			fmt.Println("Don't double-click websocket-fmp4!")
			fmt.Println("You need to open cmd.exe and run it from the command line!")
			time.Sleep(5 * time.Second)
			os.Exit(1)
		}
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	command := cmd.NewNexusSyncCommand()
	command.SetGlobalNormalizationFunc(func(f *pflag.FlagSet, name string) pflag.NormalizedName {
		if strings.Contains(name, "_") {
			return pflag.NormalizedName(strings.Replace(name, "_", "-", -1))
		}
		return pflag.NormalizedName(name)
	})

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
