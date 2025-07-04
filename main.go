package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	_ "path/filepath"
	doublestar "github.com/bmatcuk/doublestar/v4"
)

// TODO: move to github.com/earlye/go-common
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// TODO: move to github.com/earlye/go-common
func Must1[T1 any](t1 T1, err error) (T1) {
	Must(err)
	return t1
}

type YarCommand struct {
	Description string
	Script string

	// list of steps that are needed prior to running this command
	Needs []string

	// list of files that should trigger rebuilding if any are
	// newer than anything in .Creates
	Dependencies []string

	// list of files created by this command
	Creates []string
}

func ModTime(filename string) (result time.Time) {
	fileInfo := Must1(os.Stat(filename))
	result = fileInfo.ModTime()
	return
}

func (this *YarCommand) IsUpToDate() bool {
	// check if anything in .Creates is older than anything in .Dependencies
	newestDependencyTimestamp := time.Time{}
	for _, glob := range this.Dependencies {
		log.Printf("[TRACE] dependency glob: %s\n", glob)
		matches := Must1(doublestar.FilepathGlob(glob))
		for _, filename := range matches {
			fileTimestamp := ModTime(filename)
			if fileTimestamp.After(newestDependencyTimestamp) {
				newestDependencyTimestamp = fileTimestamp
			}
		}
	}

	newestCreate := time.Time{}
	for _, glob := range this.Creates {
		log.Printf("[TRACE] create glob: %s\n", glob)
		matches := Must1(doublestar.FilepathGlob(glob))
		for _, filename := range matches {
			fileTimestamp := ModTime(filename)
			if fileTimestamp.After(newestCreate) {
				newestCreate = fileTimestamp
			}
		}
	}

	log.Printf("[TRACE] newestDependency: %v newestCreate: %v\n", newestDependencyTimestamp, newestCreate)
	return newestCreate.After(newestDependencyTimestamp)
}

type YarData struct {
	Commands map[string]YarCommand
}

func loadYarFile(yarFilename string) (result *YarData) {
	result = &YarData{}

	yarFile := Must1(os.Open(yarFilename))
	defer yarFile.Close()

	yarDecoder := yaml.NewDecoder(yarFile)
	yarDecoder.Decode(result)
	return
}

func BuildScript(contents string) string {
	f := Must1(os.CreateTemp("", "temp"))
	defer f.Close()
	f.Write([]byte(contents))
	os.Chmod(f.Name(), 0755)
	return f.Name()
}


func rootCmd() (result *cobra.Command) {
	result = &cobra.Command {
		Use: fmt.Sprintf("%s args", Must1(os.Executable())),
		Short: fmt.Sprintf("%s runs YAR files", os.Executable),
		Run: func(cmd *cobra.Command, args[] string) {
			yarFilename := Must1(cmd.Flags().GetString("yar"))
			yarData := loadYarFile(yarFilename)

			if len(args) != 0 {
				commandName := args[0]
				switch commandName {
				case "help":
					if len(yarData.Commands) != 0 {
						fmt.Printf("The YAR file %s has the following commands available:\n",yarFilename)
						for command, commandData := range yarData.Commands {
							fmt.Printf("- %s [%s]\n", command, strings.TrimSpace(commandData.Description))
						}
					}
				default:
					command, ok := yarData.Commands[commandName]
					if ok {
						log.Printf("[TRACE] isUpToDate? %v\n", command.IsUpToDate())
						log.Printf("[TRACE] script contents\n%s\n", command.Script)
						name := BuildScript(command.Script)
						defer os.Remove(name)
						cmd := exec.Command(name)
						cmd.Stdin = os.Stdin
						cmd.Stdout = os.Stdout
						cmd.Stderr = os.Stderr
						Must(cmd.Run())
					}
				}
			}
		},
	}

	result.Flags().String("yar", "yar.yml", "Path to yar.yml file")
	result.Flags().SetInterspersed(false)
	return
}

func main() {
	root := rootCmd()
	root.Execute()
}
