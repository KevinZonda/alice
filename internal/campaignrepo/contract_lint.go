package campaignrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	contractTaskIDPattern   = regexp.MustCompile(`\bT[0-9]{3,}\b`)
	contractPhaseIDPattern  = regexp.MustCompile(`\bP[0-9]{2,}\b`)
	contractBacktickPattern = regexp.MustCompile("`([^`]+)`")
	contractWordPattern     = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_./-]{3,}`)
	contractPathPattern     = regexp.MustCompile(`(?:[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]*`)
)

type masterPlanTaskContract struct {
	TaskID          string
	PhaseID         string
	Path            string
	DependsOn       []string
	TargetRepos     []string
	WriteScope      []string
	AcceptanceFocus string
}

type masterPlanPhaseContract struct {
	PhaseID     string
	Path        string
	Goal        string
	TaskIDs     []string
	Parallelism string
}

type phaseGateContract struct {
	TaskID      string
	Description string
}

func runtimeRepositoryIssues(repo Repository) []ValidationIssue {
	issues := append([]ValidationIssue(nil), repo.LoadIssues...)
	issues = append(issues, planningSelfCheckIssues(repo)...)
	issues = append(issues, contractConsistencyIssues(repo)...)
	sortValidationIssues(issues)
	return issues
}

func contractConsistencyIssues(repo Repository) []ValidationIssue {
	if strings.TrimSpace(repo.Root) == "" {
		return nil
	}

	masterPlanPath := filepath.Join(repo.Root, "plans", "merged", "master-plan.md")
	raw, err := os.ReadFile(masterPlanPath)
	if err != nil {
		return nil
	}
	parsed := parseMarkdownFrontmatter(string(raw))
	taskContracts := parseMasterPlanTaskContracts(repo.Root, parsed.Body)
	phaseContracts := parseMasterPlanPhaseContracts(repo.Root, parsed.Body)
	if len(taskContracts) == 0 && len(phaseContracts) == 0 {
		return nil
	}

	taskByID := make(map[string]TaskDocument, len(repo.Tasks))
	tasksByPhase := make(map[string][]string, len(repo.Phases))
	for _, task := range repo.Tasks {
		taskID := strings.TrimSpace(task.Frontmatter.TaskID)
		if taskID == "" {
			continue
		}
		taskByID[taskID] = task
		phaseID := strings.TrimSpace(task.Frontmatter.Phase)
		if phaseID != "" {
			tasksByPhase[phaseID] = append(tasksByPhase[phaseID], taskID)
		}
	}
	for phaseID := range tasksByPhase {
		tasksByPhase[phaseID] = normalizeTaskIDList(tasksByPhase[phaseID])
	}

	phaseByID := make(map[string]PhaseDocument, len(repo.Phases))
	for _, phase := range repo.Phases {
		phaseID := strings.TrimSpace(phase.Frontmatter.Phase)
		if phaseID == "" {
			continue
		}
		phaseByID[phaseID] = phase
	}

	var issues []ValidationIssue
	for taskID, contract := range taskContracts {
		task, ok := taskByID[taskID]
		if !ok {
			issues = append(issues, ValidationIssue{
				Code:    "master_plan_task_missing_doc",
				Path:    contract.Path,
				Message: fmt.Sprintf("master-plan task %s does not have a matching task package", taskID),
			})
			continue
		}
		if contract.PhaseID != "" && !strings.EqualFold(contract.PhaseID, strings.TrimSpace(task.Frontmatter.Phase)) {
			issues = append(issues, ValidationIssue{
				Code:    "task_contract_phase_mismatch",
				Path:    task.Path,
				Message: fmt.Sprintf("task %s phase %s does not match master-plan phase %s", taskID, blankForSummary(task.Frontmatter.Phase), contract.PhaseID),
			})
		}
		if len(contract.DependsOn) > 0 && !sameNormalizedStringSet(contract.DependsOn, task.Frontmatter.DependsOn) {
			issues = append(issues, ValidationIssue{
				Code:    "task_contract_depends_on_mismatch",
				Path:    task.Path,
				Message: fmt.Sprintf("task %s depends_on %v does not match master-plan %v", taskID, normalizeTaskIDList(task.Frontmatter.DependsOn), contract.DependsOn),
			})
		}
		if len(contract.TargetRepos) > 0 && !sameNormalizedStringSet(contract.TargetRepos, task.Frontmatter.TargetRepos) {
			issues = append(issues, ValidationIssue{
				Code:    "task_contract_target_repos_mismatch",
				Path:    task.Path,
				Message: fmt.Sprintf("task %s target_repos %v does not match master-plan %v", taskID, normalizeStringList(task.Frontmatter.TargetRepos), contract.TargetRepos),
			})
		}
		if len(contract.WriteScope) > 0 && !sameNormalizedScopeSet(contract.WriteScope, task.Frontmatter.WriteScope) {
			issues = append(issues, ValidationIssue{
				Code:    "task_contract_write_scope_mismatch",
				Path:    task.Path,
				Message: fmt.Sprintf("task %s write_scope %v does not match master-plan %v", taskID, normalizeScopes(task.Frontmatter.WriteScope), normalizeScopes(contract.WriteScope)),
			})
		}
		if contract.AcceptanceFocus != "" {
			acceptance := markdownSectionContent(task.Body, "Acceptance")
			if !contractNarrativeAligned(contract.AcceptanceFocus, acceptance) {
				issues = append(issues, ValidationIssue{
					Code:    "task_contract_acceptance_mismatch",
					Path:    task.Path,
					Message: fmt.Sprintf("task %s acceptance section does not reflect the master-plan acceptance focus: %s", taskID, strings.TrimSpace(contract.AcceptanceFocus)),
				})
			}
		}
	}
	for taskID, task := range taskByID {
		if _, ok := taskContracts[taskID]; ok {
			continue
		}
		issues = append(issues, ValidationIssue{
			Code:    "task_missing_from_master_plan",
			Path:    task.Path,
			Message: fmt.Sprintf("task %s is missing from plans/merged/master-plan.md task breakdown", taskID),
		})
	}

	for phaseID, contract := range phaseContracts {
		phase, ok := phaseByID[phaseID]
		if !ok {
			issues = append(issues, ValidationIssue{
				Code:    "master_plan_phase_missing_doc",
				Path:    contract.Path,
				Message: fmt.Sprintf("master-plan phase %s does not have a matching phase.md", phaseID),
			})
			continue
		}
		if contract.Goal != "" {
			actualGoal := strings.TrimSpace(strings.Join([]string{
				phase.Frontmatter.Goal,
				markdownSectionContent(phase.Body, "Goal"),
			}, "\n"))
			if !contractNarrativeAligned(contract.Goal, actualGoal) {
				issues = append(issues, ValidationIssue{
					Code:    "phase_goal_contract_mismatch",
					Path:    phase.Path,
					Message: fmt.Sprintf("phase %s goal does not align with master-plan phase contract: %s", phaseID, strings.TrimSpace(contract.Goal)),
				})
			}
		}
		if len(contract.TaskIDs) > 0 {
			actualTaskIDs := normalizeTaskIDList(tasksByPhase[phaseID])
			if !sameNormalizedStringSet(contract.TaskIDs, actualTaskIDs) {
				issues = append(issues, ValidationIssue{
					Code:    "phase_task_set_mismatch",
					Path:    phase.Path,
					Message: fmt.Sprintf("phase %s task set %v does not match master-plan %v", phaseID, actualTaskIDs, contract.TaskIDs),
				})
			}
			phaseRefs := normalizeTaskIDList(phaseTaskReferences(phase))
			if len(phaseRefs) > 0 && !sameNormalizedStringSet(contract.TaskIDs, phaseRefs) {
				issues = append(issues, ValidationIssue{
					Code:    "phase_task_refs_mismatch",
					Path:    phase.Path,
					Message: fmt.Sprintf("phase %s references task set %v but master-plan declares %v", phaseID, phaseRefs, contract.TaskIDs),
				})
			}
		}
		if contract.Parallelism != "" {
			phaseParallelism := phaseParallelismContract(phase)
			if phaseParallelism != "" && normalizeContractText(contract.Parallelism) != normalizeContractText(phaseParallelism) {
				issues = append(issues, ValidationIssue{
					Code:    "phase_parallelism_mismatch",
					Path:    phase.Path,
					Message: fmt.Sprintf("phase %s parallelism %q does not match master-plan %q", phaseID, phaseParallelism, contract.Parallelism),
				})
			}
		}
		for _, gate := range phaseExitGateContracts(phase) {
			task, ok := taskByID[gate.TaskID]
			if !ok || strings.TrimSpace(gate.Description) == "" {
				continue
			}
			if !contractNarrativeAligned(gate.Description, markdownSectionContent(task.Body, "Acceptance")) {
				issues = append(issues, ValidationIssue{
					Code:    "phase_exit_gate_acceptance_mismatch",
					Path:    task.Path,
					Message: fmt.Sprintf("task %s acceptance section does not reflect phase %s exit gate: %s", gate.TaskID, phaseID, strings.TrimSpace(gate.Description)),
				})
			}
		}
	}

	sortValidationIssues(issues)
	return issues
}

func parseMasterPlanTaskContracts(root, body string) map[string]masterPlanTaskContract {
	section := markdownSectionContent(body, "Task Breakdown")
	if isPlaceholderText(section) {
		section = markdownSectionContent(body, "Tasks")
	}
	if isPlaceholderText(section) {
		return nil
	}
	path := filepath.ToSlash(relativePath(root, filepath.Join(root, "plans", "merged", "master-plan.md")))
	contracts := make(map[string]masterPlanTaskContract)
	for _, block := range topLevelBulletBlocks(section) {
		taskID := firstContractTaskID(block)
		if taskID == "" {
			continue
		}
		contract := masterPlanTaskContract{
			TaskID:          taskID,
			PhaseID:         firstContractPhaseID(block),
			Path:            path,
			DependsOn:       parseContractDependsOn(block),
			TargetRepos:     parseContractListField(block, "target repos"),
			WriteScope:      parseContractWriteScope(block),
			AcceptanceFocus: parseContractTextField(block, "acceptance focus"),
		}
		if existing, ok := contracts[taskID]; ok {
			if contract.PhaseID == "" {
				contract.PhaseID = existing.PhaseID
			}
			if len(contract.DependsOn) == 0 {
				contract.DependsOn = existing.DependsOn
			}
			if len(contract.TargetRepos) == 0 {
				contract.TargetRepos = existing.TargetRepos
			}
			if len(contract.WriteScope) == 0 {
				contract.WriteScope = existing.WriteScope
			}
			if contract.AcceptanceFocus == "" {
				contract.AcceptanceFocus = existing.AcceptanceFocus
			}
		}
		contracts[taskID] = contract
	}
	return contracts
}

func parseMasterPlanPhaseContracts(root, body string) map[string]masterPlanPhaseContract {
	section := markdownSectionContent(body, "Phases")
	if isPlaceholderText(section) {
		return nil
	}
	path := filepath.ToSlash(relativePath(root, filepath.Join(root, "plans", "merged", "master-plan.md")))
	contracts := make(map[string]masterPlanPhaseContract)
	for _, block := range topLevelBulletBlocks(section) {
		phaseID := firstContractPhaseID(block)
		if phaseID == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		header := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "- "))
		goal := ""
		if _, after, ok := strings.Cut(header, "："); ok {
			goal = strings.TrimSpace(after)
		} else if _, after, ok := strings.Cut(header, ":"); ok {
			goal = strings.TrimSpace(after)
		}
		contracts[phaseID] = masterPlanPhaseContract{
			PhaseID:     phaseID,
			Path:        path,
			Goal:        goal,
			TaskIDs:     normalizeTaskIDList(parseContractInlineTaskIDs(parseContractTextField(block, "tasks"))),
			Parallelism: parseContractTextField(block, "parallelism"),
		}
	}
	return contracts
}

func phaseTaskReferences(phase PhaseDocument) []string {
	var refs []string
	refs = append(refs, contractTaskIDPattern.FindAllString(strings.Join(phase.Frontmatter.ExitGates, "\n"), -1)...)
	refs = append(refs, contractTaskIDPattern.FindAllString(markdownSectionContent(phase.Body, "Exit Gates"), -1)...)
	refs = append(refs, contractTaskIDPattern.FindAllString(markdownSectionContent(phase.Body, "Notes"), -1)...)
	return normalizeTaskIDList(refs)
}

func phaseParallelismContract(phase PhaseDocument) string {
	notes := markdownSectionContent(phase.Body, "Notes")
	for _, line := range strings.Split(notes, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		switch {
		case strings.HasPrefix(trimmed, "并行组："):
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "并行组："))
		case strings.HasPrefix(strings.ToLower(trimmed), "parallelism:"):
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "parallelism:"))
		}
	}
	return ""
}

func phaseExitGateContracts(phase PhaseDocument) []phaseGateContract {
	var gates []phaseGateContract
	for _, raw := range phase.Frontmatter.ExitGates {
		taskIDs := normalizeTaskIDList(contractTaskIDPattern.FindAllString(raw, -1))
		if len(taskIDs) != 1 {
			continue
		}
		gates = append(gates, phaseGateContract{
			TaskID:      taskIDs[0],
			Description: strings.TrimSpace(raw),
		})
	}
	if len(gates) > 0 {
		return gates
	}
	for _, line := range strings.Split(markdownSectionContent(phase.Body, "Exit Gates"), "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		taskIDs := normalizeTaskIDList(contractTaskIDPattern.FindAllString(trimmed, -1))
		if len(taskIDs) != 1 {
			continue
		}
		gates = append(gates, phaseGateContract{
			TaskID:      taskIDs[0],
			Description: trimmed,
		})
	}
	return gates
}

func topLevelBulletBlocks(section string) []string {
	lines := strings.Split(strings.TrimSpace(section), "\n")
	var (
		blocks []string
		buf    []string
	)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		blocks = append(blocks, strings.TrimSpace(strings.Join(buf, "\n")))
		buf = nil
	}
	for _, line := range lines {
		if line == "" {
			if len(buf) > 0 {
				buf = append(buf, line)
			}
			continue
		}
		leading := len(line) - len(strings.TrimLeft(line, " \t"))
		trimmed := strings.TrimSpace(line)
		if leading == 0 && strings.HasPrefix(trimmed, "- ") {
			flush()
			buf = append(buf, trimmed)
			continue
		}
		if len(buf) > 0 {
			buf = append(buf, line)
		}
	}
	flush()
	return blocks
}

func parseContractDependsOn(block string) []string {
	value := parseContractTextField(block, "depends on")
	if value == "" {
		value = parseInlineLabeledValue(block, "depends_on")
	}
	return normalizeTaskIDList(contractTaskIDPattern.FindAllString(value, -1))
}

func parseContractWriteScope(block string) []string {
	value := parseContractTextField(block, "write scope")
	if value == "" {
		value = parseInlineLabeledValue(block, "write_scope")
	}
	return parseContractListFromText(value)
}

func parseContractListField(block, label string) []string {
	value := parseContractTextField(block, label)
	if value == "" {
		value = parseInlineLabeledValue(block, strings.ReplaceAll(label, " ", "_"))
	}
	return parseContractListFromText(value)
}

func parseContractTextField(block, label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		lower := strings.ToLower(trimmed)
		if !strings.HasPrefix(lower, label+":") {
			continue
		}
		return strings.TrimSpace(trimmed[len(label)+1:])
	}
	return ""
}

func parseInlineLabeledValue(block, label string) string {
	lower := strings.ToLower(block)
	label = strings.ToLower(strings.TrimSpace(label))
	index := strings.Index(lower, label+":")
	if index < 0 {
		return ""
	}
	rest := block[index+len(label)+1:]
	for _, sep := range []string{"；", ";", "\n"} {
		if cut, _, ok := strings.Cut(rest, sep); ok {
			return strings.TrimSpace(cut)
		}
	}
	return strings.TrimSpace(rest)
}

func parseContractListFromText(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.Contains(value, "[") && strings.Contains(value, "]") {
		if _, list, ok := strings.Cut(value, "["); ok {
			if inner, _, ok := strings.Cut(list, "]"); ok {
				value = inner
			}
		}
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '\n':
			return true
		default:
			return false
		}
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		candidate := strings.Trim(strings.TrimSpace(part), "`")
		if candidate == "" {
			continue
		}
		out = append(out, candidate)
	}
	return normalizeStringList(out)
}

func parseContractInlineTaskIDs(value string) []string {
	return contractTaskIDPattern.FindAllString(value, -1)
}

func firstContractTaskID(value string) string {
	match := contractTaskIDPattern.FindString(value)
	return strings.TrimSpace(match)
}

func firstContractPhaseID(value string) string {
	match := contractPhaseIDPattern.FindString(value)
	return strings.TrimSpace(match)
}

func normalizeTaskIDList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		trimmed := strings.ToUpper(strings.TrimSpace(strings.Trim(raw, "`")))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func sameNormalizedStringSet(left, right []string) bool {
	leftNorm := normalizeRepoKeys(left)
	rightNorm := normalizeRepoKeys(right)
	sort.Strings(leftNorm)
	sort.Strings(rightNorm)
	if len(leftNorm) != len(rightNorm) {
		return false
	}
	for idx := range leftNorm {
		if leftNorm[idx] != rightNorm[idx] {
			return false
		}
	}
	return true
}

func sameNormalizedScopeSet(left, right []string) bool {
	leftNorm := normalizeScopes(left)
	rightNorm := normalizeScopes(right)
	if len(leftNorm) != len(rightNorm) {
		return false
	}
	sort.Strings(leftNorm)
	sort.Strings(rightNorm)
	for idx := range leftNorm {
		if leftNorm[idx] != rightNorm[idx] {
			return false
		}
	}
	return true
}

func contractNarrativeAligned(expected, actual string) bool {
	expected = normalizeContractText(expected)
	actual = normalizeContractText(actual)
	switch {
	case expected == "":
		return true
	case actual == "":
		return false
	case strings.Contains(actual, expected), strings.Contains(expected, actual):
		return true
	}

	signals := contractSignals(expected)
	if len(signals) == 0 {
		return true
	}

	required := len(signals)
	if required > 2 {
		required = 2
	}
	matched := 0
	for _, signal := range signals {
		if strings.Contains(actual, signal) {
			matched++
		}
	}
	return matched >= required
}

func contractSignals(value string) []string {
	var out []string
	for _, match := range contractBacktickPattern.FindAllStringSubmatch(value, -1) {
		if len(match) < 2 {
			continue
		}
		out = append(out, normalizeContractText(match[1]))
	}
	for _, match := range contractPathPattern.FindAllString(value, -1) {
		out = append(out, normalizeContractText(match))
	}
	for _, match := range contractWordPattern.FindAllString(value, -1) {
		out = append(out, normalizeContractText(match))
	}
	out = normalizeStringList(out)
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

func normalizeContractText(value string) string {
	replacer := strings.NewReplacer(
		"`", " ",
		"“", " ",
		"”", " ",
		"‘", " ",
		"’", " ",
		"（", " ",
		"）", " ",
		"：", " ",
		"；", " ",
		"，", " ",
		"。", " ",
		"[", " ",
		"]", " ",
		"(", " ",
		")", " ",
	)
	value = replacer.Replace(strings.ToLower(strings.TrimSpace(value)))
	return strings.Join(strings.Fields(value), " ")
}

func sortValidationIssues(issues []ValidationIssue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		if issues[i].Path != issues[j].Path {
			return issues[i].Path < issues[j].Path
		}
		return issues[i].Message < issues[j].Message
	})
}
