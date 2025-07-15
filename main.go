package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xflag"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/swaggo/swag"
)

func main() {
	xos.AppName = "Swagger Doc"
	xos.AppVersion = "2.4.0"
	xos.CopyrightStartYear = "2019"
	xos.CopyrightHolder = "Richard A. Wilkes"

	searchDir := "."
	searchDesc := "The `dir`ectory to search for documentation directives"
	flag.StringVar(&searchDir, "search", searchDir, searchDesc)
	flag.StringVar(&searchDir, "s", searchDir, searchDesc)

	mainAPIFile := "main.go"
	mainAPIFileDesc := "The Go `file` to search for the main documentation directives"
	flag.StringVar(&mainAPIFile, "main", mainAPIFile, mainAPIFileDesc)
	flag.StringVar(&mainAPIFile, "m", mainAPIFile, mainAPIFileDesc)

	destDir := "docs"
	destDirDesc := "The destination `dir`ectory to write the documentation files to"
	flag.StringVar(&destDir, "output", destDir, destDirDesc)
	flag.StringVar(&destDir, "o", destDir, destDirDesc)

	apiDir := "api"
	apiDirDesc := "The intermediate `dir`ectory within the output directory to write the files to"
	flag.StringVar(&apiDir, "api", apiDir, apiDirDesc)
	flag.StringVar(&apiDir, "a", apiDir, apiDirDesc)

	baseName := "swagger"
	baseNameDesc := "The base `name` to use for the definition files"
	flag.StringVar(&baseName, "name", baseName, baseNameDesc)
	flag.StringVar(&baseName, "n", baseName, baseNameDesc)

	maxDependencyDepth := 2
	maxDependencyDepthDesc := "The maximum depth to resolve dependencies; use 0 for unlimited (only used if -old-method is set)"
	flag.IntVar(&maxDependencyDepth, "depth", maxDependencyDepth, maxDependencyDepthDesc)
	flag.IntVar(&maxDependencyDepth, "d", maxDependencyDepth, maxDependencyDepthDesc)

	markdownFileDir := ""
	markdownFileDirDesc := "The `dir`ectory to search for markdown includes"
	flag.StringVar(&markdownFileDir, "mdincludes", markdownFileDir, markdownFileDirDesc)
	flag.StringVar(&markdownFileDir, "i", markdownFileDir, markdownFileDirDesc)

	title := ""
	titleDesc := "The title for the HTML page. If unset, defaults to the base name"
	flag.StringVar(&title, "title", title, titleDesc)
	flag.StringVar(&title, "t", title, titleDesc)

	serverURL := ""
	serverURLDesc := "An additional server URL"
	flag.StringVar(&serverURL, "url", serverURL, serverURLDesc)
	flag.StringVar(&serverURL, "u", serverURL, serverURLDesc)

	embedded := false
	embeddedDesc := "When set, embeds the spec directly in the html"
	flag.BoolVar(&embedded, "embedded", embedded, embeddedDesc)
	flag.BoolVar(&embedded, "e", embedded, embeddedDesc)

	useOldMethod := flag.Bool("old-method", false, "Use old method for parsing dependencies")

	var exclude []string
	excludeDesc := "Exclude directories and files when searching. Example for multiple: -x file1 -x file2"
	excludeFunc := func(in string) error {
		exclude = append(exclude, in)
		return nil
	}
	flag.Func("exclude", excludeDesc, excludeFunc)
	flag.Func("x", excludeDesc, excludeFunc)

	xflag.AddVersionFlags()
	xflag.SetUsage(nil, "", "")
	xflag.Parse()
	if title == "" {
		title = baseName
	}
	xos.ExitIfErr(generate(searchDir, mainAPIFile, destDir, apiDir, baseName, title, serverURL, markdownFileDir,
		exclude, maxDependencyDepth, embedded, *useOldMethod))
	xos.Exit(0)
}

func generate(searchDir, mainAPIFile, destDir, apiDir, baseName, title, serverURL, markdownFileDir string, exclude []string, maxDependencyDepth int, embedded, useOldMethod bool) error {
	if err := os.MkdirAll(filepath.Join(destDir, apiDir), 0o755); err != nil { //nolint:gosec // Yes, I want these permissions
		return errs.Wrap(err)
	}

	opts := make([]func(*swag.Parser), 0)
	if len(exclude) != 0 {
		opts = append(opts, swag.SetExcludedDirsAndFiles(strings.Join(exclude, ",")))
	}
	if markdownFileDir != "" {
		opts = append(opts, swag.SetMarkdownFileDirectory(markdownFileDir))
	}
	opts = append(opts,
		swag.ParseUsingGoList(!useOldMethod),
		swag.SetDebugger(&filter{out: log.New(os.Stdout, "", log.LstdFlags)}),
	)

	parser := swag.New(opts...)

	parser.ParseDependency = swag.ParseModels
	parser.ParseInternal = true
	if err := parser.ParseAPI(searchDir, mainAPIFile, maxDependencyDepth); err != nil {
		return errs.Wrap(err)
	}
	jData, err := json.MarshalIndent(parser.GetSwagger(), "", "  ")
	if err != nil {
		return errs.Wrap(err)
	}
	if err = os.WriteFile(filepath.Join(destDir, apiDir, baseName+".json"), jData, 0o644); err != nil { //nolint:gosec // Yes, I want these permissions
		return errs.Wrap(err)
	}
	var specURL, extra, js string
	if serverURL != "" {
		extra = fmt.Sprintf(`
          server-url="%s"`, serverURL)
	}
	if embedded {
		js = fmt.Sprintf(`
<script>
    window.addEventListener("DOMContentLoaded", (event) => {
        const rapidocEl = document.getElementById("rapidoc");
        rapidocEl.loadSpec(%s)
    })
</script>`, string(jData))
	} else {
		specURL = fmt.Sprintf(`
          spec-url="%s.json"`, baseName)
	}
	//nolint:gosec // Yes, I want these permissions
	if err = os.WriteFile(filepath.Join(destDir, apiDir, "index.html"), []byte(fmt.Sprintf(`<!doctype html>
<html>
<head>
    <meta charset="utf-8">
	<title>%s</title>
	<script src="https://cdnjs.cloudflare.com/ajax/libs/rapidoc/9.3.8/rapidoc-min.js"
			integrity="sha512-0ES6eX4K9J1PrIEjIizv79dTlN5HwI2GW9Ku6ymb8dijMHF5CIplkS8N0iFJ/wl3GybCSqBJu8HDhiFkZRAf0g=="
			crossorigin="anonymous"
			referrerpolicy="no-referrer">
	</script>
</head>
<body>
<rapi-doc id="rapidoc"
          theme="dark"
          render-style="view"
          schema-style="table"
          schema-description-expanded="true"%s
          allow-spec-file-download="true"%s
>
</rapi-doc>%s
</body>
</html>`, title, specURL, extra, js)), 0o644); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

type filter struct {
	out *log.Logger
}

func (f *filter) Printf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	if strings.Contains(s, "warning: failed to evaluate const mProfCycleWrap") {
		return
	}
	f.out.Println(s)
}
