package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/frannotsleep/tamowolkin/pkg/agents"
	"github.com/frannotsleep/tamowolkin/pkg/config"
	"github.com/frannotsleep/tamowolkin/pkg/queue"
	"github.com/frannotsleep/tamowolkin/pkg/server"
)

const defaultEasyDescription = `A task is considered easy when all of the following hold:
- The goal is clearly stated and the scope is well bounded.
- Changes are contained to a small, isolated part of the codebase (typically one file or module).
- No new dependencies, data migrations, or architectural changes are required.
- Existing patterns in the codebase already cover the work; no novel design decisions are needed.
- The work can reasonably be completed end-to-end in under 30 minutes of focused effort.`

const defaultPlanPrompt = `Read this ticket carefully. Find any gaps or missing context. Give me a plain-language explanation of what needs to happen, then a technical plan broken down step by step — backend first, then frontend. Do not write any code yet.`

const defaultCodePrompt = `Implement the following task in the current working directory. The repository is already checked out on a fresh branch cut from the project's base branch. Make minimal, focused changes that satisfy the task. Run the project's tests or build if they exist. When you are finished, commit your changes with a short, descriptive message that starts with the task identifier.`

func main() {
	cfg := config.Load()

	easyDescription := loadEasyDescription(cfg.EasyDescriptionPath)
	planPrompt := loadPlanPrompt(cfg.PlanPromptPath)
	codePrompt := loadCodePrompt(cfg.CodePromptPath)

	if err := os.MkdirAll(cfg.PlansDir, 0o755); err != nil {
		log.Fatalf("create plans dir %s: %v", cfg.PlansDir, err)
	}
	if err := os.MkdirAll(cfg.WorktreesDir, 0o755); err != nil {
		log.Fatalf("create worktrees dir %s: %v", cfg.WorktreesDir, err)
	}

	jobsQueue := queue.NewQueue()
	agentsPool := agents.NewAgentPool(agents.AgentPoolConfig{
		WorkerCount:     cfg.WorkerCount,
		Queue:           jobsQueue,
		EasyDescription: easyDescription,
		PlanPrompt:      planPrompt,
		CodePrompt:      codePrompt,
		ProjectPath:     cfg.ProjectPath,
		PlansDir:        cfg.PlansDir,
		WorktreesDir:    cfg.WorktreesDir,
		BaseBranch:      cfg.BaseBranch,
	})

	agentsPool.Start()
	fmt.Printf("[tamowolkin] Starting a pool with %d agents\n", cfg.WorkerCount)

	fmt.Printf("[tamowolkin] Listening on :%d\n", cfg.Port)
	srv := server.NewServer(cfg.Port, cfg.LinearWebhookSecret, jobsQueue, cfg.LinearEmail)
	if err := srv.StartServer(); err != nil {
		log.Fatal(err)
	}
}

func loadEasyDescription(path string) string {
	content, err := os.ReadFile(path)
	if err == nil {
		fmt.Printf("[tamowolkin] Using easy description from %s\n", path)
		return string(content)
	}
	if !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("read easy description %s: %v", path, err)
	}
	fmt.Printf("[tamowolkin] %s not found, using built-in default easy description\n", path)
	return defaultEasyDescription
}

func loadPlanPrompt(path string) string {
	content, err := os.ReadFile(path)
	if err == nil {
		fmt.Printf("[tamowolkin] Using plan prompt from %s\n", path)
		return string(content)
	}
	if !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("read plan prompt %s: %v", path, err)
	}
	fmt.Printf("[tamowolkin] %s not found, using built-in default plan prompt\n", path)
	return defaultPlanPrompt
}

func loadCodePrompt(path string) string {
	content, err := os.ReadFile(path)
	if err == nil {
		fmt.Printf("[tamowolkin] Using code prompt from %s\n", path)
		return string(content)
	}
	if !errors.Is(err, os.ErrNotExist) {
		log.Fatalf("read code prompt %s: %v", path, err)
	}
	fmt.Printf("[tamowolkin] %s not found, using built-in default code prompt\n", path)
	return defaultCodePrompt
}
