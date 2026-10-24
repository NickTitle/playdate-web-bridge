package simulator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionIsolation(t *testing.T) {
	base := t.TempDir()
	sdk := filepath.Join(base, "installed")
	cart := filepath.Join(base, "Example.pdx")
	root := filepath.Join(base, "interactive")
	for _, part := range []string{"bin/PlaydateSimulator", "CoreLibs/resource", "Resources/resource", "VERSION.txt", "Disk/System/Assets/resource", "Disk/System/Fonts/resource", "Disk/System/GameLibrary.pdx/pdxinfo", "Disk/System/InputTest.pdx/pdxinfo", "Disk/System/Launcher.pdx/pdxinfo", "Disk/System/Settings.pdx/pdxinfo", "Disk/System/Setup.pdx/pdxinfo"} {
		path := filepath.Join(sdk, part)
		os.MkdirAll(filepath.Dir(path), 0700)
		os.WriteFile(path, []byte("reference"), 0600)
	}
	os.Mkdir(cart, 0700)
	os.WriteFile(filepath.Join(cart, "pdxinfo"), []byte("production"), 0600)
	p, err := PrepareSession(root, sdk, cart)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(p.Config, "Playdate Simulator", "Playdate Simulator.ini")
	if data, _ := os.ReadFile(config); !strings.Contains(string(data), "ShowElist=0") {
		t.Fatal("new session can show startup signup popup")
	}
	os.WriteFile(config, []byte("ShowElist=0\ncustom=keep\n"), 0600)
	save := filepath.Join(p.SDK, "Disk", "Data", "save.json")
	os.WriteFile(save, []byte("saved"), 0600)
	if _, err = PrepareSession(root, sdk, cart); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(config); string(data) != "ShowElist=0\ncustom=keep\n" {
		t.Fatal("existing simulator configuration overwritten")
	}
	if data, _ := os.ReadFile(save); string(data) != "saved" {
		t.Fatal("save overwritten")
	}
	if _, err = os.Stat(filepath.Join(sdk, "Disk", "Data")); !os.IsNotExist(err) {
		t.Fatal("installed SDK mutated")
	}
	unmarked := filepath.Join(base, "test-data")
	os.Mkdir(unmarked, 0700)
	if _, err = PrepareSession(unmarked, sdk, cart); err == nil {
		t.Fatal("adopted unmarked directory")
	}
	link := filepath.Join(base, "alias")
	os.Symlink(root, link)
	if _, err = PrepareSession(link, sdk, cart); err == nil {
		t.Fatal("accepted root symlink")
	}
	os.Symlink(sdk, filepath.Join(p.Data, "escape"))
	if _, err = PrepareSession(root, sdk, cart); err == nil {
		t.Fatal("accepted writable-tree symlink")
	}
}
