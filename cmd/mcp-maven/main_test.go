package main

import (
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersionFlagPrintsBuildInformation(t *testing.T) {
	output := runMainProcess(t, "-version")

	if !strings.Contains(output, "mcp-maven") {
		t.Fatalf("expected version output to include binary name, got %q", output)
	}
	if !strings.Contains(output, "Version:") || !strings.Contains(output, "Commit:") || !strings.Contains(output, "Built:") {
		t.Fatalf("expected version output to include build fields, got %q", output)
	}
}

func TestHelpFlagPrintsUsage(t *testing.T) {
	output := runMainProcess(t, "-help")

	if !strings.Contains(output, "Usage:") {
		t.Fatalf("expected help output to include usage, got %q", output)
	}
	if !strings.Contains(output, "-http") || !strings.Contains(output, "MAVEN_CENTRAL_URL") {
		t.Fatalf("expected help output to include flags and environment variables, got %q", output)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("MCP_MAVEN_TEST_HELPER") != "1" {
		return
	}

	args := []string{"mcp-maven"}
	for i, arg := range os.Args {
		if arg == "--" {
			args = append(args, os.Args[i+1:]...)
			break
		}
	}
	os.Args = args
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	main()
	os.Exit(0)
}

func runMainProcess(t *testing.T, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"-test.run=TestHelperProcess", "--"}, args...)
	cmd := exec.Command(os.Args[0], cmdArgs...)
	cmd.Env = append(os.Environ(), "MCP_MAVEN_TEST_HELPER=1")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helper process failed: %v; output=%s", err, string(output))
	}
	return string(output)
}
