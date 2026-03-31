package main

import (
	"testing"

	"github.com/spf13/pflag"
)

func TestServerFlagDefaults(t *testing.T) {
	if got := serverCmd.Flags().Lookup("dir").DefValue; got != "/books" {
		t.Fatalf("expected default dir /books, got %q", got)
	}

	if got := serverCmd.Flags().Lookup("port").DefValue; got != "5228" {
		t.Fatalf("expected default port 5228, got %q", got)
	}
}

func TestServerPreRunEEnvVarsSetConfig(t *testing.T) {
	resetServerState := saveServerState()
	defer resetServerState()
	resetFlagSet(serverCmd.Flags())
	resetFlagSet(serverCmd.InheritedFlags())

	t.Setenv("OPENBOOKS_NAME", "env-user")
	t.Setenv("OPENBOOKS_DIR", "/tmp/env-books")
	t.Setenv("OPENBOOKS_PORT", "9001")

	if err := serverCmd.PreRunE(serverCmd, nil); err != nil {
		t.Fatalf("pre-run failed: %v", err)
	}

	if serverConfig.DownloadDir != "/tmp/env-books" {
		t.Fatalf("expected env dir to set config, got %q", serverConfig.DownloadDir)
	}

	if serverConfig.Port != "9001" {
		t.Fatalf("expected env port to set config, got %q", serverConfig.Port)
	}
}

func TestServerPreRunECliFlagsOverrideEnvVars(t *testing.T) {
	resetServerState := saveServerState()
	defer resetServerState()
	resetFlagSet(serverCmd.Flags())
	resetFlagSet(serverCmd.InheritedFlags())

	t.Setenv("OPENBOOKS_NAME", "env-user")
	t.Setenv("OPENBOOKS_DIR", "/tmp/env-books")
	t.Setenv("OPENBOOKS_PORT", "9001")

	if err := serverCmd.Flags().Set("dir", "/tmp/cli-books"); err != nil {
		t.Fatalf("set cli dir flag: %v", err)
	}
	if err := serverCmd.Flags().Set("port", "7777"); err != nil {
		t.Fatalf("set cli port flag: %v", err)
	}
	if err := serverCmd.InheritedFlags().Set("name", "cli-user"); err != nil {
		t.Fatalf("set cli name flag: %v", err)
	}

	if err := serverCmd.PreRunE(serverCmd, nil); err != nil {
		t.Fatalf("pre-run failed: %v", err)
	}

	if serverConfig.DownloadDir != "/tmp/cli-books" {
		t.Fatalf("expected cli dir to override env, got %q", serverConfig.DownloadDir)
	}

	if serverConfig.Port != "7777" {
		t.Fatalf("expected cli port to override env, got %q", serverConfig.Port)
	}
}

func saveServerState() func() {
	originalGlobalFlags := globalFlags
	originalServerConfig := serverConfig
	return func() {
		globalFlags = originalGlobalFlags
		serverConfig = originalServerConfig
	}
}

func resetFlagSet(flagSet *pflag.FlagSet) {
	flagSet.VisitAll(func(flag *pflag.Flag) {
		_ = flagSet.Set(flag.Name, flag.DefValue)
		flag.Changed = false
	})
}
