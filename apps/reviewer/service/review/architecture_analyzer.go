package review

import (
	"context"
	"regexp"
	"strings"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/internal/events"
	"github.com/pitabwire/util"
)

// PatternArchitectureAnalyzer implements ArchitectureAnalyzer using pattern matching.
type PatternArchitectureAnalyzer struct {
	cfg *appconfig.ReviewerConfig
}

// NewPatternArchitectureAnalyzer creates a new pattern-based architecture analyzer.
func NewPatternArchitectureAnalyzer(cfg *appconfig.ReviewerConfig) *PatternArchitectureAnalyzer {
	return &PatternArchitectureAnalyzer{cfg: cfg}
}

// Analyze performs architecture analysis on the provided code.
func (a *PatternArchitectureAnalyzer) Analyze(ctx context.Context, req *ArchitectureAnalysisRequest) (*events.ArchitectureAssessment, error) {
	log := util.Log(ctx)
	log.Info("starting architecture analysis",
		"file_count", len(req.FileContents),
		"baseline_count", len(req.BaselineContents),
		"patch_count", len(req.Patches),
	)

	assessment := &events.ArchitectureAssessment{
		OverallArchitectureScore:   100,
		ArchitectureStatus:         events.ArchitectureStatusCompliant,
		BreakingChanges:            []events.BreakingChange{},
		DependencyViolations:       []events.DependencyViolation{},
		LayeringViolations:         []events.LayeringViolation{},
		InterfaceChanges:           []events.InterfaceChange{},
		CircularDependencies:       []events.CircularDependency{},
		PatternViolations:          []events.PatternViolation{},
		APIContractViolations:      []events.APIContractViolation{},
		Recommendations:            []events.ArchitectureRecommendation{},
		RequiresArchitectureReview: false,
	}

	// Detect breaking changes by comparing baseline with current
	breakingChanges := a.detectBreakingChanges(req.Patches, req.BaselineContents, req.FileContents)
	assessment.BreakingChanges = breakingChanges

	// Detect dependency violations
	depViolations := a.detectDependencyViolations(req.FileContents, req.Language)
	assessment.DependencyViolations = depViolations

	// Detect layering violations
	layerViolations := a.detectLayeringViolations(req.FileContents, req.Language)
	assessment.LayeringViolations = layerViolations

	// Detect interface changes
	interfaceChanges := a.detectInterfaceChanges(req.Patches, req.BaselineContents, req.FileContents)
	assessment.InterfaceChanges = interfaceChanges

	// Detect pattern violations
	patternViolations := a.detectPatternViolations(req.FileContents, req.Language)
	assessment.PatternViolations = patternViolations

	// Generate recommendations
	recommendations := a.generateRecommendations(assessment)
	assessment.Recommendations = recommendations

	// Calculate architecture score
	assessment.OverallArchitectureScore = a.calculateArchitectureScore(assessment)

	// Determine architecture status
	assessment.ArchitectureStatus = a.determineArchitectureStatus(assessment)

	// Determine if architecture review is required
	assessment.RequiresArchitectureReview, assessment.ArchitectureReviewReason = a.determineArchitectureReviewRequired(assessment)

	log.Info("architecture analysis complete",
		"score", assessment.OverallArchitectureScore,
		"breaking_changes", len(assessment.BreakingChanges),
		"dep_violations", len(assessment.DependencyViolations),
		"layer_violations", len(assessment.LayeringViolations),
		"pattern_violations", len(assessment.PatternViolations),
		"status", assessment.ArchitectureStatus,
	)

	return assessment, nil
}

// detectBreakingChanges detects breaking changes between baseline and current.
func (a *PatternArchitectureAnalyzer) detectBreakingChanges(patches []events.Patch, baseline, current map[string]string) []events.BreakingChange {
	var changes []events.BreakingChange

	// Check for deleted files (potential breaking change)
	for filePath := range baseline {
		if _, exists := current[filePath]; !exists {
			// File was deleted - check if it was an exported API
			if isExportedFile(filePath, baseline[filePath]) {
				changes = append(changes, events.BreakingChange{
					ChangeType:  events.BreakingChangeRemovedAPI,
					Description: "File with exported API was deleted",
					FilePath:    filePath,
					Impact:      "All consumers of this file's exports will break",
					Severity:    events.ReviewIssueSeverityCritical,
				})
			}
		}
	}

	// Check for removed or changed function signatures
	for filePath, currentContent := range current {
		if baselineContent, exists := baseline[filePath]; exists {
			// Compare function signatures
			baselineFuncs := extractFunctionSignatures(baselineContent, filePath)
			currentFuncs := extractFunctionSignatures(currentContent, filePath)

			// Check for removed functions
			for funcName, baseSig := range baselineFuncs {
				if currentSig, found := currentFuncs[funcName]; !found {
					// Function was removed
					if isExportedSymbol(funcName, filePath) {
						changes = append(changes, events.BreakingChange{
							ChangeType:    events.BreakingChangeRemovedAPI,
							Description:   "Exported function was removed",
							FilePath:      filePath,
							Symbol:        funcName,
							Impact:        "All callers of this function will break",
							MigrationPath: "Check if function was renamed or moved",
							Severity:      events.ReviewIssueSeverityCritical,
						})
					}
				} else if baseSig != currentSig {
					// Function signature changed
					if isExportedSymbol(funcName, filePath) {
						changes = append(changes, events.BreakingChange{
							ChangeType:    events.BreakingChangeChangedSignature,
							Description:   "Function signature changed",
							FilePath:      filePath,
							Symbol:        funcName,
							Impact:        "Callers may need to update their code",
							MigrationPath: "Update callers to use new signature",
							Severity:      events.ReviewIssueSeverityHigh,
						})
					}
				}
			}

			// Check for removed struct fields or interface methods
			removedFields := detectRemovedFields(baselineContent, currentContent, filePath)
			changes = append(changes, removedFields...)
		}
	}

	return changes
}

// detectDependencyViolations detects dependency rule violations.
func (a *PatternArchitectureAnalyzer) detectDependencyViolations(files map[string]string, language string) []events.DependencyViolation {
	var violations []events.DependencyViolation

	// Define forbidden dependencies based on common architecture rules
	forbiddenDeps := map[string][]string{
		// Handler layer should not import repository directly
		"handlers": {"repository", "repositories", "dao", "datastore"},
		// Repository should not import handler
		"repository": {"handlers", "handler", "controller", "controllers"},
		// Business logic should not import HTTP/transport concerns
		"business": {"http", "gin", "echo", "fiber", "chi"},
		"service":  {"http", "gin", "echo", "fiber", "chi"},
		// Domain should not import infrastructure
		"domain": {"database", "sql", "gorm", "repository", "infrastructure"},
		"models": {"http", "gin", "handlers", "repository"},
	}

	for filePath, content := range files {
		fileLayer := detectLayer(filePath)
		if fileLayer == "" {
			continue
		}

		forbidden, exists := forbiddenDeps[fileLayer]
		if !exists {
			continue
		}

		imports := extractImports(content, language)
		for _, imp := range imports {
			for _, forbiddenPkg := range forbidden {
				if strings.Contains(strings.ToLower(imp), forbiddenPkg) {
					lineNum := findImportLineNumber(content, imp)
					violations = append(violations, events.DependencyViolation{
						ViolationType: events.DependencyViolationForbidden,
						FromModule:    fileLayer,
						ToModule:      forbiddenPkg,
						FilePath:      filePath,
						LineNumber:    lineNum,
						Rule:          fileLayer + " layer should not depend on " + forbiddenPkg,
						Severity:      events.ReviewIssueSeverityMedium,
					})
				}
			}
		}
	}

	return violations
}

// detectLayeringViolations detects architectural layer violations.
func (a *PatternArchitectureAnalyzer) detectLayeringViolations(files map[string]string, language string) []events.LayeringViolation {
	var violations []events.LayeringViolation

	// Standard layer order (from outer to inner):
	// presentation -> application -> domain -> infrastructure
	layerOrder := map[string]int{
		"handlers":     1,
		"controllers":  1,
		"presentation": 1,
		"application":  2,
		"business":     2,
		"service":      2,
		"domain":       3,
		"models":       3,
		"entities":     3,
		"repository":   4,
		"infrastructure": 4,
		"database":     4,
	}

	for filePath, content := range files {
		sourceLayer := detectLayer(filePath)
		if sourceLayer == "" {
			continue
		}

		sourceOrder, hasSource := layerOrder[sourceLayer]
		if !hasSource {
			continue
		}

		imports := extractImports(content, language)
		for _, imp := range imports {
			for targetLayer, targetOrder := range layerOrder {
				if strings.Contains(strings.ToLower(imp), targetLayer) {
					// Check for reverse flow (inner layer depending on outer)
					if targetOrder < sourceOrder {
						violations = append(violations, events.LayeringViolation{
							ViolationType: events.LayeringViolationReverseFlow,
							Description:   "Inner layer depends on outer layer",
							SourceLayer:   sourceLayer,
							TargetLayer:   targetLayer,
							FilePath:      filePath,
							Severity:      events.ReviewIssueSeverityMedium,
						})
					}
					break
				}
			}
		}
	}

	return violations
}

// detectInterfaceChanges detects changes to interfaces.
func (a *PatternArchitectureAnalyzer) detectInterfaceChanges(patches []events.Patch, baseline, current map[string]string) []events.InterfaceChange {
	var changes []events.InterfaceChange

	for filePath, currentContent := range current {
		baselineContent, exists := baseline[filePath]
		if !exists {
			continue
		}

		baselineInterfaces := extractInterfaces(baselineContent, filePath)
		currentInterfaces := extractInterfaces(currentContent, filePath)

		// Check for changes in interfaces
		for ifaceName, baseMethods := range baselineInterfaces {
			if currentMethods, found := currentInterfaces[ifaceName]; !found {
				// Interface was removed
				changes = append(changes, events.InterfaceChange{
					InterfaceName: ifaceName,
					ChangeType:    events.InterfaceChangeRemovedMethod,
					Description:   "Interface was removed",
					FilePath:      filePath,
					IsBreaking:    true,
				})
			} else {
				// Check for removed methods
				for method := range baseMethods {
					if _, methodExists := currentMethods[method]; !methodExists {
						changes = append(changes, events.InterfaceChange{
							InterfaceName: ifaceName,
							ChangeType:    events.InterfaceChangeRemovedMethod,
							Description:   "Method " + method + " was removed from interface",
							FilePath:      filePath,
							IsBreaking:    true,
						})
					}
				}

				// Check for added methods (may require implementations to update)
				for method := range currentMethods {
					if _, methodExists := baseMethods[method]; !methodExists {
						changes = append(changes, events.InterfaceChange{
							InterfaceName: ifaceName,
							ChangeType:    events.InterfaceChangeAddedMethod,
							Description:   "Method " + method + " was added to interface",
							FilePath:      filePath,
							IsBreaking:    true, // Adding methods to interface breaks implementers
						})
					}
				}
			}
		}
	}

	return changes
}

// detectPatternViolations detects design pattern violations.
func (a *PatternArchitectureAnalyzer) detectPatternViolations(files map[string]string, language string) []events.PatternViolation {
	var violations []events.PatternViolation

	for filePath, content := range files {
		// Check for common anti-patterns

		// God object: file with too many responsibilities
		if countMethods(content, language) > 20 {
			violations = append(violations, events.PatternViolation{
				PatternName:    "Single Responsibility",
				ViolationType:  "god_object",
				Description:    "File has too many methods, may have too many responsibilities",
				FilePath:       filePath,
				Recommendation: "Consider splitting into smaller, focused components",
			})
		}

		// Large function
		largeFuncs := detectLargeFunctions(content, language)
		for _, funcName := range largeFuncs {
			violations = append(violations, events.PatternViolation{
				PatternName:    "Keep It Simple",
				ViolationType:  "large_function",
				Description:    "Function " + funcName + " is too long",
				FilePath:       filePath,
				Recommendation: "Break down into smaller, focused functions",
			})
		}

		// Check for service locator anti-pattern
		if containsServiceLocator(content) {
			violations = append(violations, events.PatternViolation{
				PatternName:    "Dependency Injection",
				ViolationType:  "service_locator",
				Description:    "Service locator pattern detected, prefer dependency injection",
				FilePath:       filePath,
				Recommendation: "Use constructor injection instead of service locator",
			})
		}

		// Check for global state
		if hasGlobalState(content, language) {
			violations = append(violations, events.PatternViolation{
				PatternName:    "No Global State",
				ViolationType:  "global_state",
				Description:    "Global mutable state detected",
				FilePath:       filePath,
				Recommendation: "Encapsulate state in objects and inject dependencies",
			})
		}

		// Check for direct database access in handlers
		if isHandler(filePath) && hasDirectDatabaseAccess(content) {
			violations = append(violations, events.PatternViolation{
				PatternName:    "Clean Architecture",
				ViolationType:  "handler_db_access",
				Description:    "Handler has direct database access",
				FilePath:       filePath,
				Recommendation: "Use repository pattern, inject repository into handler",
			})
		}
	}

	return violations
}

// generateRecommendations generates architecture recommendations.
func (a *PatternArchitectureAnalyzer) generateRecommendations(assessment *events.ArchitectureAssessment) []events.ArchitectureRecommendation {
	var recommendations []events.ArchitectureRecommendation

	// Recommend refactoring if many dependency violations
	if len(assessment.DependencyViolations) > 3 {
		recommendations = append(recommendations, events.ArchitectureRecommendation{
			Category:       "Architecture",
			Recommendation: "Refactor to fix dependency violations",
			Rationale:      "Multiple dependency violations indicate architectural drift",
			Priority:       "high",
		})
	}

	// Recommend breaking change documentation
	if len(assessment.BreakingChanges) > 0 {
		recommendations = append(recommendations, events.ArchitectureRecommendation{
			Category:       "Documentation",
			Recommendation: "Document breaking changes and migration path",
			Rationale:      "Breaking changes require clear communication to consumers",
			Priority:       "critical",
		})
	}

	// Recommend interface review
	if len(assessment.InterfaceChanges) > 0 {
		for _, change := range assessment.InterfaceChanges {
			if change.IsBreaking {
				recommendations = append(recommendations, events.ArchitectureRecommendation{
					Category:       "Compatibility",
					Recommendation: "Review interface changes for backward compatibility",
					Rationale:      "Interface changes may break existing implementations",
					Priority:       "high",
					AffectedFiles:  []string{change.FilePath},
				})
				break
			}
		}
	}

	// Recommend addressing pattern violations
	if len(assessment.PatternViolations) > 5 {
		recommendations = append(recommendations, events.ArchitectureRecommendation{
			Category:       "Design",
			Recommendation: "Address design pattern violations",
			Rationale:      "Multiple pattern violations affect maintainability",
			Priority:       "medium",
		})
	}

	return recommendations
}

// calculateArchitectureScore calculates the overall architecture score.
func (a *PatternArchitectureAnalyzer) calculateArchitectureScore(assessment *events.ArchitectureAssessment) int {
	score := 100

	// Deduct for breaking changes
	for _, bc := range assessment.BreakingChanges {
		switch bc.Severity {
		case events.ReviewIssueSeverityCritical:
			score -= 25
		case events.ReviewIssueSeverityHigh:
			score -= 15
		case events.ReviewIssueSeverityMedium:
			score -= 10
		default:
			score -= 5
		}
	}

	// Deduct for dependency violations
	for _, dv := range assessment.DependencyViolations {
		switch dv.Severity {
		case events.ReviewIssueSeverityCritical:
			score -= 20
		case events.ReviewIssueSeverityHigh:
			score -= 12
		case events.ReviewIssueSeverityMedium:
			score -= 8
		default:
			score -= 4
		}
	}

	// Deduct for layering violations
	for _, lv := range assessment.LayeringViolations {
		switch lv.Severity {
		case events.ReviewIssueSeverityCritical:
			score -= 15
		case events.ReviewIssueSeverityHigh:
			score -= 10
		default:
			score -= 5
		}
	}

	// Deduct for breaking interface changes
	for _, ic := range assessment.InterfaceChanges {
		if ic.IsBreaking {
			score -= 15
		} else {
			score -= 3
		}
	}

	// Deduct for pattern violations
	for range assessment.PatternViolations {
		score -= 5
	}

	// Deduct for circular dependencies
	for range assessment.CircularDependencies {
		score -= 20
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

// determineArchitectureStatus determines the overall architecture status.
func (a *PatternArchitectureAnalyzer) determineArchitectureStatus(assessment *events.ArchitectureAssessment) events.ArchitectureStatus {
	// Check for critical breaking changes
	for _, bc := range assessment.BreakingChanges {
		if bc.Severity == events.ReviewIssueSeverityCritical {
			return events.ArchitectureStatusBlocked
		}
	}

	// Check for serious violations
	if len(assessment.BreakingChanges) > 0 || len(assessment.CircularDependencies) > 0 {
		return events.ArchitectureStatusViolations
	}

	// Check score thresholds
	if assessment.OverallArchitectureScore < 50 {
		return events.ArchitectureStatusBlocked
	} else if assessment.OverallArchitectureScore < 70 {
		return events.ArchitectureStatusViolations
	} else if assessment.OverallArchitectureScore < 90 {
		return events.ArchitectureStatusWarnings
	}

	return events.ArchitectureStatusCompliant
}

// determineArchitectureReviewRequired determines if manual architecture review is needed.
func (a *PatternArchitectureAnalyzer) determineArchitectureReviewRequired(assessment *events.ArchitectureAssessment) (bool, string) {
	if len(assessment.BreakingChanges) > 0 {
		return true, "Breaking changes detected that may affect consumers"
	}

	if len(assessment.CircularDependencies) > 0 {
		return true, "Circular dependencies introduced"
	}

	for _, ic := range assessment.InterfaceChanges {
		if ic.IsBreaking {
			return true, "Breaking interface changes detected"
		}
	}

	if assessment.OverallArchitectureScore < 50 {
		return true, "Low architecture score requires review"
	}

	if len(assessment.DependencyViolations) > 5 {
		return true, "Multiple dependency violations require review"
	}

	return false, ""
}

// Helper functions

func isExportedFile(filePath, content string) bool {
	// In Go, any public symbol makes the file potentially exported
	// Check for exported (capitalized) symbols
	exportedPattern := regexp.MustCompile(`(?m)^func\s+[A-Z]|^type\s+[A-Z]|^var\s+[A-Z]|^const\s+[A-Z]`)
	return exportedPattern.MatchString(content)
}

func extractFunctionSignatures(content, filePath string) map[string]string {
	signatures := make(map[string]string)

	// Go function pattern
	if strings.HasSuffix(filePath, ".go") {
		funcPattern := regexp.MustCompile(`func\s+(?:\([^)]+\)\s+)?(\w+)\s*\([^)]*\)(?:\s*\([^)]*\)|[^{]*)?`)
		matches := funcPattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				signatures[match[1]] = match[0]
			}
		}
	}

	// JavaScript/TypeScript function pattern
	if strings.HasSuffix(filePath, ".js") || strings.HasSuffix(filePath, ".ts") {
		funcPattern := regexp.MustCompile(`(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\([^)]*\)`)
		matches := funcPattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				signatures[match[1]] = match[0]
			}
		}

		// Arrow function exports
		arrowPattern := regexp.MustCompile(`export\s+(?:const|let)\s+(\w+)\s*=\s*(?:async\s*)?\([^)]*\)\s*=>`)
		arrowMatches := arrowPattern.FindAllStringSubmatch(content, -1)
		for _, match := range arrowMatches {
			if len(match) >= 2 {
				signatures[match[1]] = match[0]
			}
		}
	}

	return signatures
}

func isExportedSymbol(name, filePath string) bool {
	// Go: capitalized names are exported
	if strings.HasSuffix(filePath, ".go") {
		return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
	}
	// JavaScript/TypeScript: assume exported if in the signatures (they're already filtered for export)
	return true
}

func detectRemovedFields(baseline, current, filePath string) []events.BreakingChange {
	var changes []events.BreakingChange

	if !strings.HasSuffix(filePath, ".go") {
		return changes
	}

	// Extract struct fields
	baselineFields := extractStructFields(baseline)
	currentFields := extractStructFields(current)

	for structName, fields := range baselineFields {
		if currentStructFields, exists := currentFields[structName]; exists {
			for fieldName := range fields {
				if _, fieldExists := currentStructFields[fieldName]; !fieldExists {
					if isExportedSymbol(fieldName, filePath) {
						changes = append(changes, events.BreakingChange{
							ChangeType:    events.BreakingChangeRemovedField,
							Description:   "Field " + fieldName + " removed from struct " + structName,
							FilePath:      filePath,
							Symbol:        structName + "." + fieldName,
							Impact:        "Code accessing this field will break",
							MigrationPath: "Update consuming code to not use this field",
							Severity:      events.ReviewIssueSeverityHigh,
						})
					}
				}
			}
		}
	}

	return changes
}

func extractStructFields(content string) map[string]map[string]bool {
	structs := make(map[string]map[string]bool)

	structPattern := regexp.MustCompile(`type\s+(\w+)\s+struct\s*\{([^}]*)\}`)
	matches := structPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			structName := match[1]
			fieldsBlock := match[2]
			fields := make(map[string]bool)

			fieldPattern := regexp.MustCompile(`^\s*(\w+)\s+\S+`)
			lines := strings.Split(fieldsBlock, "\n")
			for _, line := range lines {
				fieldMatch := fieldPattern.FindStringSubmatch(line)
				if len(fieldMatch) >= 2 {
					fields[fieldMatch[1]] = true
				}
			}

			structs[structName] = fields
		}
	}

	return structs
}

func detectLayer(filePath string) string {
	lowerPath := strings.ToLower(filePath)

	layers := []string{
		"handlers", "handler", "controllers", "controller", "presentation",
		"application", "business", "service", "services",
		"domain", "models", "entities", "entity",
		"repository", "repositories", "infrastructure", "database", "db",
	}

	for _, layer := range layers {
		if strings.Contains(lowerPath, "/"+layer+"/") || strings.Contains(lowerPath, "/"+layer+".") {
			return layer
		}
	}

	return ""
}

func extractImports(content, language string) []string {
	var imports []string

	switch language {
	case "go":
		importPattern := regexp.MustCompile(`import\s*\(([^)]+)\)|import\s+"([^"]+)"`)
		matches := importPattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if match[1] != "" {
				// Multi-line import
				lines := strings.Split(match[1], "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					line = strings.Trim(line, `"`)
					if line != "" && !strings.HasPrefix(line, "//") {
						// Handle aliased imports
						parts := strings.Fields(line)
						if len(parts) > 0 {
							impPath := strings.Trim(parts[len(parts)-1], `"`)
							imports = append(imports, impPath)
						}
					}
				}
			} else if match[2] != "" {
				imports = append(imports, match[2])
			}
		}
	case "javascript", "typescript":
		importPattern := regexp.MustCompile(`import\s+.*from\s+['"]([^'"]+)['"]`)
		matches := importPattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				imports = append(imports, match[1])
			}
		}
	case "python":
		importPattern := regexp.MustCompile(`(?:from\s+(\S+)\s+)?import\s+(\S+)`)
		matches := importPattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if match[1] != "" {
				imports = append(imports, match[1])
			}
			if match[2] != "" {
				imports = append(imports, match[2])
			}
		}
	}

	return imports
}

func findImportLineNumber(content, importPath string) int {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, importPath) {
			return i + 1
		}
	}
	return 0
}

func extractInterfaces(content, filePath string) map[string]map[string]bool {
	interfaces := make(map[string]map[string]bool)

	if !strings.HasSuffix(filePath, ".go") {
		return interfaces
	}

	interfacePattern := regexp.MustCompile(`type\s+(\w+)\s+interface\s*\{([^}]*)\}`)
	matches := interfacePattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			ifaceName := match[1]
			methodsBlock := match[2]
			methods := make(map[string]bool)

			methodPattern := regexp.MustCompile(`^\s*(\w+)\s*\(`)
			lines := strings.Split(methodsBlock, "\n")
			for _, line := range lines {
				methodMatch := methodPattern.FindStringSubmatch(line)
				if len(methodMatch) >= 2 {
					methods[methodMatch[1]] = true
				}
			}

			interfaces[ifaceName] = methods
		}
	}

	return interfaces
}

func countMethods(content, language string) int {
	var pattern *regexp.Regexp

	switch language {
	case "go":
		pattern = regexp.MustCompile(`func\s+(?:\([^)]+\)\s+)?\w+\s*\(`)
	case "javascript", "typescript":
		pattern = regexp.MustCompile(`(?:function\s+\w+|(?:async\s+)?(?:\w+)\s*\([^)]*\)\s*[:{])`)
	case "python":
		pattern = regexp.MustCompile(`def\s+\w+\s*\(`)
	default:
		return 0
	}

	return len(pattern.FindAllString(content, -1))
}

func detectLargeFunctions(content, language string) []string {
	var largeFuncs []string

	// Simple heuristic: functions with more than 50 lines
	var funcPattern *regexp.Regexp

	switch language {
	case "go":
		funcPattern = regexp.MustCompile(`func\s+(?:\([^)]+\)\s+)?(\w+)\s*\([^{]*\{`)
	case "javascript", "typescript":
		funcPattern = regexp.MustCompile(`function\s+(\w+)\s*\([^{]*\{`)
	case "python":
		funcPattern = regexp.MustCompile(`def\s+(\w+)\s*\(`)
	default:
		return largeFuncs
	}

	matches := funcPattern.FindAllStringSubmatchIndex(content, -1)
	for i, match := range matches {
		if len(match) >= 4 {
			funcName := content[match[2]:match[3]]
			startPos := match[0]

			// Find end of function (simple heuristic)
			var endPos int
			if i+1 < len(matches) {
				endPos = matches[i+1][0]
			} else {
				endPos = len(content)
			}

			lineCount := strings.Count(content[startPos:endPos], "\n")
			if lineCount > 50 {
				largeFuncs = append(largeFuncs, funcName)
			}
		}
	}

	return largeFuncs
}

func containsServiceLocator(content string) bool {
	// Common service locator patterns
	patterns := []string{
		`\.GetService\(`,
		`\.Resolve\(`,
		`ServiceLocator\.`,
		`Container\.Get\(`,
		`GetInstance\(`,
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}

	return false
}

func hasGlobalState(content, language string) bool {
	switch language {
	case "go":
		// Check for package-level var declarations that are mutable
		globalVarPattern := regexp.MustCompile(`(?m)^var\s+\w+\s+=`)
		return globalVarPattern.MatchString(content)
	case "javascript", "typescript":
		// Check for global variables
		globalPattern := regexp.MustCompile(`(?m)^(?:let|var)\s+\w+\s*=`)
		return globalPattern.MatchString(content)
	case "python":
		// Check for module-level mutable state
		// This is harder to detect accurately
		return false
	}

	return false
}

func isHandler(filePath string) bool {
	lowerPath := strings.ToLower(filePath)
	return strings.Contains(lowerPath, "handler") ||
		strings.Contains(lowerPath, "controller") ||
		strings.Contains(lowerPath, "endpoint")
}

func hasDirectDatabaseAccess(content string) bool {
	dbPatterns := []string{
		`db\.Query`,
		`db\.Exec`,
		`db\.Raw`,
		`\.Where\(.*\)\.Find\(`,
		`sql\.Open`,
		`gorm\.Open`,
		`mongo\.Connect`,
	}

	for _, pattern := range dbPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}

	return false
}
