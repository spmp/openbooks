package main

import (
	"path"
	"time"

	"github.com/evan-buss/openbooks/server"
	"github.com/evan-buss/openbooks/util"

	"github.com/spf13/cobra"
)

var openBrowser = false
var serverConfig server.Config

func init() {
	desktopCmd.AddCommand(serverCmd)

	serverCmd.Flags().StringVarP(&serverConfig.Port, "port", "p", "5228", "Set the local network port for browser mode.")
	serverCmd.Flags().IntP("rate-limit", "r", 10, "The number of seconds to wait between searches to reduce strain on IRC search servers. Minimum is 10 seconds.")
	serverCmd.Flags().BoolVar(&serverConfig.DisableBrowserDownloads, "no-browser-downloads", false, "The browser won't recieve and download eBook files, but they are still saved to the defined 'dir' path.")
	serverCmd.Flags().StringVar(&serverConfig.Basepath, "basepath", "/", `Base path where the application is accessible. For example "/openbooks/".`)
	serverCmd.Flags().BoolVarP(&openBrowser, "browser", "b", false, "Open the browser on server start.")
	serverCmd.Flags().BoolVar(&serverConfig.Persist, "persist", false, "Persist eBooks in 'dir'. Default is to delete after sending.")
	serverCmd.Flags().StringVarP(&serverConfig.DownloadDir, "dir", "d", "/books", "The directory where eBooks are saved when persist enabled.")
	serverCmd.Flags().StringVar(&serverConfig.PostDownloadHook, "post-download-hook", "", "Executable path to run after a book download completes.")
	serverCmd.Flags().Int("post-download-hook-timeout", 20, "Seconds to wait before terminating post-download-hook.")
	serverCmd.Flags().Int("post-download-hook-workers", 1, "Maximum number of post-download-hook processes running at once.")
	serverCmd.Flags().Int("assign-random-username-after", 0, "Rotate to a random IRC username after N searches and downloads. Disabled when set to 0.")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run OpenBooks in server mode.",
	Long:  "Run OpenBooks in server mode. This allows you to use a web interface to search and download eBooks.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if err := applyGlobalEnvFlags(cmd); err != nil {
			return err
		}
		if err := applyServerModeEnvFlags(cmd); err != nil {
			return err
		}

		assignRandomAfter, _ := cmd.Flags().GetInt("assign-random-username-after")
		serverConfig.AssignRandomUsernameAfter = assignRandomAfter

		if err := applyUsernamePolicy(assignRandomAfter, &globalFlags.UserName); err != nil {
			return err
		}

		bindGlobalServerFlags(&serverConfig)
		rateLimit, _ := cmd.Flags().GetInt("rate-limit")
		hookTimeout, _ := cmd.Flags().GetInt("post-download-hook-timeout")
		hookWorkers, _ := cmd.Flags().GetInt("post-download-hook-workers")
		if hookTimeout < 1 {
			hookTimeout = 20
		}
		if hookWorkers < 1 {
			hookWorkers = 1
		}
		if serverConfig.AssignRandomUsernameAfter < 0 {
			serverConfig.AssignRandomUsernameAfter = 0
		}
		serverConfig.PostDownloadHookTimeout = time.Duration(hookTimeout) * time.Second
		serverConfig.PostDownloadHookWorkers = hookWorkers
		ensureValidRate(rateLimit, &serverConfig)
		serverConfig.Basepath = sanitizePath(serverConfig.Basepath)

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if openBrowser {
			browserUrl := "http://127.0.0.1:" + path.Join(serverConfig.Port+serverConfig.Basepath)
			util.OpenBrowser(browserUrl)
		}

		server.Start(serverConfig)
	},
}
