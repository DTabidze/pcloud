package commands

import (
	"github.com/giolekva/pcloud/core/kg/log"
	"github.com/giolekva/pcloud/core/kg/server"
	"github.com/spf13/cobra"
)

// Command is an abstraction of the cobra Command
type Command = cobra.Command

// Run function starts the application
func Run(args []string) error {
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// rootCmd is a command to run the server.
var rootCmd = &Command{
	Use:   "server",
	Short: "An example of the basic server",
	RunE:  serverCmdF,
}

func serverCmdF(command *cobra.Command, args []string) error {
	config := &log.LoggerConfiguration{
		EnableConsole: true,
		ConsoleJSON:   true,
		ConsoleLevel:  "debug",
		EnableFile:    true,
		FileJSON:      true,
		FileLevel:     "debug",
		FileLocation:  "server.log",
	}
	logger := log.NewLogger(config)
	grpcServer := server.NewGRPCServer(logger)
	servers := server.New(logger)
	servers.AddServers(grpcServer)
	servers.Run()

	return nil
}
