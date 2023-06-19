package main

import (
	"errors"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
	"golang.org/x/exp/slices"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	inputDir       string
	outputDir      string
	projects       map[string][]string
	signatureFiles map[string][]string
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

func parseSignatureFiles() error {
	signatureFiles = make(map[string][]string)
	err := filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if strings.Contains(d.Name(), "signature") {
			basename := strings.Replace(strings.Replace(d.Name(), filepath.Ext(d.Name()), "", -1), "signature-", "", -1)
			logger.Debug().Msgf("signature file basename: %v\n", basename)
			suffix := strings.Split(basename, ".")[0]
			signatureFiles[suffix] = append(signatureFiles[suffix], path)
			logger.Debug().Msgf("adding file %v to suffix %v\n", path, suffix)
		}
		return err
	})
	return err
}

// func checkAndSetSignatureFiles(argsLine string) (string, error) {
//
// 	reg, err := regexp.Compile(`{[.a-z]+}`)
// 	if err != nil {
// 		return argsLine, err
// 	}
//
// 	foundStrings := reg.FindAllString(argsLine, -1)
// 	logger.Debug().Msgf("found string: %v\n", foundStrings)
//
// 	for _, v := range foundStrings {
//
// 		argsLine = strings.Replace(argsLine, v, "", -1)
//
// 		cleanedFoundString := strings.TrimSpace(strings.Trim(v, `{}`))
// 		logger.Debug().Msgf("cleaned found string: %v\n", cleanedFoundString)
//
// 		signatureFiles = append(signatureFiles, cleanedFoundString)
// 	}
//
// 	return argsLine, nil
// }

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

	// sigLine, err := checkAndSetSignatureFiles(line)
	// if err != nil {
	// 	return err
	// }
	// logger.Debug().Msgf("returned line after parsing signature file: %v\n", sigLine)

	// splitLine := strings.Split(sigLine, "::")
	splitLine := strings.Split(line, "::")

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

	if err := parseSignatureFiles(); err != nil {
		return err
	}

	// return nil

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
	sigAddedProjectFiles := addSigFiles(projectFiles)
	for _, file := range sigAddedProjectFiles {
		logger.Info().Msgf(file)
	}

	outputFile := filepath.Join(outputDir, project+".pdf")

	mergeConf := model.NewDefaultConfiguration()
	mergeConf.ValidationMode = model.ValidationNone
	err := api.MergeCreateFile(sigAddedProjectFiles, outputFile, mergeConf)
	if err != nil {
		return err
	}
	if err := api.ValidateFile(outputFile, mergeConf); err != nil {
		return err
	}
	logger.Info().Msgf("successfully validated file: %s", outputFile)
	return nil
}

func addSigFiles(projectFiles []string) []string {
	tempFiles := []string{}
	tempFiles = append(tempFiles, projectFiles...)
	for i, name := range tempFiles {
		if strings.Contains(name, "signature") {
			logger.Debug().Msgf("skipping signature file: %v\n", name)
			continue
		}
		split := strings.Split(name, "-")
		logger.Debug().Msgf("split of file: %v, %#v\n", name, split)
		if len(split) < 2 {
			continue
		}
		suffix := strings.Replace(split[1], filepath.Ext(split[1]), "", -1)

		logger.Debug().Msgf("found suffix of file: %v, suffix %v\n", name, suffix)
		logger.Debug().Msgf("sig files map %+v\n", signatureFiles)

		if foundSuffix, ok := signatureFiles[suffix]; ok {
			logger.Debug().Msgf("inserting signature files %+v\n", signatureFiles[suffix])
			idx := slices.Index(tempFiles, name)
			if i+1 == len(tempFiles) {
				logger.Debug().Msgf("appending to %+v\n", tempFiles)
				tempFiles = append(tempFiles, foundSuffix...)
			} else {
				logger.Debug().Msgf("inserting into %+v\n", tempFiles)
				tempFiles = slices.Insert(tempFiles, idx+1, foundSuffix...)
			}
		}
	}
	logger.Debug().Msgf("temp project files after adding sig file: %#v\n", tempFiles)
	return tempFiles
}
