# JIRA Tests in a Test Run — Requirements

## Context

Test runs report `@jira:<KEY>` labels, which ingest stores in `tags`
(`category='jira'`) and links through `spec_run_tags` (a spec) or
`test_run_tags` (the whole run). Runs belong to a project via
`test_runs.project_id` and are identified by clients by `test_runs.run_id`.
Clients need a REST call that answers "how many tests in run R of project P
are tagged with KEY?".

## Requirements (EARS)

- **R1** When a client sends
  `GET /api/v2/projects/{projectId}/test-runs/{runId}/jira/{issueKey}/tests`,
  the system shall return `200` with `{projectId, runId, issueKey, hasTests, testCount}`.
  `testCount` is the number of distinct non-deleted spec runs, in the
  non-deleted test run whose `run_id` is `runId` in that project, that carry
  the JIRA tag; a tag on the test run itself counts every spec in it.
  `hasTests` is `testCount > 0`. A run that does not exist yields
  `testCount: 0`.
- **R2** The system shall match the issue key case-insensitively and
  return it upper-cased.
- **R3** If the upper-cased `issueKey` does not match
  `integrations.JiraKeyPattern` (`^[A-Z][A-Z0-9]+-\d+$`, the same rule the
  coverage tree uses), the system shall return `400`.
- **R4** If the caller has no read access to `projectId` (and is not
  unrestricted), the system shall return `403` without querying.
- **R5** If the authorization lookup or the query fails, the system shall
  return `500`.
- **R6** The endpoint shall sit behind the same auth middleware as the
  other `/api/v2` routes.

## Design

- `GormTagRepository.CountJiraTaggedTestsInRun`: one
  `COUNT(DISTINCT spec_runs.id)` over the run's live suites and specs,
  matching a spec-level tag or a run-level tag via `EXISTS`.
- `integrations.CoverageService.CountJiraTaggedTestsInRun` passes through;
  `JiraKeyPattern` is exported so the handler validates keys the same way.
- `internal/api/v2/jira_tests_handler.go`: `JiraTestsHandler` with a
  one-method `JiraTestLookup` interface (satisfied by
  `integrations.CoverageService`) and the shared `ProjectScoper`.
- `MountOptions.JiraTests` wires it in `MountV2`; `cmd/fern-platform/main.go`
  passes `domainFactory.GetCoverageService()`, which is always constructed
  and does not need `JIRA_ENCRYPTION_KEY` for this lookup.
- Access is project read (same as other v2 reads).

## Tasks

- [x] T1 Repo method + service passthrough + SQLite repo tests (R1, R2)
- [x] T2 Handler + interface + handler tests (R1–R5)
- [x] T3 Mount in `MountV2` and `main.go` (R6)
- [ ] T4 End-to-end check against a running server
