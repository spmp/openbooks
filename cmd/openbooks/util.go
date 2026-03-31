package main

import (
	"crypto/rand"
	"errors"
	"math/big"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/evan-buss/openbooks/server"
	"github.com/spf13/cobra"
)

// Update a server config struct from globalFlags
func bindGlobalServerFlags(config *server.Config) {
	config.Debug = debug
	config.UserAgent = globalFlags.UserAgent
	config.UserName = globalFlags.UserName
	config.Log = globalFlags.Log
	config.Server = globalFlags.Server
	config.SearchBot = globalFlags.SearchBot
	config.EnableTLS = globalFlags.EnableTLS
}

// Make sure the server config has a valid rate limit.
func ensureValidRate(rateLimit int, config *server.Config) {

	// If user enters a limit that's too low, set to default of 10 seconds.
	if rateLimit < 10 {
		rateLimit = 10
	}

	config.SearchTimeout = time.Duration(rateLimit) * time.Second
}

func sanitizePath(basepath string) string {
	cleaned := path.Clean(basepath)
	if cleaned == "/" {
		return cleaned
	}
	return cleaned + "/"
}

func applyUsernamePolicy(assignRandomAfter int, userName *string) error {
	*userName = strings.TrimSpace(*userName)

	if assignRandomAfter > 0 {
		if *userName != "" {
			return errors.New("--assign-random-username-after cannot be used with --name")
		}

		*userName = randomAlphaNumeric(12)
		return nil
	}

	if *userName == "" {
		return errors.New("required flag(s) \"name\" not set")
	}

	return nil
}

func randomAlphaNumeric(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)

	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			b[i] = alphabet[i%len(alphabet)]
			continue
		}
		b[i] = alphabet[n.Int64()]
	}

	return string(b)
}

func applyGlobalEnvFlags(cmd *cobra.Command) error {
	bindings := []flagEnvBinding{
		{Flag: "debug", Env: "OPENBOOKS_DEBUG"},
		{Flag: "debug", Env: "DEBUG"},
		{Flag: "name", Env: "OPENBOOKS_NAME"},
		{Flag: "server", Env: "OPENBOOKS_SERVER"},
		{Flag: "tls", Env: "OPENBOOKS_TLS"},
		{Flag: "log", Env: "OPENBOOKS_LOG"},
		{Flag: "searchbot", Env: "OPENBOOKS_SEARCHBOT"},
		{Flag: "useragent", Env: "OPENBOOKS_USERAGENT"},
	}

	for _, binding := range bindings {
		if err := applyEnvToFlag(cmd, binding.Flag, binding.Env); err != nil {
			return err
		}
	}

	return nil
}

func applyServerModeEnvFlags(cmd *cobra.Command) error {
	bindings := []flagEnvBinding{
		{Flag: "port", Env: "OPENBOOKS_PORT"},
		{Flag: "rate-limit", Env: "OPENBOOKS_RATE_LIMIT"},
		{Flag: "dir", Env: "OPENBOOKS_DIR"},
		{Flag: "post-download-hook", Env: "OPENBOOKS_POST_DOWNLOAD_HOOK"},
		{Flag: "post-download-hook-timeout", Env: "OPENBOOKS_POST_DOWNLOAD_HOOK_TIMEOUT"},
		{Flag: "post-download-hook-workers", Env: "OPENBOOKS_POST_DOWNLOAD_HOOK_WORKERS"},
		{Flag: "assign-random-username-after", Env: "OPENBOOKS_ASSIGN_RANDOM_USERNAME_AFTER"},
		{Flag: "no-browser-downloads", Env: "OPENBOOKS_NO_BROWSER_DOWNLOADS"},
		{Flag: "persist", Env: "OPENBOOKS_PERSIST"},
		{Flag: "browser", Env: "OPENBOOKS_BROWSER"},
		{Flag: "basepath", Env: "OPENBOOKS_BASEPATH"},
	}

	for _, binding := range bindings {
		if err := applyEnvToFlag(cmd, binding.Flag, binding.Env); err != nil {
			return err
		}
	}

	if err := applyEnvToFlag(cmd, "basepath", "BASE_PATH"); err != nil {
		return err
	}

	return nil
}

func applyCliModeEnvFlags(cmd *cobra.Command) error {
	return applyEnvToFlag(cmd, "dir", "OPENBOOKS_DIR")
}

type flagEnvBinding struct {
	Flag string
	Env  string
}

func applyEnvToFlag(cmd *cobra.Command, flagName string, envName string) error {
	flag := cmd.Flags().Lookup(flagName)
	if flag == nil {
		flag = cmd.InheritedFlags().Lookup(flagName)
	}
	if flag == nil {
		return nil
	}

	if flag.Changed {
		return nil
	}

	raw, present := os.LookupEnv(envName)
	if !present {
		return nil
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	if err := validateFlagEnvValue(flagName, raw); err != nil {
		return errors.New(envName + " " + err.Error())
	}

	return cmd.Flags().Set(flagName, raw)
}

func validateFlagEnvValue(flagName string, raw string) error {
	switch flagName {
	case "debug", "tls", "log", "no-browser-downloads", "persist", "browser":
		if _, err := strconv.ParseBool(raw); err != nil {
			return errors.New("must be a boolean value")
		}
	case "rate-limit", "post-download-hook-timeout", "post-download-hook-workers", "assign-random-username-after":
		if _, err := strconv.Atoi(raw); err != nil {
			return errors.New("must be an integer value")
		}
	}

	return nil
}
