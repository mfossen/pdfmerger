package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/urfave/cli/v2"
)

var (
	inputDir  string
	outputDir string
	projects  map[string][]string
)

func main() {

	app := &cli.App{
		Name:   "pdfmerger",
		Usage:  "takes a directory of PDF files and merges them by project",
		Flags:  flags(),
		Action: run,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatalf("error running program: %s", err.Error())
	}
}

func flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "input-directory",
			Aliases:     []string{"i"},
			Usage:       "read PDF files from `INPUT` directory",
			Required:    true,
			Destination: &inputDir,
		},
		&cli.StringFlag{
			Name:        "output-directory",
			Aliases:     []string{"o"},
			Usage:       "write merged PDF files to `OUTPUT` directory",
			Required:    true,
			Destination: &outputDir,
		},
	}
}

func run(c *cli.Context) error {
	// create output directory if it doesn't exist
	if _, err := os.Stat(outputDir); err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(outputDir, 0755)
			if err != nil {
				return fmt.Errorf("unable to create output directory: %s", err.Error())
			}
		} else {
			return fmt.Errorf("unexpected error handling output directory: %s", err.Error())
		}
	}

	projects = make(map[string][]string)
	err := filepath.Walk(inputDir, walkFunc)
	if err != nil {
		log.Fatalf("error scanning files: %s\n", err.Error())
	}

	for project, files := range projects {
		err = mergePDF(project, files)
		if err != nil {
			log.Fatalf("error merging PDFs: %s", err.Error())
		}
	}

	return nil
}

func walkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() && info.Name() != ".." && info.Name() != filepath.Base(inputDir) {
		log.Printf("skipping directory: %s\n", info.Name())
		return filepath.SkipDir
	}

	if info.Name() == ".." {
		return nil
	}

	_, file := filepath.Split(path)

	if filepath.Ext(file) != ".pdf" {
		log.Printf("skipping non-pdf file: %s\n", path)
		return nil
	}

	// if we're here, have some file.pdf to work with
	projectName := parseProjectName(file)

	projects[projectName] = append(projects[projectName], path)

	return nil
}

// split pdf name by underscare and take index 1 as the project name
func parseProjectName(file string) string {

	cleanPath := strings.TrimSuffix(file, filepath.Ext(file))

	slice := strings.Split(cleanPath, "_")

	if len(slice) < 2 {
		return cleanPath
	}

	joined := strings.Join(slice[0:2], "_")
	return joined
}

func mergePDF(project string, projectFiles []string) error {

	sort.Slice(projectFiles, func(i, j int) bool {
		replacedI := strings.ReplaceAll(projectFiles[i], "-", "")
		replacedJ := strings.ReplaceAll(projectFiles[j], "-", "")

		return replacedI < replacedJ
	})

	fmt.Printf("order of merging into project %s:\n", project)
	for _, file := range projectFiles {
		log.Println(file)
	}

	outputFile := filepath.Join(outputDir, project+".pdf")

	return api.MergeCreateFile(projectFiles, outputFile, nil)
}
