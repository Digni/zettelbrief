package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cyphant/zettelbrief/internal/app"
	"github.com/cyphant/zettelbrief/internal/config"
	"github.com/cyphant/zettelbrief/internal/models"
	"github.com/cyphant/zettelbrief/internal/store"
	"github.com/spf13/cobra"
)

var verbose bool

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "zettelbrief", Short: "Ingest Obsidian notes into a local SQLite database", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print per-file warnings and progress")
	root.AddCommand(newInitCommand(), newScanCommand(), newStatusCommand(), newFetchCommand())
	return root
}

func newInitCommand() *cobra.Command {
	var vaultPath string
	var project string
	var folders []string
	var aliases []string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the global zettelbrief config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if vaultPath == "" {
				vaultPath = defaultObsidianVaultPath()
			}
			cfg := &config.Config{VaultPath: vaultPath, Projects: map[string]config.ProjectConfig{}}
			if project != "" {
				if len(folders) == 0 {
					return fmt.Errorf("--folder is required when --project is set")
				}
				cfg.Projects[project] = config.ProjectConfig{Folders: folders, Aliases: aliases}
			}
			if err := config.WriteGlobal("", cfg, force); err != nil {
				return err
			}
			fmt.Printf("Created %s\n", config.DefaultGlobalPath())
			fmt.Println("Next: zettelbrief scan --project <name>")
			return nil
		},
	}
	cmd.Flags().StringVar(&vaultPath, "vault-path", "", "Obsidian vault path (defaults to the common iCloud Obsidian vault path)")
	cmd.Flags().StringVar(&project, "project", "", "optional project name to add to the config")
	cmd.Flags().StringArrayVar(&folders, "folder", nil, "vault-relative project folder; repeat for multiple folders")
	cmd.Flags().StringArrayVar(&aliases, "alias", nil, "project alias for Granola folder matching; repeat for multiple aliases")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing global config")
	return cmd
}

func newScanCommand() *cobra.Command {
	var project string
	var all bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan configured project notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			if (project == "" && !all) || (project != "" && all) {
				return fmt.Errorf("scan requires exactly one of --project or --all")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			for _, warning := range cfg.Warnings {
				fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
			}
			projects := []string{project}
			if all {
				projects = cfg.SortedProjectNames()
			}
			if len(projects) == 0 || projects[0] == "" && !all {
				return fmt.Errorf("no configured projects")
			}
			db, err := store.Open(defaultDBPath())
			if err != nil {
				return err
			}
			defer db.Close()
			for _, name := range projects {
				if verbose {
					fmt.Fprintf(os.Stderr, "scanning project %s\n", name)
				}
				summary, err := app.RunProjectScan(name, *cfg, db)
				if err != nil {
					return err
				}
				fmt.Println(app.FormatSummary(summary))
				if verbose {
					for _, warning := range summary.Warnings {
						fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project name to scan")
	cmd.Flags().BoolVar(&all, "all", false, "scan all configured projects")
	return cmd
}

func newFetchCommand() *cobra.Command {
	var project, repo, noteType, since, until string
	cmd := &cobra.Command{
		Use:   "fetch [flags] <query>",
		Short: "Generate a cited briefing pack from scanned notes",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("fetch requires exactly one query argument")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			summary, err := app.RunFetch(*cfg, app.FetchOptions{Project: project, Repo: repo, Type: noteType, Since: since, Until: until, Query: args[0], DBPath: defaultDBPath()})
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), summary.OutputDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project name to fetch from")
	cmd.Flags().StringVar(&repo, "repo", "", "optional repository filter")
	cmd.Flags().StringVar(&noteType, "type", "", "optional note type filter (daily_work, knowledge, meeting, project_state)")
	cmd.Flags().StringVar(&since, "since", "", "optional inclusive start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "optional inclusive end date (YYYY-MM-DD)")
	return cmd
}

func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show scan status for configured projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			projects := cfg.SortedProjectNames()
			var statuses []models.ProjectStatus
			if store.DBExists(defaultDBPath()) {
				db, err := store.Open(defaultDBPath())
				if err != nil {
					return err
				}
				defer db.Close()
				statuses, err = db.Status(projects)
				if err != nil {
					return err
				}
			} else {
				statuses = store.EmptyStatus(projects)
			}
			fmt.Print(formatStatus(statuses))
			return nil
		},
	}
	return cmd
}

func defaultDBPath() string {
	return filepath.Join(".zettelbrief", "zettelbrief.db")
}

func defaultObsidianVaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Mobile Documents", "iCloud~md~obsidian", "Documents", "Default")
}

func formatStatus(statuses []models.ProjectStatus) string {
	if len(statuses) == 0 {
		return "No configured projects.\n"
	}
	var b strings.Builder
	for i, status := range statuses {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s\n", status.Project)
		if status.LatestCompletedScan.Valid {
			fmt.Fprintf(&b, "  last completed scan: %s\n", status.LatestCompletedScan.String)
		} else {
			b.WriteString("  last completed scan: not yet scanned\n")
		}
		fmt.Fprintf(&b, "  notes: %d\n", status.TotalNotes)
		if len(status.TypeCounts) > 0 {
			keys := make([]string, 0, len(status.TypeCounts))
			for typ := range status.TypeCounts {
				keys = append(keys, string(typ))
			}
			sort.Strings(keys)
			for _, key := range keys {
				fmt.Fprintf(&b, "  %s: %d\n", key, status.TypeCounts[models.NoteType(key)])
			}
		}
		if status.LatestFailedScan.Valid {
			fmt.Fprintf(&b, "  latest failed scan: %s", status.LatestFailedScan.String)
			if status.LatestFailedScanError.Valid {
				fmt.Fprintf(&b, " (%s)", status.LatestFailedScanError.String)
			}
			b.WriteByte('\n')
		}
	}
	return b.String()
}
