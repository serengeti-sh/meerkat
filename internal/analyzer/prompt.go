package analyzer

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrSystemPromptNotConfigured = errors.New("system prompt file path not configured")

// Skill represents a prompt-based skill that provides additional instructions to the AI.
type Skill struct {
	Name   string `yaml:"name"`
	Prompt string `yaml:"prompt"`
}

// LoadSystemPrompt loads the system prompt from the given file path.
// Returns an error if the path is empty or the file cannot be read.
func LoadSystemPrompt(customPath string) (string, error) {
	if customPath == "" {
		return "", ErrSystemPromptNotConfigured
	}

	data, err := os.ReadFile(customPath)
	if err != nil {
		return "", fmt.Errorf("failed to read system prompt file %q: %w", customPath, err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return "", fmt.Errorf("system prompt file %q is empty", customPath)
	}

	return trimmed, nil
}

// MustLoadSystemPrompt loads the system prompt and panics on error.
// Use this during initialization to fail fast if the prompt is not configured.
func MustLoadSystemPrompt(customPath string) string {
	prompt, err := LoadSystemPrompt(customPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load system prompt: %v", err))
	}
	return prompt
}

// LoadSkills loads skills from a YAML file.
// Returns nil if the path is empty or the file doesn't exist.
func LoadSkills(skillsPath string) ([]Skill, error) {
	if skillsPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(skillsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read skills file %q: %w", skillsPath, err)
	}

	var skills []Skill
	if err := yaml.Unmarshal(data, &skills); err != nil {
		return nil, fmt.Errorf("failed to parse skills file %q: %w", skillsPath, err)
	}

	return skills, nil
}

// MustLoadSkills loads skills and panics on error.
func MustLoadSkills(skillsPath string) []Skill {
	skills, err := LoadSkills(skillsPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load skills: %v", err))
	}
	return skills
}

// MergeSkillsIntoPrompt appends skills to the system prompt.
// If no skills are provided, returns the original prompt.
func MergeSkillsIntoPrompt(systemPrompt string, skills []Skill) string {
	if len(skills) == 0 {
		return systemPrompt
	}

	var sb strings.Builder
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n## Available Skills\n\n")
	sb.WriteString("You have access to the following skills for specific tasks:\n\n")

	for i, skill := range skills {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "### %s\n", skill.Name)
		sb.WriteString(skill.Prompt)
	}

	return sb.String()
}
