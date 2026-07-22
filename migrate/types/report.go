package types

import (
	"fmt"
	"io"
	"strings"
)

// Status contains all migration results and the overall completion state.
type Status struct {
	Results     []Result
	Counters    Counters
	Interrupted bool
}

// Counters contains aggregate migration counts.
type Counters struct {
	FilesScanned     int
	FilesChanged     int
	Migrated         int
	Warned           int
	Declined         int
	Unresolved       int
	Failed           int
	OptionalFound    int
	OptionalRemoved  int
	OptionalRetained int
}

// ResultStatus describes the outcome of one migration decision.
type ResultStatus string

// Result is one actionable migration outcome.
type Result struct {
	Status   ResultStatus
	File     string
	Selector string
	Element  string
	Message  string
}

const (
	ResultMigrated   ResultStatus = "migrated"
	ResultWarned     ResultStatus = "warned"
	ResultDeclined   ResultStatus = "declined"
	ResultUnresolved ResultStatus = "unresolved"
	ResultFailed     ResultStatus = "failed"
	ResultOptional   ResultStatus = "optional"
)

// Complete reports whether the theme has no required migration work left.
func (status Status) Complete() bool {
	return !status.Interrupted &&
		status.Counters.Declined == 0 &&
		status.Counters.Unresolved == 0 &&
		status.Counters.Failed == 0
}

// Add records one migration result and updates its counters.
func (status *Status) Add(result Result) {
	status.Results = append(status.Results, result)
	switch result.Status {
	case ResultMigrated:
		status.Counters.Migrated++
	case ResultWarned:
		status.Counters.Warned++
	case ResultDeclined:
		status.Counters.Declined++
	case ResultUnresolved:
		status.Counters.Unresolved++
	case ResultFailed:
		status.Counters.Failed++
	}
}

// Merge records another completed migration status.
func (status *Status) Merge(other Status) {
	status.Results = append(status.Results, other.Results...)
	status.Counters.Migrated += other.Counters.Migrated
	status.Counters.Warned += other.Counters.Warned
	status.Counters.Declined += other.Counters.Declined
	status.Counters.Unresolved += other.Counters.Unresolved
	status.Counters.Failed += other.Counters.Failed
	status.Counters.OptionalFound += other.Counters.OptionalFound
	status.Counters.OptionalRemoved += other.Counters.OptionalRemoved
	status.Counters.OptionalRetained += other.Counters.OptionalRetained
}

// Print writes a stable human-readable migration status.
func (status Status) Print(w io.Writer) error {
	if w == nil {
		return nil
	}
	for _, result := range status.Results {
		location := result.File
		context := ""
		if result.Selector != "" {
			context += " " + result.Selector
		}
		if result.Element != "" {
			context += " <" + result.Element + ">"
		}
		message := ""
		if result.Message != "" {
			message = ": " + result.Message
		}
		if _, err := fmt.Fprintf(w, "%s %s%s%s\n", result.Status, location, context, message); err != nil {
			return err
		}
	}

	result := "complete"
	if status.Interrupted {
		result = "interrupted"
	} else if status.Counters.Failed > 0 {
		result = "failed"
	} else if !status.Complete() {
		result = "incomplete"
	}
	lines := []string{
		fmt.Sprintf("files: %d scanned, %d changed", status.Counters.FilesScanned, status.Counters.FilesChanged),
		fmt.Sprintf("results: %d migrated, %d warned, %d declined, %d unresolved, %d failed", status.Counters.Migrated, status.Counters.Warned, status.Counters.Declined, status.Counters.Unresolved, status.Counters.Failed),
		fmt.Sprintf("optional: %d found, %d removed, %d retained", status.Counters.OptionalFound, status.Counters.OptionalRemoved, status.Counters.OptionalRetained),
		"result: " + result,
	}
	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}
