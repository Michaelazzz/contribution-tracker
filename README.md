# contribution-tracker

A small Go-based portfolio generator that pulls GitHub contribution data and renders a LaTeX report for a target set of repositories.

## What this project does

The tool reads your GitHub username and token, queries GitHub's GraphQL API, filters pull requests for configured repositories, and assembles a portfolio-style report. It groups work by repository, adds optional annotations and metadata, and writes a generated LaTeX file in the `output/` directory.

## Quick start

1. Create a local `config.json` file in the project root.
2. Add your GitHub username, token, target repositories, and personal project details.
3. Run:

```bash
go run main.go
```

This creates the report in `output/` and attempts to compile it to PDF with `xelatex` if that tool is installed.

## Important security note

`config.json` is intentionally local-only and is ignored by Git so you do not accidentally commit credentials.

- Keep your real `github_token` in your local `config.json` only.
- Do not commit any personal access token to source control.
- The project also caches the GitHub response in `.github_cache.json`, which is also ignored by Git.

## config.json structure

The application expects a JSON object shaped like this:

```json
{
  "github_username": "your-github-username",
  "real_name": "Your Name",
  "github_token": "ghp_your_token_here",
  "target_repositories": [
    "owner/repo-a",
    "owner/repo-b"
  ],
  "categories": {
    "Scientific Visualization & Imaging": [
      "owner/repo-a"
    ],
    "Data Parsing & Tooling": [
      "owner/repo-b"
    ]
  },
  "repo_metadata": {
    "owner/repo-a": {
      "summary": "Short summary of the repository",
      "tech_stack": ["Go", "React", "PostgreSQL"]
    }
  },
  "annotations": {
    "https://github.com/owner/repo-a/pull/123": "Custom note for this PR"
  },
  "personal_projects": [
    {
      "name": "Project Name",
      "description": "What the project does",
      "tech_stack": ["C++", "OpenGL"],
      "url": "https://github.com/yourname/project"
    }
  ],
  "output_filename": "your_report.tex"
}
```

### Field descriptions

- `github_username`: the GitHub login used for GraphQL queries.
- `real_name`: optional human-readable name used for display and personal metadata.
- `github_token`: GitHub personal access token used by the app to query GitHub data.
- `target_repositories`: the repository list the report should include.
- `categories`: optional logical grouping for repositories in the portfolio view.
- `repo_metadata`: extra summary and technology metadata keyed by `owner/repo-name`.
- `annotations`: custom notes for specific pull request URLs, shown in the generated output.
- `personal_projects`: projects you want to highlight alongside your contribution data.
- `output_filename`: file name for the generated `.tex` report.

### Notes

- The program currently reads the fields defined in `main.go` for runtime behavior, especially `github_username`, `github_token`, `target_repositories`, `categories`, `repo_metadata`, `annotations`, `personal_projects`, and `output_filename`.
- `real_name` is present in the config file but is not required by the generator code at this stage.
- If you add or remove repositories from `target_repositories`, make sure their metadata and categories remain consistent for a clean output.
