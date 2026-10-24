package simulator

import (
	"fmt"
	"os"
	"path/filepath"
)

// SessionPaths describes an isolated SDK view. Root must be dedicated to this
// interactive instance, never a smoke-test directory. Saves survive reconnects.
type SessionPaths struct{ Root, SDK, Config, Data, Cartridge, Executable string }

// PrepareSession creates only a marked, dedicated directory. Existing unmarked
// directories and symlink roots are refused rather than adopting test/user data.
// It does not start processes or alter the installed SDK or production cartridge.
func PrepareSession(root, sdk, cartridge string) (SessionPaths, error) {
	var p SessionPaths
	var err error
	for _, path := range []string{root, sdk, cartridge} {
		if !filepath.IsAbs(path) {
			return p, fmt.Errorf("absolute paths required")
		}
	}
	p = SessionPaths{Root: filepath.Clean(root), SDK: filepath.Join(root, "sdk"), Config: filepath.Join(root, "config"), Data: filepath.Join(root, "data"), Cartridge: cartridge, Executable: filepath.Join(sdk, "bin", "PlaydateSimulator")}
	for _, path := range []string{p.Executable, filepath.Join(cartridge, "pdxinfo")} {
		if _, err = os.Stat(path); err != nil {
			return SessionPaths{}, err
		}
	}
	marker := filepath.Join(root, ".playdate-web-bridge-session-v1")
	if info, e := os.Lstat(root); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return SessionPaths{}, fmt.Errorf("session root must be a real directory")
		}
		content, e := os.ReadFile(marker)
		if e != nil || string(content) != "playdate-web-bridge\n" {
			return SessionPaths{}, fmt.Errorf("refusing unmarked session directory")
		}
	} else if os.IsNotExist(e) {
		if err = os.Mkdir(root, 0700); err != nil {
			return SessionPaths{}, err
		}
		if err = os.WriteFile(marker, []byte("playdate-web-bridge\n"), 0600); err != nil {
			return SessionPaths{}, err
		}
	} else {
		return SessionPaths{}, e
	}
	// Reject symlinks anywhere inside the writable tree before following paths.
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			rel, _ := filepath.Rel(p.SDK, path)
			allowed := map[string]bool{"bin": true, "CoreLibs": true, "Resources": true, "VERSION.txt": true}
			for _, part := range []string{"Assets", "Fonts", "GameLibrary.pdx", "InputTest.pdx", "Launcher.pdx", "Settings.pdx", "Setup.pdx"} {
				allowed[filepath.Join("Disk", "System", part)] = true
			}
			target, e := os.Readlink(path)
			if !allowed[rel] || e != nil || target != filepath.Join(sdk, rel) {
				return fmt.Errorf("unexpected session symlink: %s", path)
			}
		}
		return nil
	})
	if err != nil {
		return SessionPaths{}, err
	}
	for _, dir := range []string{p.Config, p.Data, filepath.Join(p.Config, "Playdate Simulator"), filepath.Join(p.SDK, "Disk", "Data"), filepath.Join(p.SDK, "Disk", "Games"), filepath.Join(p.SDK, "Disk", "Shared"), filepath.Join(p.SDK, "Disk", "Screenshots"), filepath.Join(p.SDK, "Disk", "System", "Data")} {
		if err = os.MkdirAll(dir, 0700); err != nil {
			return SessionPaths{}, err
		}
	}
	for _, part := range []string{"bin", "CoreLibs", "Resources", "VERSION.txt", "Disk/System/Assets", "Disk/System/Fonts", "Disk/System/GameLibrary.pdx", "Disk/System/InputTest.pdx", "Disk/System/Launcher.pdx", "Disk/System/Settings.pdx", "Disk/System/Setup.pdx"} {
		target := filepath.Join(sdk, part)
		link := filepath.Join(p.SDK, part)
		if _, err = os.Stat(target); err != nil {
			return SessionPaths{}, err
		}
		if err = os.Symlink(target, link); err != nil && !os.IsExist(err) {
			return SessionPaths{}, err
		}
	}
	for path, content := range map[string]string{filepath.Join(p.Config, "Playdate Simulator", "Playdate Simulator.ini"): "ShowPerfWarning=0\nShowElist=0\n", filepath.Join(p.SDK, "Disk", "System", "Data", "global.settings"): "volume=5\n"} {
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(e) {
			continue
		}
		if e != nil {
			return SessionPaths{}, e
		}
		_, e = f.WriteString(content)
		closeErr := f.Close()
		if e != nil {
			return SessionPaths{}, e
		}
		if closeErr != nil {
			return SessionPaths{}, closeErr
		}
	}
	return p, nil
}
