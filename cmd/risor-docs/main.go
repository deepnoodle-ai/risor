package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func genMeta(names []string) []byte {
	meta := map[string]interface{}{}
	for _, name := range names {
		meta[name] = map[string]interface{}{
			"title": name,
		}
	}
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		fmt.Printf("error marshaling meta: %s\n", err)
		os.Exit(1)
	}
	return metaJSON
}

// modulesDir is where module sources live, relative to the repository root.
const modulesDir = "pkg/modules"

func listModules() ([]string, error) {
	mods, err := os.ReadDir(modulesDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, mod := range mods {
		if mod.IsDir() {
			name := mod.Name()
			if name == "all" {
				continue
			}
			names = append(names, filepath.Join(modulesDir, name))
		}
	}
	return names, nil
}

func main() {
	var siteRepoPath string
	flag.StringVar(&siteRepoPath, "site-repo", "../risor-site", "path to the risor-site repository")
	flag.Parse()

	modPaths, err := listModules()
	if err != nil {
		fmt.Printf("error listing modules: %s\n", err)
		os.Exit(1)
	}

	var modNames []string

	for _, modPath := range modPaths {
		name := filepath.Base(modPath)

		mdPath := filepath.Join(modPath, fmt.Sprintf("%s.md", name))
		doc, err := os.ReadFile(mdPath)
		if err != nil {
			fmt.Printf("%s: skipped due to no markdown file\n", modPath)
			continue
		}
		modNames = append(modNames, name)
		fmt.Println(modPath)
		text := string(doc)

		dstPath := filepath.Join(siteRepoPath, "pages", "docs", "modules",
			fmt.Sprintf("%s.mdx", name))

		err = os.WriteFile(dstPath, []byte(text), 0o644)
		if err != nil {
			fmt.Printf("error writing %s: %s\n", dstPath, err)
			os.Exit(1)
		}
	}

	metaJSON := genMeta(modNames)
	dstPath := filepath.Join(siteRepoPath, "pages", "docs", "modules", "_meta.json")
	err = os.WriteFile(dstPath, metaJSON, 0o644)
	if err != nil {
		fmt.Printf("error writing %s: %s\n", dstPath, err)
		os.Exit(1)
	}
}
