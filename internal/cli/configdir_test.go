package cli

import (
	"errors"
	"os"
	"testing"
	"time"
)

type fakeFileInfo struct{ isDir bool }

func (f fakeFileInfo) Name() string       { return "" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.isDir }
func (f fakeFileInfo) Sys() interface{}   { return nil }

func TestFindConfigDir_PreferenceOrder(t *testing.T) {
	calls := []string{}
	stat := func(path string) (os.FileInfo, error) {
		calls = append(calls, path)
		switch path {
		case "/override":
			return fakeFileInfo{true}, nil
		case "/tmp123/dockswap-cfg":
			return fakeFileInfo{true}, nil
		case "/home/.config/dockswap-cfg":
			return fakeFileInfo{true}, nil
		case "/etc/dockswap-cfg/":
			return fakeFileInfo{true}, nil
		}
		return nil, errors.New("not found")
	}
	home := func() (string, error) { return "/home", nil }
	getwd := func() (string, error) { return "/tmp123", nil }

	// 1. --config
	flags := GlobalFlags{Config: "/override"}
	path, err := FindConfigDir(flags, stat, home, getwd)
	if err != nil || path != "/override" {
		t.Errorf("expected /override, got %v, %v", path, err)
	}

	// 2. ./dockswap-cfg
	flags = GlobalFlags{}
	path, err = FindConfigDir(flags, stat, home, getwd)
	if err != nil || path != "/tmp123/dockswap-cfg" {
		t.Errorf("expected /tmp123/dockswap-cfg, got %v, %v", path, err)
	}

	// 3. $HOME/.config/dockswap-cfg
	stat = func(path string) (os.FileInfo, error) {
		if path == "/home/.config/dockswap-cfg" {
			return fakeFileInfo{true}, nil
		}
		return nil, errors.New("not found")
	}
	path, err = FindConfigDir(flags, stat, home, getwd)
	if err != nil || path != "/home/.config/dockswap-cfg" {
		t.Errorf("expected /home/.config/dockswap-cfg, got %v, %v", path, err)
	}

	// 4. /etc/dockswap-cfg/
	stat = func(path string) (os.FileInfo, error) {
		if path == "/etc/dockswap-cfg/" {
			return fakeFileInfo{true}, nil
		}
		return nil, errors.New("not found")
	}
	path, err = FindConfigDir(flags, stat, home, getwd)
	if err != nil || path != "/etc/dockswap-cfg/" {
		t.Errorf("expected /etc/dockswap-cfg/, got %v, %v", path, err)
	}

	// 5. None exist
	stat = func(path string) (os.FileInfo, error) { return nil, errors.New("not found") }
	path, err = FindConfigDir(flags, stat, home, getwd)
	if err == nil {
		t.Errorf("expected error, got %v", path)
	}
}

func TestFindConfigDir_RelativeOverride(t *testing.T) {
	stat := func(path string) (os.FileInfo, error) {
		if path == "./mycfg" {
			return fakeFileInfo{true}, nil
		}
		return nil, errors.New("not found")
	}
	home := func() (string, error) { return "/home", nil }
	getwd := func() (string, error) { return "/tmp", nil }
	flags := GlobalFlags{Config: "./mycfg"}
	path, err := FindConfigDir(flags, stat, home, getwd)
	if err != nil || path != "./mycfg" {
		t.Errorf("expected ./mycfg, got %v, %v", path, err)
	}
}
