package migrate

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	htdocument "github.com/herdworks/hypertemplates-go/document"
	htfiles "github.com/herdworks/hypertemplates-go/files"
	migrationrules "github.com/herdworks/hypertemplates-go/migrate/rules"
	migrationtypes "github.com/herdworks/hypertemplates-go/migrate/types"
)

var errJobFailed = errors.New("migration job failed")

// Job applies one rule to one theme resource.
type Job struct {
	Rule       migrationrules.Rule
	File       migrationtypes.File
	Nodes      []*html.Node
	Candidates []*migrationtypes.Candidate
	Report     migrationtypes.Status
	Modified   bool
	themeRoot  string
}

// JobOptions configures one Job execution.
type JobOptions struct {
	Interactive bool
	Input       *bufio.Reader
	Output      io.Writer
}

// NewJob loads one resource and discovers the candidates for one rule.
// It returns nil, nil when the resource has no candidates for the rule.
func NewJob(themeRoot string, file migrationtypes.File, rule migrationrules.Rule) (*Job, error) {
	if err := migrationrules.Validate(rule); err != nil {
		return nil, err
	}
	job := &Job{Rule: rule, File: file, themeRoot: themeRoot}
	path, err := job.Path()
	if err != nil {
		return nil, fmt.Errorf("resolve file: %w", err)
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	job.Nodes, err = parseMigrationNodes(source, file.Kind)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}
	job.Candidates = migrationCandidates(job.Nodes, file, job.Rule.Selectors())
	if len(job.Candidates) == 0 {
		return nil, nil
	}
	return job, nil
}

// Path resolves the Job file within its theme root.
func (job *Job) Path() (string, error) {
	if job == nil {
		return "", fmt.Errorf("migration job is nil")
	}
	return htfiles.LocalPath(job.themeRoot, job.File.Path)
}

// Run processes and commits the Job.
func (job *Job) Run(options JobOptions) error {
	if job == nil {
		return fmt.Errorf("migration job is nil")
	}
	if options.Input == nil {
		options.Input = bufio.NewReader(os.Stdin)
	}
	if options.Output == nil {
		options.Output = os.Stdout
	}
	for _, candidate := range job.Candidates {
		err := job.processCandidate(options, candidate)
		if errors.Is(err, errJobFailed) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	if err := job.commit(); err != nil {
		job.fail(err.Error())
	}
	return nil
}

func (job *Job) processCandidate(options JobOptions, candidate *migrationtypes.Candidate) error {
	proposal, err := job.processProposal(options, candidate)
	if err != nil || proposal == nil {
		return err
	}
	accepted, err := job.promptProposal(options, candidate, proposal)
	if err != nil || !accepted {
		return err
	}
	return job.applyProposal(candidate, proposal)
}

func (job *Job) processProposal(options JobOptions, candidate *migrationtypes.Candidate) (*migrationtypes.Proposal, error) {
	result := candidateResult(candidate, job.Rule.Selectors())
	proposal, err := job.Rule.Proposal(candidate.Clone())
	var abort *migrationtypes.AbortMigrationError
	var manual *migrationtypes.ManualMigrationRequiredError
	switch {
	case err == nil && proposal == nil:
		return nil, nil
	case err == nil && proposal.Optional:
		job.Report.Counters.OptionalFound++
		return proposal, nil
	case err == nil:
		return proposal, nil
	case errors.As(err, &abort):
		return nil, abort
	case errors.As(err, &manual):
		result.Status = migrationtypes.ResultUnresolved
		result.Message = manual.Error()
		if !options.Interactive {
			job.Report.Add(result)
			return nil, nil
		}
		elementNode := htdocument.CloneElement(candidate.Element)
		elementString, renderErr := htdocument.NodesToHTMLString([]*html.Node{elementNode})
		if renderErr != nil {
			result.Status = migrationtypes.ResultFailed
			result.Message = fmt.Sprintf("render candidate: %v", renderErr)
			job.Report.Add(result)
			return nil, nil
		}
		job.Report.Add(result)
		return nil, job.pauseManual(options, result, elementString)
	default:
		result.Status = migrationtypes.ResultFailed
		result.Message = err.Error()
		job.Report.Add(result)
		return nil, nil
	}
}

func (job *Job) promptProposal(options JobOptions, candidate *migrationtypes.Candidate, proposal *migrationtypes.Proposal) (bool, error) {
	result := candidateResult(candidate, job.Rule.Selectors())
	if !options.Interactive && !proposal.AutoApply {
		if proposal.Optional {
			job.Report.Counters.OptionalRetained++
			result.Status = migrationtypes.ResultOptional
			result.Message = "optional migration retained; use --interactive to apply"
		} else {
			result.Status = migrationtypes.ResultUnresolved
			result.Message = "migration proposal requires interactive approval"
			job.Report.Add(result)
		}
		return false, nil
	}
	result.Message = proposal.Message
	if proposal.After == nil {
		result.Status = migrationtypes.ResultFailed
		result.Message = "rule returned a proposal without a preview element"
		job.Report.Add(result)
		return false, nil
	}
	beforeNode := htdocument.CloneElement(candidate.Element)
	before, err := htdocument.NodesToHTMLString([]*html.Node{beforeNode})
	if err != nil {
		result.Status = migrationtypes.ResultFailed
		result.Message = fmt.Sprintf("render candidate: %v", err)
		job.Report.Add(result)
		return false, nil
	}
	after, err := htdocument.NodesToHTMLString([]*html.Node{proposal.After})
	if err != nil {
		result.Status = migrationtypes.ResultFailed
		result.Message = fmt.Sprintf("render proposal: %v", err)
		job.Report.Add(result)
		return false, nil
	}
	accepted, err := job.reviewProposal(options, result, before, after)
	if err != nil {
		return false, err
	}
	if !accepted {
		if proposal.Optional {
			job.Report.Counters.OptionalRetained++
		} else {
			job.Report.Counters.Declined++
		}
		return false, nil
	}
	return true, nil
}

func (job *Job) applyProposal(candidate *migrationtypes.Candidate, proposal *migrationtypes.Proposal) error {
	result := candidateResult(candidate, job.Rule.Selectors())
	applyErr := job.Rule.Apply(candidate)
	var warning *migrationtypes.MigrationWarning
	if applyErr != nil && !errors.As(applyErr, &warning) {
		var abort *migrationtypes.AbortMigrationError
		if errors.As(applyErr, &abort) {
			return abort
		}
		job.fail(applyErr.Error())
		return errJobFailed
	}
	if candidate.Element == nil {
		job.fail("candidate element is nil")
		return errJobFailed
	}

	job.Modified = true
	if proposal.Optional {
		job.Report.Counters.OptionalRemoved++
		result.Status = migrationtypes.ResultOptional
	} else if warning != nil {
		result.Status = migrationtypes.ResultWarned
	} else {
		result.Status = migrationtypes.ResultMigrated
	}
	result.Message = proposal.Message
	if warning != nil {
		if result.Message == "" {
			result.Message = warning.Error()
		} else {
			result.Message += ": " + warning.Error()
		}
	}
	job.Report.Add(result)
	return nil
}

func (job *Job) pauseManual(options JobOptions, result migrationtypes.Result, element string) error {
	if _, err := fmt.Fprintf(options.Output, "Migration candidate for %s in %s:\n%s\n%s\n[Enter/q] ", result.Selector, result.File, result.Message, element); err != nil {
		return err
	}
	for {
		answer, err := readAnswer(options.Input)
		if errors.Is(err, io.EOF) {
			return ErrInterrupted
		}
		if err != nil {
			return err
		}
		switch answer {
		case "":
			return nil
		case "q", "Q":
			return ErrInterrupted
		default:
			if _, err := fmt.Fprint(options.Output, "[Enter/q] "); err != nil {
				return err
			}
		}
	}
}

func (job *Job) reviewProposal(options JobOptions, result migrationtypes.Result, before, after string) (bool, error) {
	if !options.Interactive {
		return true, nil
	}
	if _, err := fmt.Fprintf(options.Output, "Migration candidate for %s in %s:\n%s\nbefore:\n%s\nafter:\n%s\n[y/N/q] ", result.Selector, result.File, result.Message, before, after); err != nil {
		return false, err
	}
	for {
		answer, err := readAnswer(options.Input)
		if errors.Is(err, io.EOF) {
			return false, ErrInterrupted
		}
		if err != nil {
			return false, err
		}
		switch answer {
		case "y", "Y":
			return true, nil
		case "", "n", "N":
			return false, nil
		case "q", "Q":
			return false, ErrInterrupted
		default:
			if _, err := fmt.Fprint(options.Output, "[y/N/q] "); err != nil {
				return false, err
			}
		}
	}
}

func (job *Job) commit() error {
	if !job.Modified {
		return nil
	}
	path, err := job.Path()
	if err != nil {
		return fmt.Errorf("resolve file: %w", err)
	}
	rendered, err := htdocument.NodesToHTMLString(job.Nodes)
	if err != nil {
		return fmt.Errorf("serialize HTML: %w", err)
	}
	if err := os.WriteFile(path, []byte(rendered), 0o600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func (job *Job) fail(message string) {
	job.Nodes = nil
	job.Modified = false
	job.Report = migrationtypes.Status{}
	job.Report.Add(migrationtypes.Result{Status: migrationtypes.ResultFailed, File: job.File.Path, Message: message})
}

func parseMigrationNodes(source []byte, kind migrationtypes.FileKind) ([]*html.Node, error) {
	switch kind {
	case migrationtypes.LayoutFile:
		root, err := html.Parse(bytes.NewReader(source))
		if err != nil {
			return nil, err
		}
		if root.Type != html.DocumentNode {
			return nil, fmt.Errorf("layout root is not a document node")
		}
		if element := htdocument.Query(root, "html"); element == nil {
			return nil, fmt.Errorf("layout document has no root html element")
		}
		return []*html.Node{root}, nil
	case migrationtypes.FragmentFile:
		context := &html.Node{Type: html.ElementNode, DataAtom: atom.Template, Data: atom.Template.String()}
		fragments, err := html.ParseFragment(bytes.NewReader(source), context)
		if err != nil {
			return nil, err
		}
		root := &html.Node{Type: html.DocumentNode}
		for _, fragment := range fragments {
			root.AppendChild(fragment)
		}
		return []*html.Node{root}, nil
	default:
		return nil, fmt.Errorf("unknown migration resource kind")
	}
}

func candidateResult(candidate *migrationtypes.Candidate, selectors []string) migrationtypes.Result {
	return migrationtypes.Result{
		File:     candidate.File,
		Selector: strings.Join(selectors, ", "),
		Element:  candidate.Element.Data,
	}
}
