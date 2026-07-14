package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pbzona/mkdb/internal/config"
	"github.com/pbzona/mkdb/internal/database"
	"github.com/pbzona/mkdb/internal/docker"
	"github.com/pbzona/mkdb/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose the mkdb environment",
	Long: `Run a series of checks on the local environment: Docker connectivity,
data directory access, encryption key, the state database, and drift between
Docker and mkdb's own records.

Exit codes: 0 all checks pass, 3 Docker is unreachable, 1 another check failed.`,
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type doctorReport struct {
	OK     bool          `json:"ok"`
	Checks []doctorCheck `json:"checks"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	var checks []doctorCheck

	// Docker daemon reachability.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dockerErr := docker.Ping(ctx)
	checks = append(checks, boolCheck("docker", dockerErr == nil, dockerErr, "Docker daemon reachable"))

	// Data directory writable.
	dataErr := checkWritable(config.DataDir)
	checks = append(checks, boolCheck("data_dir", dataErr == nil, dataErr,
		fmt.Sprintf("data directory writable (%s)", config.DataDir)))

	// Encryption key present.
	keyPath := filepath.Join(config.DataDir, config.KeyFileName)
	_, keyErr := os.Stat(keyPath)
	checks = append(checks, boolCheck("encryption_key", keyErr == nil, keyErr, "encryption key present"))

	// State database readable.
	containers, dbErr := database.ListContainers()
	checks = append(checks, boolCheck("state_db", dbErr == nil, dbErr, "state database readable"))

	// Drift between Docker and mkdb records (only meaningful when Docker is up).
	if dockerErr == nil && dbErr == nil {
		checks = append(checks, driftCheck(ctx, containers))
	}

	ok := true
	for _, c := range checks {
		if !c.OK {
			ok = false
		}
	}

	if jsonOutput {
		if err := outputJSON(doctorReport{OK: ok, Checks: checks}); err != nil {
			return err
		}
	} else {
		printDoctor(checks, ok)
	}

	if ok {
		return nil
	}
	if dockerErr != nil {
		return withExitCode(exitDockerUnavailable, fmt.Errorf("Docker is unavailable"))
	}
	return withExitCode(exitGeneral, fmt.Errorf("one or more checks failed"))
}

// driftCheck reports containers that exist in Docker but not mkdb's records (or
// vice versa).
func driftCheck(ctx context.Context, containers []*database.Container) doctorCheck {
	managed, err := docker.ListManaged(ctx)
	if err != nil {
		return doctorCheck{Name: "drift", OK: false, Detail: err.Error()}
	}

	dockerByName := make(map[string]bool, len(managed))
	for _, m := range managed {
		dockerByName[m.Name] = true
	}
	recordByName := make(map[string]bool, len(containers))
	for _, c := range containers {
		recordByName[c.DisplayName] = true
	}

	var issues []string
	for _, m := range managed {
		if m.Name != "" && !recordByName[m.Name] {
			issues = append(issues, fmt.Sprintf("%q running in Docker but not tracked", m.Name))
		}
	}
	for _, c := range containers {
		if c.ContainerID != "" && !dockerByName[c.DisplayName] {
			issues = append(issues, fmt.Sprintf("%q tracked but missing from Docker", c.DisplayName))
		}
	}

	if len(issues) == 0 {
		return doctorCheck{Name: "drift", OK: true, Detail: "Docker and mkdb records agree"}
	}
	detail := issues[0]
	for _, i := range issues[1:] {
		detail += "; " + i
	}
	return doctorCheck{Name: "drift", OK: false, Detail: detail}
}

func boolCheck(name string, ok bool, err error, okDetail string) doctorCheck {
	if ok {
		return doctorCheck{Name: name, OK: true, Detail: okDetail}
	}
	detail := okDetail
	if err != nil {
		detail = err.Error()
	}
	return doctorCheck{Name: name, OK: false, Detail: detail}
}

func checkWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".doctor-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

func printDoctor(checks []doctorCheck, ok bool) {
	for _, c := range checks {
		if c.OK {
			ui.Success(fmt.Sprintf("%-16s %s", c.Name, c.Detail))
		} else {
			ui.Error(fmt.Sprintf("%-16s %s", c.Name, c.Detail))
		}
	}
	if ok {
		ui.Success("All checks passed")
	} else {
		ui.Warning("Some checks failed")
	}
}
