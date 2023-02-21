package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/richardwilkes/toolbox/atexit"
	"github.com/richardwilkes/toolbox/cmdline"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/swaggo/swag"
)

const apiDir = "api"

func main() {
	cmdline.AppName = "Swagger Doc"
	cmdline.AppVersion = "2.0.2"
	cmdline.CopyrightStartYear = "2019"
	cmdline.CopyrightHolder = "Richard A. Wilkes"

	cl := cmdline.New(true)
	searchDir := "."
	mainAPIFile := "main.go"
	destDir := "docs"
	baseName := "swagger"
	maxDependencyDepth := 2
	markdownFileDir := ""
	title := ""
	serverURL := ""
	embedded := false
	exclude := ""
	cl.NewGeneralOption(&searchDir).SetSingle('s').SetName("search").SetArg("dir").SetUsage("The directory root to search for documentation directives")
	cl.NewGeneralOption(&mainAPIFile).SetSingle('m').SetName("main").SetArg("file").SetUsage("The Go file to search for the main documentation directives")
	cl.NewGeneralOption(&destDir).SetSingle('o').SetName("output").SetArg("dir").SetUsage("The destination directory to write the documentation files to")
	cl.NewGeneralOption(&baseName).SetSingle('n').SetName("name").SetArg("name").SetUsage("The base name to use for the definition files")
	cl.NewGeneralOption(&maxDependencyDepth).SetSingle('d').SetName("depth").SetUsage("The maximum depth to resolve dependencies; use 0 for unlimited")
	cl.NewGeneralOption(&markdownFileDir).SetSingle('i').SetName("mdincludes").SetArg("dir").SetUsage("The directory root to search for markdown includes")
	cl.NewGeneralOption(&title).SetSingle('t').SetName("title").SetArg("text").SetUsage("The title for the HTML page. If unset, defaults to the base name")
	cl.NewGeneralOption(&serverURL).SetSingle('u').SetName("url").SetArg("url").SetUsage("An additional server URL")
	cl.NewGeneralOption(&embedded).SetSingle('e').SetName("embedded").SetUsage("When set, embeds the spec directly in the html")
	cl.NewGeneralOption(&exclude).SetSingle('x').SetName("exclude").SetUsage("Exclude directories and files when searching, comma separated")
	cl.Parse(os.Args[1:])
	if title == "" {
		title = baseName
	}
	jot.FatalIfErr(generate(searchDir, mainAPIFile, destDir, baseName, title, serverURL, markdownFileDir, exclude, maxDependencyDepth, embedded))
	atexit.Exit(0)
}

func generate(searchDir, mainAPIFile, destDir, baseName, title, serverURL, markdownFileDir, exclude string, maxDependencyDepth int, embedded bool) error {
	if err := os.MkdirAll(filepath.Join(destDir, apiDir), 0o755); err != nil { //nolint:gosec // Yes, I want these permissions
		return errs.Wrap(err)
	}

	opts := make([]func(*swag.Parser), 0)
	if exclude != "" {
		opts = append(opts, swag.SetExcludedDirsAndFiles(exclude))
	}
	if markdownFileDir != "" {
		opts = append(opts, swag.SetMarkdownFileDirectory(markdownFileDir))
	}

	parser := swag.New(opts...)

	parser.ParseDependency = true
	parser.ParseInternal = true
	if err := parser.ParseAPI(searchDir, mainAPIFile, maxDependencyDepth); err != nil {
		return errs.Wrap(err)
	}
	jData, err := json.MarshalIndent(parser.GetSwagger(), "", "  ")
	if err != nil {
		return errs.Wrap(err)
	}
	if err = ioutil.WriteFile(filepath.Join(destDir, apiDir, baseName+".json"), jData, 0o644); err != nil { //nolint:gosec // Yes, I want these permissions
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
	if err = ioutil.WriteFile(filepath.Join(destDir, apiDir, "index.html"), []byte(fmt.Sprintf(`<!doctype html>
<html>
<head>
    <meta charset="utf-8">
	<title>%s</title>
    <script type="module" src="https://unpkg.com/rapidoc/dist/rapidoc-min.js"></script>
</head>
<body>
<rapi-doc id="rapidoc"
          theme="dark"
          render-style="view"
          schema-style="table"
          schema-description-expanded="true"%s
          allow-spec-file-download="true"%s
></rapi-doc>%s
</body>
</html>`, title, specURL, extra, js)), 0o644); err != nil {
		return errs.Wrap(err)
	}
	return nil
}
