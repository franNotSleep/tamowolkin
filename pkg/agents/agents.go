package agents

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/frannotsleep/tamowolkin/pkg/queue"
)

type Result struct{}

type AgentPoolConfig struct {
	WorkerCount     int
	Queue           *queue.Queue
	EasyDescription string
	PlanPrompt      string
	CodePrompt      string
	ProjectPath     string
	PlansDir        string
	WorktreesDir    string
	BaseBranch      string
}

type AgentPool struct {
	wc              int
	jobs            *queue.Queue
	results         chan Result
	projectPath     string
	plansDir        string
	worktreesDir    string
	baseBranch      string
	easyDescription string
	planPrompt      string
	codePrompt      string
}

func NewAgentPool(c AgentPoolConfig) *AgentPool {
	return &AgentPool{
		wc:              c.WorkerCount,
		jobs:            c.Queue,
		easyDescription: c.EasyDescription,
		planPrompt:      c.PlanPrompt,
		codePrompt:      c.CodePrompt,
		projectPath:     c.ProjectPath,
		plansDir:        c.PlansDir,
		worktreesDir:    c.WorktreesDir,
		baseBranch:      c.BaseBranch,
	}
}

func (ap *AgentPool) planPath(task queue.Task) string {
	ts := time.Now().UTC().Format("20060102-150405")
	return filepath.Join(ap.plansDir, fmt.Sprintf("%s-%s.md", task.ID, ts))
}

func (ap *AgentPool) branchName(task queue.Task) string {
	if task.BranchName != "" {
		return task.BranchName
	}
	return "tamowolkin/" + string(task.ID)
}

func (ap *AgentPool) Start() {
	for i := 0; i < ap.wc; i++ {
		go ap.agent(i, ap.jobs.Dequeue(), ap.results)
	}
}

func (ap *AgentPool) agent(id int, jobs <-chan queue.Task, results chan<- Result) {
	for job := range jobs {
		fmt.Printf("[agent %d] working on %s: %s\n", id, job.ID, job.Description)
		isEasy, err := ap.isEasy(id, job)
		if err != nil {
			log.Printf("[agent %d] classify %s: %v", id, job.ID, err)
			results <- Result{}
			continue
		}
		fmt.Printf("[agent %d] task %s easy=%t\n", id, job.ID, isEasy)
		if isEasy {
			if err := ap.codeTask(id, job); err != nil {
				log.Printf("[agent %d] code %s: %v", id, job.ID, err)
			}
		} else {
			if err := ap.generatePlan(id, job); err != nil {
				log.Printf("[agent %d] plan %s: %v", id, job.ID, err)
			}
		}
		results <- Result{}
	}
}

func (ap *AgentPool) codeTask(id int, job queue.Task) error {
	branch := ap.branchName(job)
	worktreePath := filepath.Join(ap.worktreesDir, string(job.ID))

	if _, err := os.Stat(worktreePath); err == nil {
		log.Printf("[agent %d] worktree %s already exists, skipping task %s", id, worktreePath, job.ID)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat worktree %s: %w", worktreePath, err)
	}

	if _, err := runGit(ap.projectPath, "fetch", "origin", ap.baseBranch); err != nil {
		log.Printf("[agent %d] fetch origin/%s failed (continuing with local ref): %v", id, ap.baseBranch, err)
	}

	startPoint := "origin/" + ap.baseBranch
	if _, err := runGit(ap.projectPath, "rev-parse", "--verify", startPoint); err != nil {
		startPoint = ap.baseBranch
		if _, err := runGit(ap.projectPath, "rev-parse", "--verify", startPoint); err != nil {
			return fmt.Errorf("base branch %q not found locally or on origin", ap.baseBranch)
		}
	}

	if _, err := runGit(ap.projectPath, "worktree", "add", worktreePath, "-b", branch, startPoint); err != nil {
		return fmt.Errorf("worktree add: %w", err)
	}
	fmt.Printf("[agent %d] worktree %s on branch %s (from %s)\n", id, worktreePath, branch, startPoint)

	prompt := buildCodePrompt(job, ap.codePrompt)
	fmt.Printf("[agent %d] code prompt:\n%s\n", id, prompt)
	cmd := exec.Command("agent", "-p", "--trust", prompt)
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("coding agent exited %d: %s", exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("run coding agent: %w", err)
	}
	if len(out) > 0 {
		fmt.Printf("[agent %d] coding agent output:\n%s\n", id, string(out))
	}

	if _, err := runGit(worktreePath, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	diffCmd := exec.Command("git", "diff", "--cached", "--quiet")
	diffCmd.Dir = worktreePath
	if err := diffCmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			msg := fmt.Sprintf("%s: %s", job.ID, job.Title)
			if _, cerr := runGit(worktreePath, "commit", "-m", msg); cerr != nil {
				return fmt.Errorf("git commit: %w", cerr)
			}
		} else if !errors.As(err, &exitErr) {
			return fmt.Errorf("git diff --cached: %w", err)
		}
	}

	headOut, err := runGit(worktreePath, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("rev-parse HEAD: %w", err)
	}
	baseOut, err := runGit(worktreePath, "rev-parse", startPoint)
	if err != nil {
		return fmt.Errorf("rev-parse %s: %w", startPoint, err)
	}
	if strings.TrimSpace(string(headOut)) == strings.TrimSpace(string(baseOut)) {
		fmt.Printf("[agent %d] task %s produced no commits, skipping push/PR\n", id, job.ID)
		return nil
	}

	if _, err := runGit(worktreePath, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	prBody := fmt.Sprintf("%s\n\nLinear: %s", job.Description, job.ID)
	if _, err := runGH(worktreePath, "pr", "create",
		"--base", ap.baseBranch,
		"--head", branch,
		"--title", fmt.Sprintf("%s: %s", job.ID, job.Title),
		"--body", prBody,
	); err != nil {
		return fmt.Errorf("gh pr create: %w", err)
	}

	fmt.Printf("[agent %d] opened PR for %s on branch %s\n", id, job.ID, branch)
	return nil
}

func (ap *AgentPool) generatePlan(id int, job queue.Task) error {
	prompt := buildPlanPrompt(job, ap.planPrompt)
	fmt.Printf("[agent %d] plan prompt:\n%s\n", id, prompt)
	cmd := exec.Command("agent", "-p", "--plan", "--trust", prompt)
	cmd.Dir = ap.projectPath
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("agent exited %d: %s", exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("run agent: %w", err)
	}
	path := ap.planPath(job)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write plan %s: %w", path, err)
	}
	fmt.Printf("[agent %d] wrote plan for %s to %s\n", id, job.ID, path)
	return nil
}

func buildPlanPrompt(task queue.Task, planPrompt string) string {
	return fmt.Sprintf(
		"%s\n\nTask title: %s\nTask description: %s",
		strings.TrimSpace(planPrompt), task.Title, task.Description,
	)
}

func buildCodePrompt(task queue.Task, codePrompt string) string {
	return fmt.Sprintf(
		"%s\n\nTask title: %s\nTask description: %s",
		strings.TrimSpace(codePrompt), task.Title, task.Description,
	)
}

func (ap *AgentPool) isEasy(id int, job queue.Task) (bool, error) {
	prompt := buildEasyPrompt(job, ap.easyDescription)
	fmt.Printf("[agent %d] easy prompt:\n%s\n", id, prompt)
	cmd := exec.Command("agent", "-p", "--trust", prompt)
	cmd.Dir = ap.projectPath
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, fmt.Errorf("agent exited %d: %s", exitErr.ExitCode(), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return false, fmt.Errorf("run agent: %w", err)
	}
	return parseEasyResponse(out)
}

func parseEasyResponse(output []byte) (bool, error) {
	s := strings.ToLower(strings.TrimSpace(string(output)))
	s = strings.ReplaceAll(s, " ", "")
	switch {
	case strings.Contains(s, "easy:true"):
		return true, nil
	case strings.Contains(s, "easy:false"):
		return false, nil
	default:
		return false, fmt.Errorf("unexpected easy response: %q", string(output))
	}
}

func buildEasyPrompt(task queue.Task, easyDescription string) string {
	return fmt.Sprintf(
		"Do you consider this task to be easy based on: %s\n\n"+
			"Task title: %s\nTask description: %s\n\n"+
			"Respond only with { easy: true } or { easy: false }. Nothing else, nothing more.",
		easyDescription, task.Title, task.Description,
	)
}
