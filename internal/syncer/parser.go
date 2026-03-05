package syncer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fsamin/phoebus/internal/logging"
	"sigs.k8s.io/yaml"
)

// --- phoebus.yaml ---

type phoebusMeta struct {
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	Icon              string   `json:"icon"`
	Tags              []string `json:"tags"`
	EstimatedDuration string   `json:"estimated_duration"`
	Prerequisites     []string `json:"prerequisites"`
}

func parsePhoebus(ctx context.Context, repoDir string) (*phoebusMeta, error) {
	logger := logging.FromContext(ctx)
	data, err := os.ReadFile(filepath.Join(repoDir, "phoebus.yaml"))
	if err != nil {
		return nil, fmt.Errorf("phoebus.yaml not found: %w", err)
	}
	var meta phoebusMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("invalid phoebus.yaml: %w", err)
	}
	if meta.Title == "" {
		return nil, fmt.Errorf("phoebus.yaml: title is required")
	}
	if meta.Description == "" {
		logger.Warn("phoebus.yaml: description is empty", "title", meta.Title)
	}
	if len(meta.Tags) == 0 {
		logger.Warn("phoebus.yaml: no tags defined", "title", meta.Title)
	}
	logger.Debug("parsed phoebus.yaml", "title", meta.Title, "tags", len(meta.Tags))
	return &meta, nil
}

// --- Module index.md ---

type moduleMeta struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Competencies []string `json:"competencies"`
}

func parseModuleIndex(ctx context.Context, moduleDir string) (*moduleMeta, error) {
	logger := logging.FromContext(ctx)
	data, err := os.ReadFile(filepath.Join(moduleDir, "index.md"))
	if err != nil {
		return nil, fmt.Errorf("index.md not found: %w", err)
	}
	fm, _, err := splitFrontMatter(string(data))
	if err != nil {
		return nil, err
	}
	if fm == "" {
		logger.Warn("index.md: no front matter found", "dir", filepath.Base(moduleDir))
	}
	var meta moduleMeta
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("invalid front matter in index.md: %w", err)
	}
	if meta.Title == "" {
		return nil, fmt.Errorf("index.md: title is required")
	}
	logger.Debug("parsed module index", "title", meta.Title, "competencies", len(meta.Competencies))
	return &meta, nil
}

// --- Step files ---

type stepMeta struct {
	Title    string `json:"title"`
	Type     string `json:"type"`
	Duration string `json:"estimated_duration"`
	// Code exercise fields
	Mode   string      `json:"mode"`
	Target interface{} `json:"target"`
}

func parseStep(ctx context.Context, stepPath string) (*stepMeta, string, json.RawMessage, error) {
	logger := logging.FromContext(ctx)
	data, err := os.ReadFile(stepPath)
	if err != nil {
		return nil, "", nil, err
	}

	fm, body, err := splitFrontMatter(string(data))
	if err != nil {
		return nil, "", nil, fmt.Errorf("parse front matter in %s: %w", filepath.Base(stepPath), err)
	}

	var meta stepMeta
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, "", nil, fmt.Errorf("invalid front matter in %s: %w", filepath.Base(stepPath), err)
	}
	if meta.Title == "" || meta.Type == "" {
		return nil, "", nil, fmt.Errorf("title and type are required in front matter of %s", filepath.Base(stepPath))
	}
	if meta.Duration == "" {
		logger.Warn("step missing estimated_duration", "file", filepath.Base(stepPath), "title", meta.Title)
	}

	// Parse exercise data from body based on type
	var exerciseData json.RawMessage
	switch meta.Type {
	case "lesson":
		// No exercise data
	case "quiz":
		exerciseData, err = parseQuizBody(ctx, body)
	case "terminal-exercise":
		exerciseData, err = parseTerminalBody(ctx, body)
	case "code-exercise":
		exerciseData, err = parseCodeExerciseBody(ctx, body, meta.Mode, meta.Target)
	default:
		return nil, "", nil, fmt.Errorf("unknown step type %q in %s", meta.Type, filepath.Base(stepPath))
	}
	if err != nil {
		return nil, "", nil, fmt.Errorf("parse %s body in %s: %w", meta.Type, filepath.Base(stepPath), err)
	}

	logger.Debug("parsed step", "file", filepath.Base(stepPath), "title", meta.Title, "type", meta.Type)
	return &meta, body, exerciseData, nil
}

// --- Front matter split ---

func splitFrontMatter(content string) (string, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}
	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) < 2 {
		return "", content, fmt.Errorf("unterminated front matter")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

// --- Quiz parser ---

func parseQuizBody(ctx context.Context, body string) (json.RawMessage, error) {
	logger := logging.FromContext(ctx)
	sections := splitOnH2(body)
	if len(sections) == 0 {
		return nil, fmt.Errorf("no questions found")
	}

	var questions []map[string]any
	for _, section := range sections {
		q, err := parseQuizQuestion(section.tag, section.title, section.body)
		if err != nil {
			return nil, err
		}
		if q["explanation"] == "" {
			logger.Warn("quiz question missing explanation", "question", section.title)
		}
		questions = append(questions, q)
	}

	logger.Debug("parsed quiz", "questions", len(questions))
	return json.Marshal(map[string]any{"questions": questions})
}

func parseQuizQuestion(tag, title, body string) (map[string]any, error) {
	q := map[string]any{
		"text": title,
		"type": tag,
	}

	switch tag {
	case "multiple-choice":
		var choices []map[string]any
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- [x] ") {
				choices = append(choices, map[string]any{"text": strings.TrimPrefix(line, "- [x] "), "correct": true})
			} else if strings.HasPrefix(line, "- [ ] ") {
				choices = append(choices, map[string]any{"text": strings.TrimPrefix(line, "- [ ] "), "correct": false})
			}
		}
		if len(choices) == 0 {
			return nil, fmt.Errorf("no choices found for question: %s", title)
		}
		correctCount := 0
		for _, c := range choices {
			if c["correct"] == true {
				correctCount++
			}
		}
		q["choices"] = choices
		q["multi_select"] = correctCount > 1

	case "short-answer":
		// Find indented code block (pattern)
		lines := strings.Split(body, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t") {
				pattern := strings.TrimSpace(line)
				// Validate regex at parse-time (E6)
				if _, err := regexp.Compile(pattern); err != nil {
					return nil, fmt.Errorf("invalid regex pattern %q for question %q: %w", pattern, title, err)
				}
				q["pattern"] = pattern
				break
			}
		}
		if q["pattern"] == nil {
			return nil, fmt.Errorf("no answer pattern found for question: %s", title)
		}
	}

	// Extract explanation (blockquote)
	q["explanation"] = extractBlockquote(body)

	return q, nil
}

// --- Terminal exercise parser ---

func parseTerminalBody(ctx context.Context, body string) (json.RawMessage, error) {
	logger := logging.FromContext(ctx)

	// Split on ## Step headings
	stepRegex := regexp.MustCompile(`(?mi)^## Step (\d+)`)
	indices := stepRegex.FindAllStringIndex(body, -1)

	var intro string
	if len(indices) > 0 {
		intro = strings.TrimSpace(body[:indices[0][0]])
	}
	if intro == "" {
		logger.Warn("terminal exercise has no introduction")
	}

	var steps []map[string]any
	for i, idx := range indices {
		var sectionBody string
		if i+1 < len(indices) {
			sectionBody = body[idx[1]:indices[i+1][0]]
		} else {
			sectionBody = body[idx[1]:]
		}
		sectionBody = strings.TrimSpace(sectionBody)

		step, err := parseTerminalStep(sectionBody)
		if err != nil {
			return nil, fmt.Errorf("step %d: %w", i+1, err)
		}
		steps = append(steps, step)
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("no steps found")
	}

	logger.Debug("parsed terminal exercise", "steps", len(steps))
	return json.Marshal(map[string]any{
		"introduction": intro,
		"steps":        steps,
	})
}

func parseTerminalStep(body string) (map[string]any, error) {
	step := map[string]any{}

	// Extract context (text before first code block or checkbox)
	contextEnd := len(body)
	if idx := strings.Index(body, "```"); idx >= 0 && idx < contextEnd {
		contextEnd = idx
	}
	if idx := strings.Index(body, "- ["); idx >= 0 && idx < contextEnd {
		contextEnd = idx
	}
	step["context"] = strings.TrimSpace(body[:contextEnd])

	// Extract console prompt
	step["prompt"] = extractCodeBlock(body, "console")

	// Extract output
	step["output"] = extractCodeBlock(body, "output")

	// Extract proposals
	var proposals []map[string]any
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		correct := false
		var rest string
		if strings.HasPrefix(line, "- [x] ") {
			correct = true
			rest = strings.TrimPrefix(line, "- [x] ")
		} else if strings.HasPrefix(line, "- [ ] ") {
			rest = strings.TrimPrefix(line, "- [ ] ")
		} else {
			continue
		}

		// Extract command from backticks
		cmd, explanation := parseProposalLine(rest)
		proposals = append(proposals, map[string]any{
			"command":     cmd,
			"correct":     correct,
			"explanation": explanation,
		})
	}

	correctCount := 0
	for _, p := range proposals {
		if p["correct"] == true {
			correctCount++
		}
	}
	if correctCount != 1 {
		return nil, fmt.Errorf("expected exactly 1 correct command, got %d", correctCount)
	}

	step["proposals"] = proposals
	return step, nil
}

// --- Code exercise parser ---

func parseCodeExerciseBody(ctx context.Context, body, mode string, target interface{}) (json.RawMessage, error) {
	logger := logging.FromContext(ctx)

	// Split on ## Patches
	patchIdx := strings.Index(body, "## Patches")
	if patchIdx < 0 {
		return nil, fmt.Errorf("## Patches section not found")
	}

	description := strings.TrimSpace(body[:patchIdx])
	if description == "" {
		logger.Warn("code exercise has no description")
	}
	patchesBody := body[patchIdx:]

	// Parse patches
	sections := splitOnH3Checkbox(patchesBody)
	var patches []map[string]any
	correctCount := 0
	for _, section := range sections {
		patch := map[string]any{
			"label":       section.title,
			"correct":     section.correct,
			"explanation": extractNonCodeText(section.body),
			"diff":        extractCodeBlock(section.body, "diff"),
		}
		if section.correct {
			correctCount++
		}
		patches = append(patches, patch)
	}

	if correctCount != 1 {
		return nil, fmt.Errorf("expected exactly 1 correct patch, got %d", correctCount)
	}

	logger.Debug("parsed code exercise", "patches", len(patches))

	result := map[string]any{
		"mode":        mode,
		"description": description,
		"patches":     patches,
	}
	if target != nil {
		result["target"] = convertYAMLToJSON(target)
	}

	return json.Marshal(result)
}

// --- Shared parsing helpers ---

type h2Section struct {
	tag   string
	title string
	body  string
}

func splitOnH2(body string) []h2Section {
	re := regexp.MustCompile(`(?m)^## \[([^\]]+)\]\s*(.*)$`)
	matches := re.FindAllStringSubmatchIndex(body, -1)

	var sections []h2Section
	for i, match := range matches {
		tag := body[match[2]:match[3]]
		title := body[match[4]:match[5]]
		var sectionBody string
		if i+1 < len(matches) {
			sectionBody = body[match[1]:matches[i+1][0]]
		} else {
			sectionBody = body[match[1]:]
		}
		sections = append(sections, h2Section{tag: tag, title: strings.TrimSpace(title), body: trimBlankLines(sectionBody)})
	}
	return sections
}

type h3CheckboxSection struct {
	correct bool
	title   string
	body    string
}

func splitOnH3Checkbox(body string) []h3CheckboxSection {
	re := regexp.MustCompile(`(?m)^### \[([ x])\]\s*(.*)$`)
	matches := re.FindAllStringSubmatchIndex(body, -1)

	var sections []h3CheckboxSection
	for i, match := range matches {
		correct := body[match[2]:match[3]] == "x"
		title := body[match[4]:match[5]]
		var sectionBody string
		if i+1 < len(matches) {
			sectionBody = body[match[1]:matches[i+1][0]]
		} else {
			sectionBody = body[match[1]:]
		}
		sections = append(sections, h3CheckboxSection{correct: correct, title: strings.TrimSpace(title), body: strings.TrimSpace(sectionBody)})
	}
	return sections
}

func extractCodeBlock(body, lang string) string {
	marker := "```" + lang
	start := strings.Index(body, marker)
	if start < 0 {
		return ""
	}
	start += len(marker)
	// Skip to end of line
	if nl := strings.IndexByte(body[start:], '\n'); nl >= 0 {
		start += nl + 1
	}
	end := strings.Index(body[start:], "```")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(body[start : start+end])
}

func extractBlockquote(body string) string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "> ") {
			lines = append(lines, strings.TrimPrefix(line, "> "))
		} else if strings.HasPrefix(line, ">") {
			lines = append(lines, strings.TrimPrefix(line, ">"))
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractNonCodeText(body string) string {
	var lines []string
	inCode := false
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			continue
		}
		if !inCode {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// trimBlankLines removes leading and trailing blank lines but preserves
// indentation on content lines (unlike strings.TrimSpace).
func trimBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	start, end := 0, len(lines)-1
	for start <= end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end >= start && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if start > end {
		return ""
	}
	return strings.Join(lines[start:end+1], "\n")
}

func parseProposalLine(rest string) (string, string) {
	// Extract command from first backtick pair
	start := strings.IndexByte(rest, '`')
	if start < 0 {
		return rest, ""
	}
	end := strings.IndexByte(rest[start+1:], '`')
	if end < 0 {
		return rest, ""
	}
	cmd := rest[start+1 : start+1+end]

	// Extract explanation after em dash
	explanation := ""
	if idx := strings.Index(rest, " — "); idx >= 0 {
		explanation = strings.TrimSpace(rest[idx+len(" — "):])
	}
	return cmd, explanation
}

// convertYAMLToJSON recursively converts map[interface{}]interface{} to map[string]interface{}
// so it can be marshalled to JSON.
func convertYAMLToJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, v := range val {
			m[fmt.Sprintf("%v", k)] = convertYAMLToJSON(v)
		}
		return m
	case []interface{}:
		for i, item := range val {
			val[i] = convertYAMLToJSON(item)
		}
		return val
	default:
		return v
	}
}
