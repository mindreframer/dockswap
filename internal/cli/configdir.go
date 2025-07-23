package cli

import (
	"errors"
	"os"
	"path/filepath"
)

type StatFunc func(string) (os.FileInfo, error)
type HomeDirFunc func() (string, error)
type GetwdFunc func() (string, error)

// FindConfigDir returns the most relevant config directory according to preference order.
// Optionally accepts statFunc, homeDirFunc, and getwdFunc for testability (if nil, uses os.Stat, os.UserHomeDir, os.Getwd).
func FindConfigDir(flags GlobalFlags, statFunc StatFunc, homeDirFunc HomeDirFunc, getwdFunc GetwdFunc) (string, error) {
	if statFunc == nil {
		statFunc = os.Stat
	}
	if homeDirFunc == nil {
		homeDirFunc = os.UserHomeDir
	}
	if getwdFunc == nil {
		getwdFunc = os.Getwd
	}

	// 1. --config arg
	if flags.Config != "" {
		if info, err := statFunc(flags.Config); err == nil && info.IsDir() {
			return flags.Config, nil
		}
	}

	// 2. ./dockswap-cfg
	cwd, _ := getwdFunc()
	local := filepath.Join(cwd, "dockswap-cfg")
	if info, err := statFunc(local); err == nil && info.IsDir() {
		return local, nil
	}

	// 3. $HOME/.config/dockswap-cfg
	home, err := homeDirFunc()
	if err == nil {
		homeCfg := filepath.Join(home, ".config", "dockswap-cfg")
		if info, err := statFunc(homeCfg); err == nil && info.IsDir() {
			return homeCfg, nil
		}
	}

	// 4. /etc/dockswap-cfg/
	etc := "/etc/dockswap-cfg/"
	if info, err := statFunc(etc); err == nil && info.IsDir() {
		return etc, nil
	}

	return "", errors.New("no config directory found (tried --config, ./dockswap-cfg, $HOME/.config/dockswap-cfg, /etc/dockswap-cfg/)")
}
