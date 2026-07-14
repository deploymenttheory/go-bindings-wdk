package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deploymenttheory/go-winmd/nuget"
)

const (
	wdkPackage        = "microsoft.windows.wdk.win32metadata"
	wdkPackageDisplay = "Microsoft.Windows.WDK.Win32Metadata"
	wdkWinmdFileName  = "Windows.Wdk.winmd"

	win32Package        = "microsoft.windows.sdk.win32metadata"
	win32PackageDisplay = "Microsoft.Windows.SDK.Win32Metadata"
	win32WinmdFileName  = "Windows.Win32.winmd"
)

// runFetchMetadata downloads BOTH winmds this generator needs: the WDK
// metadata (the assembly this repo emits) and the win32 metadata it
// references for type resolution. The win32 version is pinned — it must
// match the winmd version of the go-bindings-win32 release in go.mod, so
// cross-assembly type references resolve against the same API surface.
func runFetchMetadata(args []string) error {
	flags := flag.NewFlagSet("fetch-metadata", flag.ExitOnError)
	version := flags.String("version", "", "WDK package version (empty = latest published)")
	win32Version := flags.String("win32-version", "", "pinned win32metadata version (empty = keep the committed pin)")
	outDir := flags.String("out", filepath.Join("metadata", "winmd"), "output directory")
	force := flags.Bool("force", false, "re-download even when the versions match")
	if err := flags.Parse(args); err != nil {
		return err
	}

	client := nuget.NewClient()
	provenancePath := filepath.Join(*outDir, "PROVENANCE.json")
	committed, _ := nuget.ReadProvenance(provenancePath)
	recordFor := func(file string) *nuget.Provenance {
		for i := range committed {
			if committed[i].File == file {
				return &committed[i]
			}
		}
		return nil
	}

	wdkTarget := *version
	if wdkTarget == "" {
		latest, err := nuget.LatestVersion(client, wdkPackage)
		if err != nil {
			return err
		}
		wdkTarget = latest
	}
	win32Target := *win32Version
	if win32Target == "" {
		if pin := recordFor(win32WinmdFileName); pin != nil {
			win32Target = pin.Version
		} else {
			return fmt.Errorf("no committed win32 pin; pass --win32-version matching the go-bindings-win32 release in go.mod")
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	var records []nuget.Provenance
	changed := false
	for _, want := range []struct {
		pkg, display, file, version string
	}{
		{wdkPackage, wdkPackageDisplay, wdkWinmdFileName, wdkTarget},
		{win32Package, win32PackageDisplay, win32WinmdFileName, win32Target},
	} {
		path := filepath.Join(*outDir, want.file)
		current := recordFor(want.file)
		if !*force && current != nil && current.Version == want.version {
			if _, err := os.Stat(path); err == nil {
				fmt.Printf("up-to-date %s %s\n", want.file, want.version)
				records = append(records, *current)
				continue
			}
		}
		fmt.Printf("downloading %s\n", nuget.SourceURL(want.pkg, want.version))
		content, record, err := nuget.Fetch(client, want.pkg, want.display, want.version, want.file)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
		fmt.Printf("updated %s -> %s (%d bytes)\n", want.file, want.version, len(content))
		records = append(records, record)
		changed = true
	}
	if changed || len(committed) != len(records) {
		return nuget.WriteProvenance(provenancePath, records)
	}
	return nil
}
