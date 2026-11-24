package v2_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	apiv2 "github.com/guidewire-oss/fern-platform/internal/api/v2"
)

type fakeJiraLookup struct {
	count  int
	err    error
	gotKey string // empty until called
	gotRun string
}

func (f *fakeJiraLookup) CountJiraTaggedTestsInRun(_ context.Context, _, runID, issueKey string) (int, error) {
	f.gotKey, f.gotRun = issueKey, runID
	return f.count, f.err
}

type jiraTestsResp struct {
	ProjectID string `json:"projectId"`
	RunID     string `json:"runId"`
	IssueKey  string `json:"issueKey"`
	HasTests  bool   `json:"hasTests"`
	TestCount int    `json:"testCount"`
}

func TestJiraTestsHandler(t *testing.T) {
	p1 := fakeScope{allowed: allow("p1")}
	cases := []struct {
		name       string
		pid, key   string
		repo       *fakeJiraLookup
		scope      apiv2.ProjectScoper
		wantCode   int
		wantCalled bool
	}{
		{"run has tagged tests", "p1", "GWCP-12345", &fakeJiraLookup{count: 3}, p1, http.StatusOK, true},
		{"run has no tagged tests", "p1", "GWCP-12345", &fakeJiraLookup{}, p1, http.StatusOK, true},
		{"lowercase key accepted", "p1", "gwcp-12345", &fakeJiraLookup{count: 3}, p1, http.StatusOK, true},
		{"unrestricted caller", "any", "GWCP-1", &fakeJiraLookup{count: 1}, fakeScope{unrestricted: true}, http.StatusOK, true},
		{"nil scope", "any", "GWCP-1", &fakeJiraLookup{}, nil, http.StatusOK, true},
		{"invalid key", "p1", "not-a-key", &fakeJiraLookup{}, p1, http.StatusBadRequest, false},
		{"foreign project", "other", "GWCP-1", &fakeJiraLookup{count: 3}, p1, http.StatusForbidden, false},
		{"scope error", "p1", "GWCP-1", &fakeJiraLookup{}, fakeScope{err: errors.New("boom")}, http.StatusInternalServerError, false},
		{"repo error", "p1", "GWCP-1", &fakeJiraLookup{err: errors.New("db down")}, p1, http.StatusInternalServerError, true},
	}
	gin.SetMode(gin.TestMode)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			apiv2.NewJiraTestsHandler(tc.repo).WithScope(tc.scope).Register(r.Group("/api/v2"))
			w := httptest.NewRecorder()
			path := fmt.Sprintf("/api/v2/projects/%s/test-runs/run-9/jira/%s/tests", tc.pid, tc.key)
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

			if w.Code != tc.wantCode {
				t.Fatalf("code = %d, want %d; body=%s", w.Code, tc.wantCode, w.Body.String())
			}
			if called := tc.repo.gotKey != ""; called != tc.wantCalled {
				t.Fatalf("repo called = %v, want %v", called, tc.wantCalled)
			}
			if tc.wantCode != http.StatusOK {
				return
			}
			if tc.repo.gotRun != "run-9" {
				t.Errorf("runID passed = %q, want run-9", tc.repo.gotRun)
			}
			var resp jiraTestsResp
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			want := jiraTestsResp{tc.pid, "run-9", strings.ToUpper(tc.key), tc.repo.count > 0, tc.repo.count}
			if resp != want {
				t.Errorf("response = %+v, want %+v", resp, want)
			}
		})
	}
}
