package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/crowdsecurity/crowdsec/pkg/metabase"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	metabaseUser         = "crowdsec@crowdsec.net"
	metabasePassword     string
	metabaseDbPath       string
	metabaseConfigPath   string
	metabaseConfigFolder = "metabase/"
	metabaseConfigFile   = "metabase.yaml"
	metabaseImage        = "metabase/metabase"
	/**/
	metabaseListenAddress = "127.0.0.1"
	metabaseListenPort    = "3000"
	metabaseContainerID   = "/crowdsec-metabase"

	forceYes bool

	dockerGatewayIPAddr = "172.17.0.1"
	/*informations needed to setup a random password on user's behalf*/
)

func NewDashboardCmd() *cobra.Command {
	/* ---- UPDATE COMMAND */
	var cmdDashboard = &cobra.Command{
		Use:   "dashboard [command]",
		Short: "Manage your metabase dashboard container",
		Long:  `Install/Start/Stop/Remove a metabase container exposing dashboard and metrics.`,
		Args:  cobra.ExactArgs(1),
		Example: `
cscli dashboard setup
cscli dashboard start
cscli dashboard stop
cscli dashboard remove
`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			metabaseConfigFolderPath := filepath.Join(csConfig.ConfigPaths.ConfigDir, metabaseConfigFolder)
			metabaseConfigPath = filepath.Join(metabaseConfigFolderPath, metabaseConfigFile)
			if err := os.MkdirAll(metabaseConfigFolderPath, os.ModePerm); err != nil {
				log.Fatalf(err.Error())
			}
		},
	}

	var force bool
	var cmdDashSetup = &cobra.Command{
		Use:   "setup",
		Short: "Setup a metabase container.",
		Long:  `Perform a metabase docker setup, download standard dashboards, create a fresh user and start the container`,
		Args:  cobra.ExactArgs(0),
		Example: `
cscli dashboard setup
cscli dashboard setup --listen 0.0.0.0
cscli dashboard setup -l 0.0.0.0 -p 443 --password <password>
 `,
		Run: func(cmd *cobra.Command, args []string) {
			if metabaseDbPath == "" {
				metabaseDbPath = csConfig.ConfigPaths.DataDir
			}

			if metabasePassword == "" {
				metabasePassword = generatePassword(16)
			}
			mb, err := metabase.SetupMetabase(csConfig.API.Server.DbConfig, metabaseListenAddress, metabaseListenPort, metabaseUser, metabasePassword, metabaseDbPath)
			if err != nil {
				log.Fatalf(err.Error())
			}

			if err := mb.DumpConfig(metabaseConfigPath); err != nil {
				log.Fatalf(err.Error())
			}

			log.Infof("Metabase is ready")
			fmt.Println()
			fmt.Printf("\tURL       : '%s'\n", mb.Config.ListenURL)
			fmt.Printf("\tusername  : '%s'\n", mb.Config.Username)
			fmt.Printf("\tpassword  : '%s'\n", mb.Config.Password)
		},
	}
	cmdDashSetup.Flags().BoolVarP(&force, "force", "f", false, "Force setup : override existing files.")
	cmdDashSetup.Flags().StringVarP(&metabaseDbPath, "dir", "d", "", "Shared directory with metabase container.")
	cmdDashSetup.Flags().StringVarP(&metabaseListenAddress, "listen", "l", metabaseListenAddress, "Listen address of container")
	cmdDashSetup.Flags().StringVarP(&metabaseListenPort, "port", "p", metabaseListenPort, "Listen port of container")
	//cmdDashSetup.Flags().StringVarP(&metabaseUser, "user", "u", "crowdsec@crowdsec.net", "metabase user")
	cmdDashSetup.Flags().StringVar(&metabasePassword, "password", "", "metabase password")

	cmdDashboard.AddCommand(cmdDashSetup)

	var cmdDashStart = &cobra.Command{
		Use:   "start",
		Short: "Start the metabase container.",
		Long:  `Stats the metabase container using docker.`,
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			mb, err := metabase.NewMetabase(metabaseConfigPath)
			if err != nil {
				log.Fatalf(err.Error())
			}
			if err := mb.Container.Start(); err != nil {
				log.Fatalf("Failed to start metabase container : %s", err)
			}
			log.Infof("Started metabase")
			log.Infof("url : http://%s:%s", metabaseListenAddress, metabaseListenPort)
		},
	}
	cmdDashboard.AddCommand(cmdDashStart)

	var cmdDashStop = &cobra.Command{
		Use:   "stop",
		Short: "Stops the metabase container.",
		Long:  `Stops the metabase container using docker.`,
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			if err := metabase.StopContainer(metabaseContainerID); err != nil {
				log.Fatalf("unable to stop container '%s': %s", metabaseContainerID, err)
			}
		},
	}
	cmdDashboard.AddCommand(cmdDashStop)

	var cmdDashRemove = &cobra.Command{
		Use:   "remove",
		Short: "removes the metabase container.",
		Long:  `removes the metabase container using docker.`,
		Args:  cobra.ExactArgs(0),
		Example: `
cscli dashboard remove
cscli dashboard remove --force
 `,
		Run: func(cmd *cobra.Command, args []string) {
			answer := true
			if !forceYes {
				prompt := &survey.Confirm{
					Message: "Do you really want to remove crowdsec dashboard? (all your changes will be lost)",
					Default: true,
				}
				if err := survey.AskOne(prompt, &answer); err != nil {
					log.Fatalf("unable to ask to force: %s", err)
				}
			}

			if answer {
				if metabase.IsContainerExist(metabaseContainerID) {
					log.Debugf("Stopping container %s", metabaseContainerID)
					if err := metabase.StopContainer(metabaseContainerID); err != nil {
						log.Warningf("unable to stop container '%s': %s", metabaseContainerID, err)
					}
					log.Debugf("Removing container %s", metabaseContainerID)
					if err := metabase.RemoveContainer(metabaseContainerID); err != nil {
						log.Warningf("unable to remove container '%s': %s", metabaseContainerID, err)
					}
					log.Infof("container %s stopped & removed", metabaseContainerID)
				}
				log.Debugf("Removing database %s", csConfig.ConfigPaths.DataDir)
				if err := metabase.RemoveDatabase(csConfig.ConfigPaths.DataDir); err != nil {
					log.Warningf("failed to remove metabase internal db : %s", err)
				}
				if force {
					log.Debugf("Removing image %s", metabaseImage)
					if err := metabase.RemoveImageContainer(metabaseImage); err != nil {
            log.Warningf("Failed to remove metabase container %s : %s", metabaseImage, err)

					}
				}
			}
		},
	}
	cmdDashRemove.Flags().BoolVarP(&force, "force", "f", false, "Force remove : stop the container if running and remove.")
	cmdDashRemove.Flags().BoolVarP(&forceYes, "yes", "y", false, "force  yes")
	cmdDashboard.AddCommand(cmdDashRemove)

	return cmdDashboard
}
