package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestApplyEnvToFlagSetsDefaultFromEnv(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	command.Flags().String("basepath", "/", "")

	t.Setenv("OPENBOOKS_BASEPATH", "/openbooks/")

	if err := applyEnvToFlag(command, "basepath", "OPENBOOKS_BASEPATH"); err != nil {
		t.Fatalf("apply env to flag: %v", err)
	}

	value, err := command.Flags().GetString("basepath")
	if err != nil {
		t.Fatalf("get basepath flag: %v", err)
	}
	if value != "/openbooks/" {
		t.Fatalf("expected env basepath to be applied, got %q", value)
	}
}

func TestApplyEnvToFlagDoesNotOverrideExplicitFlag(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	command.Flags().String("name", "", "")

	if err := command.Flags().Set("name", "from-flag"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	t.Setenv("OPENBOOKS_NAME", "from-env")

	if err := applyEnvToFlag(command, "name", "OPENBOOKS_NAME"); err != nil {
		t.Fatalf("apply env to flag: %v", err)
	}

	value, err := command.Flags().GetString("name")
	if err != nil {
		t.Fatalf("get name flag: %v", err)
	}
	if value != "from-flag" {
		t.Fatalf("expected explicit flag to win, got %q", value)
	}
}

func TestApplyEnvToFlagRejectsInvalidBool(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	command.Flags().Bool("tls", true, "")
	t.Setenv("OPENBOOKS_TLS", "not-a-bool")

	err := applyEnvToFlag(command, "tls", "OPENBOOKS_TLS")
	if err == nil {
		t.Fatal("expected invalid bool env value to return error")
	}
}
