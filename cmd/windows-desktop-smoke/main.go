package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"jindi-drama/internal/storage/sqlite"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 2 && args[0] == "verify-db" {
		return sqlite.VerifyDesktopDatabase(args[1])
	}
	if len(args) == 3 && args[0] == "seed" {
		return seed(args[1], args[2])
	}
	return runNative(args)
}

func seed(path, scenario string) error {
	var action string
	switch scenario {
	case "Minimize":
		action = "最小化到任务栏"
	case "Tray":
		action = "隐藏到菜单栏"
	default:
		return fmt.Errorf("unknown close preference fixture %q", scenario)
	}

	// The fixture may create a database, but must never modify an existing one.
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	store, err := sqlite.Open(path)
	if err != nil {
		return err
	}
	value, _ := json.Marshal(action)
	return errors.Join(store.SetSetting("window_close_action", string(value)), store.Close())
}
