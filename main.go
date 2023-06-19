package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

var (
	inputDir       string
	outputDir      string
	projects       map[string][]string
	signatureFiles []string
	debug          bool           = false
	logger         zerolog.Logger = zerolog.New(zerolog.MultiLevelWriter(zerolog.NewConsoleWriter())).With().Timestamp().Logger()
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
		logger.Fatal().Msgf("error running program: %s", err.Error())
	}
}

func flags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:        "debug",
			Usage:       "set debug logging",
			Destination: &debug,
		},
		&cli.StringFlag{
			Name:        "input-directory",
			Aliases:     []string{"i"},
			Usage:       "read PDF files from `INPUT` directory",
			Required:    false,
			Destination: &inputDir,
		},
		&cli.StringFlag{
			Name:        "output-directory",
			Aliases:     []string{"o"},
			Usage:       "write merged PDF files to `OUTPUT` directory",
			Required:    false,
			Destination: &outputDir,
		},
	}
}

func checkAndSetSignatureFiles(argsLine string) (string, error) {

	reg, err := regexp.Compile(`{[.a-z]+}`)
	if err != nil {
		return argsLine, err
	}

	foundStrings := reg.FindAllString(argsLine, -1)
	logger.Debug().Msgf("found string: %v\n", foundStrings)

	for _, v := range foundStrings {

		argsLine = strings.Replace(argsLine, v, "", -1)

		cleanedFoundString := strings.TrimSpace(strings.Trim(v, `{}`))
		logger.Debug().Msgf("cleaned found string: %v\n", cleanedFoundString)

		signatureFiles = append(signatureFiles, cleanedFoundString)
	}

	return argsLine, nil
}

func checkAndSetAlternateDirectories(args []string) error {
	if inputDir != "" && outputDir != "" {
		return nil
	}

	if (inputDir != "" && outputDir == "") || (inputDir == "" && outputDir != "") {
		return errors.New("must use both -i and -o or neither")
	}

	//do some parsing, input and output dir separated by ::
	line := strings.Join(args, " ")
	logger.Debug().Msgf("joined line: %v\n", line)

	sigLine, err := checkAndSetSignatureFiles(line)
	if err != nil {
		return err
	}
	logger.Debug().Msgf("returned line after parsing signature file: %v\n", sigLine)

	splitLine := strings.Split(sigLine, "::")

	if len(splitLine) != 2 {
		return fmt.Errorf("split line does not end up with two directories: %v", splitLine)
	}

	inputDir = strings.TrimSpace(splitLine[0])
	outputDir = strings.TrimSpace(splitLine[1])

	return nil
}

func run(c *cli.Context) error {
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if err := checkAndSetAlternateDirectories(c.Args().Slice()); err != nil {
		return err
	}

	logger.Debug().Msgf(`
    input dir: %v
    output dir: %v
    signature file: %v
	`, inputDir, outputDir, signatureFiles)

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

	f, err := os.Create(filepath.Join(outputDir, "log.txt"))
	if err != nil {
		return err
	}
	defer f.Close()
	logger = zerolog.New(zerolog.MultiLevelWriter(
		zerolog.NewConsoleWriter(),
		zerolog.ConsoleWriter{Out: f, NoColor: true})).With().Timestamp().Logger()

	projects = make(map[string][]string)
	err = filepath.Walk(inputDir, walkFunc)
	if err != nil {
		logger.Fatal().Msgf("error scanning files: %s", err.Error())
	}

	sortedProjectNames := sortProjects(projects)

	for _, pName := range sortedProjectNames {
		err = mergePDF(pName, projects[pName])
		if err != nil {
			logger.Warn().Msgf("error merging PDFs: %s", err.Error())
		}
	}

	return nil
}

func sortProjects(projects map[string][]string) []string {
	projectNames := []string{}
	for p := range projects {
		projectNames = append(projectNames, p)
	}
	sort.Strings(projectNames)
	return projectNames
}

func walkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() && info.Name() != ".." && info.Name() != filepath.Base(inputDir) {
		logger.Info().Msgf("skipping directory: %s", info.Name())
		return filepath.SkipDir
	}

	if info.Name() == ".." {
		return nil
	}

	_, file := filepath.Split(path)

	if filepath.Ext(file) != ".pdf" {
		logger.Info().Msgf("skipping non-pdf file: %s\n", path)
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

	slice := strings.Split(cleanPath, "-")

	if len(slice) < 2 {
		return cleanPath
	}

	//joined := strings.Join(slice[0:2], "_")
	return slice[0]
}

func mergePDF(project string, projectFiles []string) error {

	sort.Slice(projectFiles, func(i, j int) bool {
		replacedI := strings.ReplaceAll(projectFiles[i], "-", "")
		replacedJ := strings.ReplaceAll(projectFiles[j], "-", "")

		return replacedI < replacedJ
	})

	// if signatureFile != "" {
	// 	projectFiles = append(projectFiles, signatureFile)
	// }

	logger.Info().Msgf("order of merging into project %s:", project)
	for _, file := range projectFiles {
		logger.Info().Msgf(file)
	}

	outputFile := filepath.Join(outputDir, project+".pdf")

	mergeConf := model.NewDefaultConfiguration()
	mergeConf.ValidationMode = model.ValidationNone
	err := api.MergeCreateFile(projectFiles, outputFile, mergeConf)
	if err != nil {
		return err
	}
	if err := api.ValidateFile(outputFile, mergeConf); err != nil {
		return err
	}
	logger.Info().Msgf("successfully validated file: %s", outputFile)
	return nil
}
