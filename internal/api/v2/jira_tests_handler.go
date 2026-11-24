package v2

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
)

// JiraTestLookup counts the tests in a run tagged with a JIRA issue key.
// integrations.CoverageService satisfies it.
type JiraTestLookup interface {
	CountJiraTaggedTestsInRun(ctx context.Context, projectID, runID, issueKey string) (int, error)
}

// JiraTestsHandler serves GET /projects/:projectId/test-runs/:runId/jira/:issueKey/tests.
type JiraTestsHandler struct {
	repo  JiraTestLookup
	scope ProjectScoper
}

func NewJiraTestsHandler(repo JiraTestLookup) *JiraTestsHandler {
	return &JiraTestsHandler{repo: repo}
}

// WithScope attaches the project read-access boundary. Nil disables it.
func (h *JiraTestsHandler) WithScope(s ProjectScoper) *JiraTestsHandler {
	h.scope = s
	return h
}

func (h *JiraTestsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/projects/:projectId/test-runs/:runId/jira/:issueKey/tests", h.get)
}

type jiraTestsResponse struct {
	ProjectID string `json:"projectId"`
	RunID     string `json:"runId"`
	IssueKey  string `json:"issueKey"`
	HasTests  bool   `json:"hasTests"`
	TestCount int    `json:"testCount"`
}

// authorize validates the issue key and project access, writing the error
// response itself. It returns the upper-cased key and false on rejection.
func (h *JiraTestsHandler) authorize(c *gin.Context, projectID string) (string, bool) {
	key := strings.ToUpper(c.Param("issueKey"))
	if !integrations.JiraKeyPattern.MatchString(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid jira issue key"})
		return "", false
	}
	if h.scope != nil {
		allowed, unrestricted, err := h.scope.AccessibleProjects(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "authorization failed"})
			return "", false
		}
		if _, ok := allowed[projectID]; !unrestricted && !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "no access to project"})
			return "", false
		}
	}
	return key, true
}

func (h *JiraTestsHandler) get(c *gin.Context) {
	projectID, runID := c.Param("projectId"), c.Param("runId")
	key, ok := h.authorize(c, projectID)
	if !ok {
		return
	}
	count, err := h.repo.CountJiraTaggedTestsInRun(c.Request.Context(), projectID, runID, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "lookup failed"})
		return
	}
	c.JSON(http.StatusOK, jiraTestsResponse{ProjectID: projectID, RunID: runID, IssueKey: key, HasTests: count > 0, TestCount: count})
}
