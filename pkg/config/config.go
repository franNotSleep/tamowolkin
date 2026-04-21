package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	LinearAPIKey        string
	LinearWebhookSecret string
	LinearEmail         string
	EasyDescriptionPath string
	PlanPromptPath      string
	CodePromptPath      string
	ProjectPath         string
	PlansDir            string
	WorktreesDir        string
	BaseBranch          string
	Port                int
	WorkerCount         int
}

func Load() Config {
	_ = godotenv.Load()

	cfg := Config{
		LinearAPIKey:        getOrPanic("LINEAR_API_KEY"),
		LinearWebhookSecret: getOrPanic("LINEAR_WEBHOOK_SECRET"),
		LinearEmail:         getOrPanic("LINEAR_EMAIL"),
		EasyDescriptionPath: getDefault("EASY_DESCRIPTION_PATH", "./tamowolkin-easy.md"),
		PlanPromptPath:      getDefault("PLAN_PROMPT_PATH", "./tamowolkin-plan-prompt.md"),
		CodePromptPath:      getDefault("CODE_PROMPT_PATH", "./tamowolkin-code-prompt.md"),
		ProjectPath:         getDefault("PROJECT_PATH", "./"),
		PlansDir:            getDefault("PLANS_DIR", "./tamowolkin-plans"),
		WorktreesDir:        getDefault("WORKTREES_DIR", "./tamowolkin-worktrees"),
		BaseBranch:          getDefault("BASE_BRANCH", "main"),
		WorkerCount:         getIntDefault("WORKER_COUNT", 3),
		Port:                getIntDefault("PORT", 3000),
	}

	return cfg
}

func getOrPanic(key string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	panic(fmt.Sprintf("%s is required", key))
}

func getDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getIntDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
