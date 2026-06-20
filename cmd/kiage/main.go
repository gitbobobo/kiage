package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	_ "time/tzdata" // Kindle 无系统 zoneinfo，需内嵌时区库

	"github.com/godbobo/kiage/internal/app"
	"github.com/godbobo/kiage/internal/config"
	"github.com/godbobo/kiage/internal/log"
	"github.com/godbobo/kiage/internal/paths"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "run":
		runCmd()
	case "dev":
		devCmd()
	case "fetch":
		fetchCmd()
	case "import-edge":
		importEdgeCmd()
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: kiage <run|dev|fetch|import-edge>\n")
}

func runCmd() {
	roots := paths.Resolve()
	if err := log.Init(roots.Log); err != nil {
		fmt.Fprintf(os.Stderr, "log init %s: %v\n", roots.Log, err)
	}
	defer func() {
		if r := recover(); r != nil {
			log.Error("panic: %v\n%s", r, debug.Stack())
			os.Exit(2)
		}
	}()
	defer log.Close()
	defer log.Info("kiage run finished")

	app.LogRunEnvironment(roots)

	a, err := app.New(roots)
	if err != nil {
		fatal(err)
	}
	defer a.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	log.Info("entering kindle main loop")
	if err := a.RunKindle(ctx); err != nil {
		fatal(err)
	}
}

func devCmd() {
	addr := flag.NewFlagSet("dev", flag.ExitOnError)
	listen := addr.String("addr", ":8088", "listen address")
	_ = addr.Parse(os.Args[2:])
	roots := paths.Resolve()
	a, err := app.New(roots)
	if err != nil {
		fatal(err)
	}
	defer a.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := a.RunDev(ctx, *listen); err != nil && err != context.Canceled {
		fatal(err)
	}
}

func fetchCmd() {
	roots := paths.Resolve()
	a, err := app.New(roots)
	if err != nil {
		fatal(err)
	}
	defer a.Close()
	ctx := context.Background()
	if err := a.DoSync(ctx); err != nil {
		fatal(err)
	}
	fmt.Println("sync ok")
}

func importEdgeCmd() {
	token, err := config.ImportEdgeCursorToken()
	if err != nil {
		token, err = config.ImportEdgeViaPython()
	}
	if err != nil {
		fatal(err)
	}
	roots := paths.Resolve()
	cfg, err := config.Load(roots.Config)
	if err != nil {
		fatal(err)
	}
	cfg.Cursor.SessionToken = token
	if err := config.Save(roots.Config, cfg); err != nil {
		fatal(err)
	}
	fmt.Printf("imported edge token (%s) -> %s\n", config.RedactToken(token), roots.Config)
}

func fatal(err error) {
	log.Error("%v", err)
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
