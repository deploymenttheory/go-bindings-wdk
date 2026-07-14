// Command generate drives the go-bindings-wdk pipeline. The WDK metadata
// (Windows.Wdk.winmd) references types in Windows.Win32.*, so ingest runs
// over BOTH committed winmds: the win32 IR (namespaces prefixed "Win32.") is
// loaded for type resolution only and its packages are imported from the
// published go-bindings-win32 module, never emitted here.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	rawwin "github.com/deploymenttheory/go-bindings-wdk/internal/codegen/emit/raw"
	"github.com/deploymenttheory/go-bindings-wdk/internal/codegen/pipeline"
	"github.com/deploymenttheory/go-bindings-wdk/internal/diagnostics"
	"github.com/deploymenttheory/go-bindings-wdk/internal/win32meta"
	"github.com/deploymenttheory/go-bindings-wdk/internal/win32meta/ingest"
	"github.com/deploymenttheory/go-winmd"
	"github.com/deploymenttheory/go-winmd/nuget"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "fetch-metadata":
		err = runFetchMetadata(os.Args[2:])
	case "ingest":
		err = runIngest(os.Args[2:])
	case "bindings":
		err = runBindings(os.Args[2:])
	case "abitest":
		err = runABITest(os.Args[2:])
	case "validate":
		err = runValidate(os.Args[2:])
	case "diff":
		err = runDiff(os.Args[2:])
	case "list":
		err = runList(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: generate <command> [flags]

commands:
  fetch-metadata  download the latest winmd from NuGet into metadata/winmd
  ingest          project the winmd into per-namespace .w32meta.json files
  bindings        emit the Go bindings from the .w32meta.json metadata (self-cleaning)
  abitest         generate the ABI layout acceptance test
  validate        structural integrity checks over the metadata
  diff            semantic API diff between two metadata trees
  list            list the namespaces in the winmd`)
}

// winmdVersion reads the winmd's version from the PROVENANCE.json record
// whose File matches the winmd's base name.
func winmdVersion(winmdPath string) string {
	records, err := nuget.ReadProvenance(filepath.Join(filepath.Dir(winmdPath), "PROVENANCE.json"))
	if err != nil || len(records) == 0 {
		return ""
	}
	base := filepath.Base(winmdPath)
	for _, record := range records {
		if record.File == base {
			return record.Version
		}
	}
	return records[0].Version
}

func runIngest(args []string) error {
	flags := flag.NewFlagSet("ingest", flag.ExitOnError)
	winmdPath := flags.String("winmd", filepath.Join("metadata", "winmd", "Windows.Wdk.winmd"), "path to Windows.Wdk.winmd")
	win32WinmdPath := flags.String("win32-winmd", filepath.Join("metadata", "winmd", "Windows.Win32.winmd"), "path to Windows.Win32.winmd (type resolution)")
	outDir := flags.String("out", filepath.Join("metadata", "wdk"), "output directory for Wdk .w32meta.json files")
	win32OutDir := flags.String("win32-out", filepath.Join("metadata", "win32"), "output directory for external Win32. IR")
	namespaceFilter := flags.String("namespace", "", "comma-separated Wdk namespace filter; empty = all")
	verbose := flags.Bool("v", false, "print diagnostics")
	if err := flags.Parse(args); err != nil {
		return err
	}

	// Run 1: the full win32 assembly as external "Win32."-prefixed IR. Its
	// kind index feeds the wdk run's cross-assembly TypeRef targeting.
	win32File, err := winmd.Open(*win32WinmdPath)
	if err != nil {
		return err
	}
	win32Ingester := ingest.NewWithOptions(win32File, winmdVersion(*win32WinmdPath), win32IngestOptions())
	win32Namespaces, err := win32Ingester.Ingest()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*win32OutDir, 0o755); err != nil {
		return err
	}
	for _, meta := range win32Namespaces {
		if err := win32meta.Write(*win32OutDir, meta); err != nil {
			return err
		}
	}

	// Run 2: the Wdk assembly (the tree this repo emits).
	file, err := winmd.Open(*winmdPath)
	if err != nil {
		return err
	}
	ingester := ingest.NewWithOptions(file, winmdVersion(*winmdPath), wdkIngestOptions(win32Ingester.KindIndex()))
	namespaces, err := ingester.Ingest()
	if err != nil {
		return err
	}

	filter := map[string]bool{}
	for _, name := range strings.Split(*namespaceFilter, ",") {
		if name = strings.TrimSpace(name); name != "" {
			filter[name] = true
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}
	written := 0
	for _, meta := range namespaces {
		if len(filter) > 0 && !filter[meta.Namespace] {
			continue
		}
		if err := win32meta.Write(*outDir, meta); err != nil {
			return err
		}
		written++
	}
	if *verbose {
		for _, diagnostic := range ingester.Diagnostics {
			fmt.Fprintln(os.Stderr, "diagnostic:", diagnostic)
		}
	}
	fmt.Printf("ingested %d Wdk namespaces → %s (+%d external Win32. namespaces → %s; %d diagnostics)\n",
		written, *outDir, len(win32Namespaces), *win32OutDir, len(ingester.Diagnostics))
	return nil
}

// runBindings emits the single idiomatic-shaped bindings tree (bindings/win32)
// from the committed metadata. It is self-cleaning — files from an earlier run
// that this run does not rewrite are pruned.
func runBindings(args []string) error {
	flags := flag.NewFlagSet("bindings", flag.ExitOnError)
	metadataDir := flags.String("metadata", filepath.Join("metadata", "wdk"), "directory of Wdk .w32meta.json files")
	win32MetadataDir := flags.String("win32-metadata", filepath.Join("metadata", "win32"), "directory of external Win32. IR")
	outDir := flags.String("out", filepath.Join("bindings", "wdk"), "bindings output root")
	namespaceFilter := flags.String("namespace", "", "comma-separated namespace filter; empty = all loaded")
	verbose := flags.Bool("v", false, "print diagnostics")
	writeBaseline := flags.String("diagnostics", "", "write the diagnostics baseline to this path")
	checkBaseline := flags.String("diagnostics-baseline", "", "fail if any diagnostic is not in this committed baseline")
	if err := flags.Parse(args); err != nil {
		return err
	}

	registry, err := pipeline.LoadAll(*metadataDir, *win32MetadataDir)
	if err != nil {
		return err
	}
	filter := map[string]bool{}
	for _, name := range strings.Split(*namespaceFilter, ",") {
		if name = strings.TrimSpace(name); name != "" {
			filter[name] = true
		}
	}

	generator := rawwin.New(registry, modulePath, *outDir)
	written, err := generator.EmitAll(filter)
	if err != nil {
		return err
	}
	diags := generator.Diagnostics
	if *verbose {
		for _, diagnostic := range diags {
			fmt.Fprintln(os.Stderr, "diagnostic:", diagnostic)
		}
	}
	fmt.Printf("emitted %d packages → %s (%d diagnostics)\n", written, *outDir, len(diags))

	if *writeBaseline != "" {
		if err := diagnostics.WriteBaseline(*writeBaseline, diags); err != nil {
			return err
		}
		fmt.Printf("wrote diagnostics baseline → %s\n", *writeBaseline)
	}
	if *checkBaseline != "" {
		newEntries, err := diagnostics.CheckBaseline(*checkBaseline, diags)
		if err != nil {
			return err
		}
		if len(newEntries) > 0 {
			for _, entry := range newEntries {
				fmt.Fprintln(os.Stderr, "new diagnostic:", entry)
			}
			return fmt.Errorf("%d diagnostics beyond baseline %s (fix them, or rewrite the baseline with --diagnostics after review)",
				len(newEntries), *checkBaseline)
		}
		fmt.Println("diagnostics within baseline")
	}
	return nil
}

// modulePath is this module's import path root.
const modulePath = "github.com/deploymenttheory/go-bindings-wdk"

// wdkApiName maps full CLR namespaces to IR Api keys for both assemblies:
// local Windows.Wdk.X → X; external Windows.Win32.X → Win32.X; else "".
func wdkApiName(fullNamespace string) string {
	switch {
	case strings.HasPrefix(fullNamespace, "Windows.Wdk."):
		return strings.TrimPrefix(fullNamespace, "Windows.Wdk.")
	case strings.HasPrefix(fullNamespace, "Windows.Win32."):
		return "Win32." + strings.TrimPrefix(fullNamespace, "Windows.Win32.")
	default:
		return ""
	}
}

// wdkIngestOptions projects the Windows.Wdk assembly; extraKinds carries the
// win32 run's TypeDef classification for cross-assembly refs.
func wdkIngestOptions(extraKinds map[string]string) ingest.Options {
	return ingest.Options{
		ProjectPrefix:      "Windows.Wdk.",
		ApiName:            wdkApiName,
		ExcludedNamespaces: []string{"Windows.Wdk.Foundation.Metadata"},
		ExtraKinds:         extraKinds,
	}
}

// win32IngestOptions projects the win32 assembly into "Win32."-prefixed
// external IR (resolution only, never emitted).
func win32IngestOptions() ingest.Options {
	return ingest.Options{
		ProjectPrefix:      "Windows.Win32.",
		ApiName:            wdkApiName,
		ExcludedNamespaces: []string{"Windows.Win32.Foundation.Metadata", "Windows.Win32.Interop"},
	}
}

// runABITest regenerates all bindings (collecting expected struct layouts)
// and writes the sampled ABI acceptance test.
func runABITest(args []string) error {
	flags := flag.NewFlagSet("abitest", flag.ExitOnError)
	metadataDir := flags.String("metadata", filepath.Join("metadata", "wdk"), "directory of Wdk .w32meta.json files")
	win32MetadataDir := flags.String("win32-metadata", filepath.Join("metadata", "win32"), "directory of external Win32. IR")
	outDir := flags.String("out", filepath.Join("bindings", "wdk"), "bindings output root")
	testPath := flags.String("test-out", filepath.Join("acceptance", "abi_generated_test.go"), "generated test path")
	sample := flags.Int("sample", 400, "approximate number of sampled structs (Foundation always included)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	registry, err := pipeline.LoadAll(*metadataDir, *win32MetadataDir)
	if err != nil {
		return err
	}
	generator := rawwin.New(registry, modulePath, *outDir)
	if _, err := generator.EmitAll(nil); err != nil {
		return err
	}
	source := rawwin.BuildABITest(generator.ABIRecords(), generator.ImportPathFor, *sample)
	if err := os.WriteFile(*testPath, []byte(source), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d structs recorded)\n", *testPath, len(generator.ABIRecords()))
	return nil
}

func runList(args []string) error {
	flags := flag.NewFlagSet("list", flag.ExitOnError)
	winmdPath := flags.String("winmd", filepath.Join("metadata", "winmd", "Windows.Wdk.winmd"), "path to Windows.Wdk.winmd")
	if err := flags.Parse(args); err != nil {
		return err
	}
	file, err := winmd.Open(*winmdPath)
	if err != nil {
		return err
	}
	ingester := ingest.NewWithOptions(file, "", wdkIngestOptions(nil))
	namespaces, err := ingester.Ingest()
	if err != nil {
		return err
	}
	for _, meta := range namespaces {
		fmt.Printf("%-60s %5d funcs %5d structs %5d enums %5d ifaces %6d consts\n",
			meta.Namespace, len(meta.Functions), len(meta.Structs), len(meta.Enums),
			len(meta.Interfaces), len(meta.Constants))
	}
	return nil
}
