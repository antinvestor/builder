package review //nolint:testpackage // white-box testing requires internal access

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/antinvestor/builder/internal/events"
)

func TestPatternArchitectureAnalyzer_BreakingChanges(t *testing.T) {
	analyzer := NewPatternArchitectureAnalyzer(nil)

	tests := []struct {
		name              string
		baseline          map[string]string
		current           map[string]string
		wantBreaking      int
		wantBreakingTypes []events.BreakingChangeType
	}{
		{
			name: "detects removed exported function",
			baseline: map[string]string{
				"service.go": `
					package service
					func ProcessOrder(order Order) error {
						return nil
					}
					func GetOrder(id string) Order {
						return Order{}
					}
				`,
			},
			current: map[string]string{
				"service.go": `
					package service
					func GetOrder(id string) Order {
						return Order{}
					}
				`,
			},
			wantBreaking:      1,
			wantBreakingTypes: []events.BreakingChangeType{events.BreakingChangeRemovedAPI},
		},
		{
			name: "detects deleted file with exports",
			baseline: map[string]string{
				"api.go": `
					package api
					func HandleRequest(w http.ResponseWriter, r *http.Request) {}
				`,
			},
			current: map[string]string{
				"api.go": `
					package api
					// File was emptied, function removed
				`,
			},
			wantBreaking:      1,
			wantBreakingTypes: []events.BreakingChangeType{events.BreakingChangeRemovedAPI},
		},
		{
			name: "no breaking change for private function removal",
			baseline: map[string]string{
				"internal.go": `
					package internal
					func processData(data []byte) error {
						return nil
					}
				`,
			},
			current: map[string]string{
				"internal.go": `
					package internal
				`,
			},
			wantBreaking: 0,
		},
		{
			name: "detects changed function signature",
			baseline: map[string]string{
				"service.go": `
					package service
					func ProcessOrder(order Order) error {
						return nil
					}
				`,
			},
			current: map[string]string{
				"service.go": `
					package service
					func ProcessOrder(order Order, options Options) error {
						return nil
					}
				`,
			},
			wantBreaking:      1,
			wantBreakingTypes: []events.BreakingChangeType{events.BreakingChangeChangedSignature},
		},
		{
			name: "detects removed struct field",
			baseline: map[string]string{
				"types.go": `
					package types
					type User struct {
						ID       string
						Name     string
						Email    string
						Password string
					}
				`,
			},
			current: map[string]string{
				"types.go": `
					package types
					type User struct {
						ID    string
						Name  string
						Email string
					}
				`,
			},
			wantBreaking:      1,
			wantBreakingTypes: []events.BreakingChangeType{events.BreakingChangeRemovedField},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ArchitectureAnalysisRequest{
				FileContents:     tt.current,
				BaselineContents: tt.baseline,
				Language:         "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			require.Len(t, assessment.BreakingChanges, tt.wantBreaking,
				"breaking changes count mismatch")

			if len(tt.wantBreakingTypes) > 0 {
				for i, bc := range assessment.BreakingChanges {
					if i < len(tt.wantBreakingTypes) {
						require.Equal(t, tt.wantBreakingTypes[i], bc.ChangeType)
					}
				}
			}
		})
	}
}

func TestPatternArchitectureAnalyzer_DependencyViolations(t *testing.T) {
	analyzer := NewPatternArchitectureAnalyzer(nil)

	tests := []struct {
		name           string
		fileContents   map[string]string
		wantViolations int
	}{
		{
			name: "detects handler importing repository directly",
			fileContents: map[string]string{
				"app/handlers/user_handler.go": `package handlers

import (
	"github.com/project/repository"
)

func HandleUser() {
	repo := repository.NewUserRepo()
}
`,
			},
			wantViolations: 1,
		},
		{
			name: "detects repository importing handler",
			fileContents: map[string]string{
				"app/repository/user_repo.go": `package repository

import (
	"github.com/project/handlers"
)

func SaveUser(user User) {
	handlers.NotifyUser(user)
}
`,
			},
			wantViolations: 2, // handler import and layering violation
		},
		{
			name: "clean architecture with proper layering",
			fileContents: map[string]string{
				"app/handlers/user_handler.go": `package handlers

import (
	"github.com/project/service"
)

func HandleUser() {
	svc := service.NewUserService()
}
`,
				"app/service/user_service.go": `package service

import (
	"github.com/project/domain"
)

type UserService struct {
	repo domain.UserRepository
}
`,
			},
			wantViolations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ArchitectureAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			require.Len(t, assessment.DependencyViolations, tt.wantViolations)
		})
	}
}

func TestPatternArchitectureAnalyzer_LayeringViolations(t *testing.T) {
	analyzer := NewPatternArchitectureAnalyzer(nil)

	tests := []struct {
		name           string
		fileContents   map[string]string
		wantViolations int
	}{
		{
			name: "detects domain depending on presentation",
			fileContents: map[string]string{
				"app/domain/user.go": `package domain

import (
	"github.com/project/handlers"
)

type User struct {
	handler handlers.UserHandler
}
`,
			},
			wantViolations: 1,
		},
		{
			name: "clean layering",
			fileContents: map[string]string{
				"app/handlers/user_handler.go": `package handlers

import (
	"github.com/project/domain"
)

func HandleUser(u domain.User) {}
`,
				"app/domain/user.go": `package domain

type User struct {
	ID   string
	Name string
}
`,
			},
			wantViolations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ArchitectureAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			require.Len(t, assessment.LayeringViolations, tt.wantViolations)
		})
	}
}

func TestPatternArchitectureAnalyzer_InterfaceChanges(t *testing.T) {
	analyzer := NewPatternArchitectureAnalyzer(nil)

	tests := []struct {
		name         string
		baseline     map[string]string
		current      map[string]string
		wantChanges  int
		wantBreaking bool
	}{
		{
			name: "detects added method to interface",
			baseline: map[string]string{
				"interface.go": `
					package service
					type UserRepository interface {
						GetUser(id string) User
						SaveUser(user User) error
					}
				`,
			},
			current: map[string]string{
				"interface.go": `
					package service
					type UserRepository interface {
						GetUser(id string) User
						SaveUser(user User) error
						DeleteUser(id string) error
					}
				`,
			},
			wantChanges:  1,
			wantBreaking: true, // Adding method to interface is breaking for implementers
		},
		{
			name: "detects removed method from interface",
			baseline: map[string]string{
				"interface.go": `
					package service
					type UserRepository interface {
						GetUser(id string) User
						SaveUser(user User) error
						DeleteUser(id string) error
					}
				`,
			},
			current: map[string]string{
				"interface.go": `
					package service
					type UserRepository interface {
						GetUser(id string) User
						SaveUser(user User) error
					}
				`,
			},
			wantChanges:  1,
			wantBreaking: true,
		},
		{
			name: "no changes to interface",
			baseline: map[string]string{
				"interface.go": `
					package service
					type UserRepository interface {
						GetUser(id string) User
						SaveUser(user User) error
					}
				`,
			},
			current: map[string]string{
				"interface.go": `
					package service
					type UserRepository interface {
						GetUser(id string) User
						SaveUser(user User) error
					}
				`,
			},
			wantChanges:  0,
			wantBreaking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ArchitectureAnalysisRequest{
				FileContents:     tt.current,
				BaselineContents: tt.baseline,
				Language:         "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			require.Len(t, assessment.InterfaceChanges, tt.wantChanges)

			if tt.wantBreaking && len(assessment.InterfaceChanges) > 0 {
				require.True(t, assessment.InterfaceChanges[0].IsBreaking)
			}
		})
	}
}

func TestPatternArchitectureAnalyzer_PatternViolations(t *testing.T) {
	analyzer := NewPatternArchitectureAnalyzer(nil)

	tests := []struct {
		name           string
		fileContents   map[string]string
		wantViolations int
		wantTypes      []string
	}{
		{
			name: "detects god object with too many methods",
			fileContents: map[string]string{
				"giant.go": generateGiantClass(25), // 25 methods
			},
			wantViolations: 1,
			wantTypes:      []string{"god_object"},
		},
		{
			name: "detects service locator pattern",
			fileContents: map[string]string{
				"service.go": `
					package service
					func NewUserService() *UserService {
						return &UserService{
							repo: Container.Get("UserRepository"),
						}
					}
				`,
			},
			wantViolations: 1,
			wantTypes:      []string{"service_locator"},
		},
		{
			name: "detects handler with direct DB access",
			fileContents: map[string]string{
				"handlers/user_handler.go": `
					package handlers
					func GetUser(w http.ResponseWriter, r *http.Request) {
						user := db.Query("SELECT * FROM users WHERE id = ?", id)
						json.NewEncoder(w).Encode(user)
					}
				`,
			},
			wantViolations: 1,
			wantTypes:      []string{"handler_db_access"},
		},
		{
			name: "clean code without violations",
			fileContents: map[string]string{
				"service/user_service.go": `
					package service
					type UserService struct {
						repo UserRepository
					}
					func NewUserService(repo UserRepository) *UserService {
						return &UserService{repo: repo}
					}
					func (s *UserService) GetUser(id string) (User, error) {
						return s.repo.GetByID(id)
					}
				`,
			},
			wantViolations: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ArchitectureAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			require.Len(t, assessment.PatternViolations, tt.wantViolations,
				"pattern violations count mismatch")

			if len(tt.wantTypes) > 0 {
				foundTypes := make(map[string]bool)
				for _, pv := range assessment.PatternViolations {
					foundTypes[pv.ViolationType] = true
				}
				for _, wantType := range tt.wantTypes {
					require.True(t, foundTypes[wantType], "expected violation type %s not found", wantType)
				}
			}
		})
	}
}

func TestPatternArchitectureAnalyzer_ArchitectureScore(t *testing.T) {
	analyzer := NewPatternArchitectureAnalyzer(nil)

	tests := []struct {
		name           string
		baseline       map[string]string
		current        map[string]string
		wantScoreAbove int
		wantStatus     events.ArchitectureStatus
	}{
		{
			name:     "clean code with no changes",
			baseline: map[string]string{},
			current: map[string]string{
				"service.go": `
					package service
					type UserService struct {
						repo UserRepository
					}
				`,
			},
			wantScoreAbove: 90,
			wantStatus:     events.ArchitectureStatusCompliant,
		},
		{
			name: "breaking changes lower score",
			baseline: map[string]string{
				"api.go": `
					package api
					func ProcessOrder(order Order) error { return nil }
				`,
			},
			current: map[string]string{
				"api.go": `
					package api
					// Function removed
				`,
			},
			wantScoreAbove: 50,
			wantStatus:     events.ArchitectureStatusBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &ArchitectureAnalysisRequest{
				FileContents:     tt.current,
				BaselineContents: tt.baseline,
				Language:         "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			require.GreaterOrEqual(t, assessment.OverallArchitectureScore, tt.wantScoreAbove)
			require.Equal(t, tt.wantStatus, assessment.ArchitectureStatus)
		})
	}
}

func TestPatternArchitectureAnalyzer_Recommendations(t *testing.T) {
	analyzer := NewPatternArchitectureAnalyzer(nil)

	// Test that recommendations are generated for breaking changes
	req := &ArchitectureAnalysisRequest{
		FileContents: map[string]string{
			"api.go": `
				package api
				// Function removed
			`,
		},
		BaselineContents: map[string]string{
			"api.go": `
				package api
				func HandleRequest() {}
			`,
		},
		Language: "go",
	}

	assessment, err := analyzer.Analyze(context.Background(), req)
	require.NoError(t, err)

	// Should have recommendation to document breaking changes
	hasDocRecommendation := false
	for _, rec := range assessment.Recommendations {
		if rec.Category == "Documentation" {
			hasDocRecommendation = true
			break
		}
	}
	require.True(t, hasDocRecommendation, "expected documentation recommendation for breaking changes")
}

// generateGiantClass creates a class with many methods for testing.
func generateGiantClass(methodCount int) string {
	var builder strings.Builder
	builder.WriteString(`package giant
type GiantClass struct {}
`)
	for i := range methodCount {
		builder.WriteString(`func (g *GiantClass) Method`)
		builder.WriteRune(rune('A' + i%26))
		builder.WriteString(`() {}
`)
	}
	return builder.String()
}
