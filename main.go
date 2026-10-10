package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type RepoMetadata struct {
	Summary   string   `json:"summary"`
	TechStack []string `json:"tech_stack"`
}

type Config struct {
	GitHubUsername     string                  `json:"github_username"`
	GitHubToken        string                  `json:"github_token"`
	TargetRepositories []string                `json:"target_repositories"`
	Categories         map[string][]string     `json:"categories"`
	RepoMeta           map[string]RepoMetadata `json:"repo_metadata"`
	Annotations        map[string]string       `json:"annotations"`
	PersonalProjects   []PersonalProject       `json:"personal_projects"`
	OutputFilename     string                  `json:"output_filename"`
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type PullRequestNode struct {
	PullRequest struct {
		Title      string    `json:"title"`
		URL        string    `json:"url"`
		Merged     bool      `json:"merged"`
		CreatedAt  time.Time `json:"createdAt"`
		Repository struct {
			NameWithOwner string `json:"nameWithOwner"`
		} `json:"repository"`
	} `json:"pullRequest"`
}

type GraphQLError struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Locations []struct {
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"locations"`
}

type GraphQLData struct {
	User struct {
		ContributionsCollection struct {
			TotalCommitContributions      int `json:"totalCommitContributions"`
			TotalPullRequestContributions int `json:"totalPullRequestContributions"`
			TotalIssueContributions       int `json:"totalIssueContributions"`
			PullRequestContributions      struct {
				Nodes []PullRequestNode `json:"nodes"`
			} `json:"pullRequestContributions"`
		} `json:"contributionsCollection"`
	} `json:"user"`
}

type GraphQLResponse struct {
	Data   GraphQLData    `json:"data"`
	Errors []GraphQLError `json:"errors"`
}

type PortfolioMetrics struct {
	TotalPRs     int
	MergedPRs    int
	UniqueRepos  int
	EarliestDate time.Time
	LatestDate   time.Time
}

type PRViewItem struct {
	RepoName string
	URL      string
	Title    string
	Merged   bool
}

type TemplateData struct {
	Username     string
	TotalCommits int
	TotalPRs     int
	TotalIssues  int
	TopPRs       []PRViewItem
}

// project centric data
type PRItem struct {
	Title  string
	URL    string
	Merged bool
}

type RepoGroup struct {
	Name      string
	Summary   string
	TechStack []string
	PRs       []PRItem
}

type ProjectCentricData struct {
	Username         string
	TotalPRs         int
	TotalCommits     int
	TotalIssues      int
	RepoGroups       []RepoGroup
	PersonalProjects []PersonalProject
}

// personal project struct
type PersonalProject struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	TechStack   []string `json:"tech_stack"`
	URL         string   `json:"url"`
}

// If you want to force a refresh, run: rm .github_cache.json && go run main.go
func main() {
	// Define command-line flags
	configPath := flag.String("config", "config.json", "Path to the JSON configuration file")
	templatePath := flag.String("template", "templates/tex.tmpl", "Path to the LaTeX template file")
	outputDir := flag.String("outdir", "output", "Directory for generated LaTeX and PDF files")
	flag.Parse()

	// 1. Load Configuration from config.json
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config.json: %v", err)
	}

	fmt.Printf("Initialising portfolio generator for GitHub user: %s...\n", cfg.GitHubUsername)

	// 2. Fetch Contributions (automatically uses local cache if .github_cache.json exists)
	prs, totalCommits, totalIssues, err := FetchGitHubContributionsWithCache(cfg)
	if err != nil {
		log.Fatalf("Failed to fetch or process GitHub data: %v", err)
	}

	fmt.Printf("Processing %d filtered pull requests across target repositories...\n", len(prs))

	// Prepare PR items for the template view
	// var topPRs []PRViewItem
	// for _, pr := range prs {
	// 	topPRs = append(topPRs, PRViewItem{
	// 		RepoName: escapeLaTeX(pr.PullRequest.Repository.NameWithOwner),
	// 		URL:      pr.PullRequest.URL,
	// 		Title:    escapeLaTeX(pr.PullRequest.Title),
	// 		Merged:   pr.PullRequest.Merged,
	// 	})
	// }

	// // Bundle information into the template data struct
	// data := TemplateData{
	// 	Username:     cfg.GitHubUsername,
	// 	TotalCommits: totalCommits,
	// 	TotalPRs:     len(prs),
	// 	TotalIssues:  totalIssues,
	// 	TopPRs:       topPRs,
	// }

	// Group pull requests by repository and apply config annotations & metadata
	repoMap := make(map[string][]PRItem)
	for _, pr := range prs {
		repoName := pr.PullRequest.Repository.NameWithOwner
		url := pr.PullRequest.URL

		// Use custom annotation from config if available, otherwise raw title
		displayTitle := escapeLaTeX(pr.PullRequest.Title)
		if customNote, annotated := cfg.Annotations[url]; annotated {
			displayTitle = fmt.Sprintf("\\textbf{%s} \\emph{(PR: %s)}", escapeLaTeX(customNote), displayTitle)
		}

		repoMap[repoName] = append(repoMap[repoName], PRItem{
			Title:  displayTitle,
			URL:    url,
			Merged: pr.PullRequest.Merged,
		})
	}

	// Build sorted or configured repo groups
	var repoGroups []RepoGroup
	for _, repoName := range cfg.TargetRepositories {
		items, exists := repoMap[repoName]
		if !exists || len(items) == 0 {
			continue // Skip repos with no contributions in this fetch window
		}

		meta := cfg.RepoMeta[repoName] // Assumes RepoMeta map[string]RepoMetadata in Config struct
		repoGroups = append(repoGroups, RepoGroup{
			Name:      repoName,
			Summary:   escapeLaTeX(meta.Summary),
			TechStack: meta.TechStack,
			PRs:       items,
		})
	}

	templateData := ProjectCentricData{
		Username:         cfg.GitHubUsername,
		TotalPRs:         len(prs),
		TotalCommits:     totalCommits,
		TotalIssues:      totalIssues,
		RepoGroups:       repoGroups,
		PersonalProjects: cfg.PersonalProjects,
	}

	// 4. Write LaTeX File
	texFilename := filepath.Join(*outputDir, cfg.OutputFilename)
	err = GenerateLaTeX(*templatePath, texFilename, templateData)
	if err != nil {
		log.Fatalf("Failed to generate LaTeX: %v", err)
	}
	fmt.Printf("LaTeX file successfully written to %s\n", texFilename)

	// 5. Compile to PDF using pdflatex
	baseName := strings.TrimSuffix(cfg.OutputFilename, ".tex")
	pdfFilename := fmt.Sprintf("%s.pdf", baseName)
	cmd := exec.Command("xelatex", fmt.Sprintf("-output-directory=%s", *outputDir), texFilename)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Compiling LaTeX to PDF in %s/ via xelatex...\n", *outputDir)
	if err := cmd.Run(); err != nil {
		log.Printf("Warning: xelatex execution failed: %v", err)
		fmt.Println("You can manually compile the generated .tex file using your local LaTeX environment.")
	} else {
		fmt.Printf("PDF successfully compiled: %s/%s\n", *outputDir, pdfFilename)
	}
}

func LoadConfig(filepath string) (*Config, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var cfg Config
	if err := json.NewDecoder(file).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

const cacheFileName = ".github_cache.json"

func FetchGitHubContributionsWithCache(cfg *Config) ([]PullRequestNode, int, int, error) {
	var bodyBytes []byte
	var err error

	// 1. Check if cache exists
	if _, err = os.Stat(cacheFileName); err == nil {
		fmt.Println("Loading GitHub response from local cache (.github_cache.json)...")
		bodyBytes, err = os.ReadFile(cacheFileName)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("failed to read cache file: %w", err)
		}
	} else {
		fmt.Println("Cache not found. Fetching fresh data from GitHub GraphQL API...")
		bodyBytes, err = fetchFromGitHubAPI(cfg)
		if err != nil {
			return nil, 0, 0, err
		}

		// Save response to cache
		err = os.WriteFile(cacheFileName, bodyBytes, 0644)
		if err != nil {
			log.Printf("Warning: failed to write cache file: %v", err)
		} else {
			fmt.Println("Successfully cached response to .github_cache.json")
		}
	}

	// 2. Decode and parse the JSON response (from cache or API)
	return parseGraphQLResponse(bodyBytes, cfg)
}

func fetchFromGitHubAPI(cfg *Config) ([]byte, error) {
	query := `
		query($userName: String!) {
		  user(login: $userName) {
			contributionsCollection {
			  totalCommitContributions
			  totalPullRequestContributions
			  totalIssueContributions
			  pullRequestContributions(first: 100) {
				nodes {
				  pullRequest {
					title
					url
					merged
					createdAt
					repository {
					  nameWithOwner
					}
				  }
				}
			  }
			}
		  }
		}
	`

	reqBody := GraphQLRequest{
		Query: query,
		Variables: map[string]interface{}{
			"userName": cfg.GitHubUsername,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.GitHubToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Go-Contrib-Visualizer")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error communicating with github api: %w", err)
	}
	defer resp.Body.Close()

	// Check Rate Limit and Status codes (same as before)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status code: %d", resp.StatusCode)
	}

	// Read all bytes
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return respBytes, nil
}

func parseGraphQLResponse(bodyBytes []byte, cfg *Config) ([]PullRequestNode, int, int, error) {
	var gqlResp GraphQLResponse
	if err := json.Unmarshal(bodyBytes, &gqlResp); err != nil {
		return nil, 0, 0, fmt.Errorf("failed to decode response json: %w", err)
	}

	// Check for GraphQL-level errors
	if len(gqlResp.Errors) > 0 {
		var errorMsgs []string
		for _, gErr := range gqlResp.Errors {
			errorMsgs = append(errorMsgs, gErr.Message)
		}
		return nil, 0, 0, fmt.Errorf("github graphql errors: %s", strings.Join(errorMsgs, "; "))
	}

	collection := gqlResp.Data.User.ContributionsCollection
	totalCommits := collection.TotalCommitContributions
	totalIssues := collection.TotalIssueContributions

	// Filter PRs against target repositories from config
	targetMap := make(map[string]bool)
	for _, repo := range cfg.TargetRepositories {
		targetMap[repo] = true
	}

	var filtered []PullRequestNode
	for _, node := range collection.PullRequestContributions.Nodes {
		repoName := node.PullRequest.Repository.NameWithOwner
		if targetMap[repoName] {
			filtered = append(filtered, node)
		}
	}

	return filtered, totalCommits, totalIssues, nil
}

func CalculateMetrics(prs []PullRequestNode) PortfolioMetrics {
	metrics := PortfolioMetrics{
		TotalPRs: len(prs),
	}

	repoSet := make(map[string]bool)

	for i, pr := range prs {
		if pr.PullRequest.Merged {
			metrics.MergedPRs++
		}
		repoSet[pr.PullRequest.Repository.NameWithOwner] = true

		// Track date range
		if i == 0 || pr.PullRequest.CreatedAt.Before(metrics.EarliestDate) {
			metrics.EarliestDate = pr.PullRequest.CreatedAt
		}
		if i == 0 || pr.PullRequest.CreatedAt.After(metrics.LatestDate) {
			metrics.LatestDate = pr.PullRequest.CreatedAt
		}
	}
	metrics.UniqueRepos = len(repoSet)
	return metrics
}

func GenerateLaTeX(tmplPath string, outputFilename string, data ProjectCentricData) error {
	// Parse the template file
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to parse template file at %s: %w", tmplPath, err)
	}

	// Create output .tex file
	f, err := os.Create(outputFilename)
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", outputFilename, err)
	}
	defer f.Close()

	// Execute template into file
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func escapeLaTeX(s string) string {
	replacer := strings.NewReplacer(
		"&", "\\&",
		"%", "\\%",
		"$", "\\$",
		"#", "\\#",
		"_", "\\_",
		"{", "\\{",
		"}", "\\}",
	)
	return replacer.Replace(s)
}
