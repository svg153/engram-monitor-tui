package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/svg153/engram-monitor-tui/internal/api"
	"github.com/svg153/engram-monitor-tui/internal/app"
	"github.com/svg153/engram-monitor-tui/internal/model"
)

type Runner struct {
	Version    string
	NewService func(addr string) api.Service
	RunTUI     func(api.Service) error
}

func NewRunner(version string) Runner {
	return Runner{
		Version: version,
		NewService: func(addr string) api.Service {
			return api.New(addr)
		},
		RunTUI: app.Run,
	}
}

func (r Runner) Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return r.runTUI(args, stderr)
	}

	switch args[0] {
	case "tui":
		return r.runTUI(args[1:], stderr)
	case "version", "--version":
		fmt.Fprintf(stdout, "engram-monitor-tui %s\n", r.Version)
		return 0
	case "help", "--help", "-h":
		r.printHelp(stdout)
		return 0
	case "health":
		return r.runHealth(args[1:], stdout, stderr)
	case "projects":
		return r.runProjects(args[1:], stdout, stderr)
	case "sessions":
		return r.runSessions(args[1:], stdout, stderr)
	case "prompts":
		return r.runPrompts(args[1:], stdout, stderr)
	case "memories":
		return r.runMemories(args[1:], stdout, stderr)
	case "export":
		return r.runExport(args[1:], stdout, stderr)
	case "merge-projects":
		return r.runMergeProjects(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		r.printHelp(stderr)
		return 1
	}
}

func (r Runner) runTUI(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr(), "Engram HTTP server address")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if err := r.RunTUI(r.NewService(*addr)); err != nil {
		fmt.Fprintf(stderr, "engram-monitor-tui: %v\n", err)
		return 1
	}
	return 0
}

func (r Runner) runHealth(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr(), "Engram HTTP server address")
	asJSON := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	health, err := r.NewService(*addr).Health(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		return writeJSON(stdout, health)
	}
	fmt.Fprintf(stdout, "service\tstatus\tversion\n%s\t%s\t%s\n", health.Service, health.Status, health.Version)
	return 0
}

func (r Runner) runProjects(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(stderr, "usage: engram-monitor-tui projects list [--addr URL] [--json]")
		return 2
	}
	fs := flag.NewFlagSet("projects list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr(), "Engram HTTP server address")
	asJSON := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	service := r.NewService(*addr)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stats, err := service.Stats(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		return writeJSON(stdout, stats.Projects)
	}
	for _, project := range stats.Projects {
		fmt.Fprintln(stdout, project)
	}
	return 0
}

func (r Runner) runSessions(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(stderr, "usage: engram-monitor-tui sessions list [--addr URL] [--project NAME] [--json]")
		return 2
	}
	fs := flag.NewFlagSet("sessions list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr(), "Engram HTTP server address")
	project := fs.String("project", "", "Filter by project")
	asJSON := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	service := r.NewService(*addr)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	observations, err := service.AllObservations(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	summaries, err := service.RecentSessions(ctx, 1000)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	sessions := model.DeriveSessions(observations, summaries)
	if *project != "" {
		filtered := sessions[:0]
		for _, session := range sessions {
			if session.Project == *project {
				filtered = append(filtered, session)
			}
		}
		sessions = filtered
	}
	if *asJSON {
		return writeJSON(stdout, sessions)
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SESSION\tPROJECT\tAGENT\tOBS\tUPDATED\tTITLE")
	for _, session := range sessions {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\t%s\n",
			session.SessionID,
			session.Project,
			session.AgentName,
			session.ObservationCount,
			model.PrettyTime(session.Date),
			session.LatestTitle,
		)
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func (r Runner) runPrompts(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(stderr, "usage: engram-monitor-tui prompts list [--addr URL] [--project NAME] [--json]")
		return 2
	}
	fs := flag.NewFlagSet("prompts list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr(), "Engram HTTP server address")
	project := fs.String("project", "", "Filter by project")
	asJSON := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	prompts, err := r.NewService(*addr).RecentPrompts(ctx, 500)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *project != "" {
		filtered := prompts[:0]
		for _, prompt := range prompts {
			if prompt.Project == *project {
				filtered = append(filtered, prompt)
			}
		}
		prompts = filtered
	}
	if *asJSON {
		return writeJSON(stdout, prompts)
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tPROJECT\tSESSION\tCREATED\tCONTENT")
	for _, prompt := range prompts {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			prompt.ID,
			or(prompt.Project, "(none)"),
			prompt.SessionID,
			model.PrettyTime(prompt.CreatedAt),
			truncate(prompt.Content, 72),
		)
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func (r Runner) runMemories(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "search" {
		fmt.Fprintln(stderr, "usage: engram-monitor-tui memories search --query TEXT [--addr URL] [--type TYPE] [--project NAME] [--scope SCOPE] [--limit N] [--json]")
		return 2
	}
	fs := flag.NewFlagSet("memories search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr(), "Engram HTTP server address")
	query := fs.String("query", "", "Search text")
	typ := fs.String("type", "", "Observation type")
	project := fs.String("project", "", "Project filter")
	scope := fs.String("scope", "", "Scope filter")
	limit := fs.Int("limit", 50, "Maximum results")
	asJSON := fs.Bool("json", false, "Output JSON")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*query) == "" {
		fmt.Fprintln(stderr, "--query is required")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	results, err := r.NewService(*addr).Search(ctx, model.SearchParams{
		Q:       *query,
		Type:    *typ,
		Project: *project,
		Scope:   *scope,
		Limit:   *limit,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if *asJSON {
		return writeJSON(stdout, results)
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTYPE\tPROJECT\tSESSION\tTITLE")
	for _, obs := range results {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			obs.ID,
			obs.Type,
			or(projectOf(obs), "(none)"),
			obs.SessionID,
			obs.Title,
		)
	}
	if err := tw.Flush(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func (r Runner) runExport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr(), "Engram HTTP server address")
	project := fs.String("project", "", "Optional project filter")
	out := fs.String("out", "", "Output file")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*out) == "" {
		fmt.Fprintln(stderr, "--out is required")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := r.NewService(*addr).Export(ctx, *project)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.WriteFile(*out, raw, 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "exported to %s\n", *out)
	return 0
}

func (r Runner) runMergeProjects(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("merge-projects", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", defaultAddr(), "Engram HTTP server address")
	from := fs.String("from", "", "Source project")
	to := fs.String("to", "", "Destination project")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if strings.TrimSpace(*from) == "" || strings.TrimSpace(*to) == "" {
		fmt.Fprintln(stderr, "--from and --to are required")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := r.NewService(*addr).MergeProjects(ctx, *from, *to); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "merged %s into %s\n", *from, *to)
	return 0
}

func (r Runner) printHelp(w io.Writer) {
	fmt.Fprintf(w, `engram-monitor-tui %s

Usage:
  engram-monitor-tui tui [--addr URL]
  engram-monitor-tui health [--addr URL] [--json]
  engram-monitor-tui projects list [--addr URL] [--json]
  engram-monitor-tui sessions list [--addr URL] [--project NAME] [--json]
  engram-monitor-tui prompts list [--addr URL] [--project NAME] [--json]
  engram-monitor-tui memories search --query TEXT [--addr URL] [--type TYPE] [--project NAME] [--scope SCOPE] [--limit N] [--json]
  engram-monitor-tui export --out FILE [--addr URL] [--project NAME]
  engram-monitor-tui merge-projects --from NAME --to NAME [--addr URL]
  engram-monitor-tui version

Direct commands are intended for shell automation and quick inspection.
`, r.Version)
}

func writeJSON(w io.Writer, value any) int {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return 1
	}
	return 0
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func or(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// defaultAddr returns the configured Engram address from env or the hardcoded default.
func defaultAddr() string {
	if addr := os.Getenv("ENGRAM_ADDR"); addr != "" {
		return addr
	}
	return "http://127.0.0.1:7437"
}

func projectOf(obs model.Observation) string {
	if obs.Project == nil {
		return ""
	}
	return *obs.Project
}
