package main

import (
	"log"
	"os"

	"github.com/TwiN/go-color"
	"github.com/alknopfler/seactl/cmd"
	"github.com/spf13/cobra"
)

var version = "dev"
var osExit = os.Exit

func init() {

}

func main() {
	command := newCommand()
	if err := command.Execute(); err != nil {
		log.Fatalf(color.InRed("[ERROR] %s"), err.Error())
	}
}

func newCommand() *cobra.Command {
	c := &cobra.Command{
		Use:   "seactl",
		Short: "SUSE Edge Air-gap tool enables to create an air-gap scenario using the suse-edge airgap manifest",
		Long: "SUSE Edge Air-gap tool enables to create an air-gap scenario using the suse-edge airgap manifest. The output could be a tarball, but also you could upload to a private registry.\n" +
			"Features: \n" +
			"- Read the SUSE Edge airgap manifest (pulling from release container)\n" +
			"- Save artifacts to a tarball\n" +
			"- Login to a private registry\n" +
			"- Upload and preload the private registry with the artifacts\n" +
			"\n",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			osExit(0)
		},
		Version: version,
	}

	c.SetVersionTemplate("seactl version {{.Version}}\n")

	c.AddCommand(cmd.NewAirGapCommand())

	return c
}
