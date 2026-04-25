package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/plexusone/versionconductor/internal/graph"
	"github.com/plexusone/versionconductor/internal/report"
)

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Dependency graph commands",
	Long:  `Build and analyze dependency relationships across repositories.`,
}

var graphBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build dependency graph from repositories",
	Long: `Build a dependency graph by scanning repositories in specified organizations.

Examples:
  # Build graph for single org
  versionconductor graph build --orgs grokify

  # Build graph for multiple orgs
  versionconductor graph build --orgs grokify,agentplexus,agentlegion`,
	RunE: runGraphBuild,
}

var graphDependentsCmd = &cobra.Command{
	Use:   "dependents <module>",
	Short: "List modules that depend on the specified module",
	Long: `Find all modules in the graph that depend on the specified module.

Examples:
  # Find dependents of mogo
  versionconductor graph dependents github.com/grokify/mogo`,
	Args: cobra.ExactArgs(1),
	RunE: runGraphDependents,
}

var graphDependenciesCmd = &cobra.Command{
	Use:   "dependencies <module>",
	Short: "List dependencies of the specified module",
	Long: `Find all modules that the specified module depends on.

Examples:
  # Find dependencies of gogithub
  versionconductor graph dependencies github.com/grokify/gogithub`,
	Args: cobra.ExactArgs(1),
	RunE: runGraphDependencies,
}

var graphOrderCmd = &cobra.Command{
	Use:   "order",
	Short: "Show upgrade order for managed modules",
	Long: `Display the topological order for upgrading modules.
Modules should be upgraded in this order to maintain compatibility.

Examples:
  # Show upgrade order for all managed modules
  versionconductor graph order

  # Filter by org
  versionconductor graph order --org github.com/grokify`,
	RunE: runGraphOrder,
}

var graphStaleCmd = &cobra.Command{
	Use:   "stale <module> --min-version <version>",
	Short: "Find modules using outdated versions",
	Long: `Find managed modules that are using outdated versions of a dependency.

Examples:
  # Find modules using old gogithub
  versionconductor graph stale github.com/grokify/gogithub --min-version v0.7.0`,
	Args: cobra.ExactArgs(1),
	RunE: runGraphStale,
}

var graphStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show graph statistics",
	Long:  `Display statistics about the dependency graph.`,
	RunE:  runGraphStats,
}

var graphVisualizeCmd = &cobra.Command{
	Use:   "visualize",
	Short: "Output graph in DOT or Mermaid format",
	Long: `Generate visualization output for the dependency graph.

Supported formats:
  - dot: Graphviz DOT format (default)
  - mermaid: Mermaid diagram format

Examples:
  # Output DOT format for Graphviz
  versionconductor graph visualize --orgs grokify > graph.dot
  dot -Tpng graph.dot -o graph.png

  # Output Mermaid format
  versionconductor graph visualize --orgs grokify --viz-format mermaid

  # Include external dependencies
  versionconductor graph visualize --orgs grokify --show-external`,
	RunE: runGraphVisualize,
}

func init() {
	rootCmd.AddCommand(graphCmd)

	// Add subcommands
	graphCmd.AddCommand(graphBuildCmd)
	graphCmd.AddCommand(graphDependentsCmd)
	graphCmd.AddCommand(graphDependenciesCmd)
	graphCmd.AddCommand(graphOrderCmd)
	graphCmd.AddCommand(graphStaleCmd)
	graphCmd.AddCommand(graphStatsCmd)
	graphCmd.AddCommand(graphVisualizeCmd)

	// Build command flags
	graphBuildCmd.Flags().StringSlice("languages", []string{"go"}, "Languages to scan: go, typescript, swift")
	graphBuildCmd.Flags().String("output", "", "Output file for graph JSON (default: stdout)")

	// Order command flags
	graphOrderCmd.Flags().String("org", "", "Filter by organization")

	// Stale command flags
	graphStaleCmd.Flags().String("min-version", "", "Minimum required version")
	_ = graphStaleCmd.MarkFlagRequired("min-version")

	// Visualize command flags
	graphVisualizeCmd.Flags().String("viz-format", "dot", "Output format: dot, mermaid")
	graphVisualizeCmd.Flags().Bool("show-external", false, "Include external dependencies")
	graphVisualizeCmd.Flags().Bool("show-versions", true, "Show version labels on edges")
	graphVisualizeCmd.Flags().Bool("cluster", true, "Cluster nodes by organization")
	graphVisualizeCmd.Flags().String("direction", "TB", "Layout direction: TB, LR, BT, RL")

	// Cache flags (apply to all graph commands)
	graphCmd.PersistentFlags().Bool("cache", true, "Enable caching of API responses")
	graphCmd.PersistentFlags().String("cache-dir", "", "Cache directory (default: system temp)")
	graphCmd.PersistentFlags().Duration("cache-ttl", time.Hour, "Cache TTL duration")
	graphCmd.PersistentFlags().Bool("no-cache", false, "Disable caching")

	_ = viper.BindPFlag("graph.languages", graphBuildCmd.Flags().Lookup("languages"))
	_ = viper.BindPFlag("graph.output", graphBuildCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("graph.org", graphOrderCmd.Flags().Lookup("org"))
	_ = viper.BindPFlag("graph.min-version", graphStaleCmd.Flags().Lookup("min-version"))
	_ = viper.BindPFlag("graph.cache", graphCmd.PersistentFlags().Lookup("cache"))
	_ = viper.BindPFlag("graph.cache-dir", graphCmd.PersistentFlags().Lookup("cache-dir"))
	_ = viper.BindPFlag("graph.cache-ttl", graphCmd.PersistentFlags().Lookup("cache-ttl"))
	_ = viper.BindPFlag("graph.no-cache", graphCmd.PersistentFlags().Lookup("no-cache"))
}

func runGraphBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	token := viper.GetString("token")
	if token == "" {
		return fmt.Errorf("GitHub token required. Set GITHUB_TOKEN or use --token flag")
	}

	orgs := viper.GetStringSlice("orgs")
	if len(orgs) == 0 {
		return fmt.Errorf("at least one organization required (--orgs)")
	}

	verbose := viper.GetBool("verbose")
	languages := viper.GetStringSlice("graph.languages")

	// Build portfolio
	portfolio := graph.Portfolio{
		Name:      "cli-portfolio",
		Orgs:      expandOrgs(orgs),
		Languages: languages,
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Building graph for orgs: %v\n", portfolio.Orgs)
		fmt.Fprintf(os.Stderr, "Languages: %v\n", languages)
	}

	// Build graph
	builder := graph.NewBuilder(token)
	g, err := builder.Build(ctx, portfolio)
	if err != nil {
		return fmt.Errorf("failed to build graph: %w", err)
	}

	// Output results
	format := viper.GetString("format")
	output := viper.GetString("graph.output")

	modules := g.AllModules()
	if verbose {
		fmt.Fprintf(os.Stderr, "Found %d modules\n", len(modules))
	}

	var result string
	switch format {
	case "json":
		snapshot := g.Snapshot()
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return err
		}
		result = string(data)
	default:
		// Table format
		result = formatModulesTable(modules)
	}

	if output != "" {
		return os.WriteFile(output, []byte(result), 0600)
	}

	fmt.Print(result)
	return nil
}

func runGraphDependents(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	moduleName := args[0]

	g, err := loadOrBuildGraph(ctx)
	if err != nil {
		return err
	}

	// Find the module
	moduleID := graph.NewModuleID(graph.LanguageGo, moduleName)
	dependents := g.Dependents(moduleID)

	format := viper.GetString("format")
	switch format {
	case "json":
		data, err := json.MarshalIndent(dependents, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		if len(dependents) == 0 {
			fmt.Printf("No dependents found for %s\n", moduleName)
			return nil
		}
		fmt.Printf("Modules that depend on %s:\n\n", moduleName)
		for _, d := range dependents {
			managed := ""
			if d.IsManaged {
				managed = " (managed)"
			}
			fmt.Printf("  - %s%s\n", d.Name, managed)
		}
	}

	return nil
}

func runGraphDependencies(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	moduleName := args[0]

	g, err := loadOrBuildGraph(ctx)
	if err != nil {
		return err
	}

	// Find the module
	moduleID := graph.NewModuleID(graph.LanguageGo, moduleName)
	deps := g.Dependencies(moduleID)

	format := viper.GetString("format")
	switch format {
	case "json":
		data, err := json.MarshalIndent(deps, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		if len(deps) == 0 {
			fmt.Printf("No dependencies found for %s\n", moduleName)
			return nil
		}
		fmt.Printf("Dependencies of %s:\n\n", moduleName)
		for _, d := range deps {
			managed := ""
			if d.IsManaged {
				managed = " (managed)"
			}
			fmt.Printf("  - %s @ %s%s\n", d.Name, d.Version, managed)
		}
	}

	return nil
}

func runGraphOrder(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	g, err := loadOrBuildGraph(ctx)
	if err != nil {
		return err
	}

	// Filter by org if specified
	org := viper.GetString("graph.org")
	if org != "" {
		g = g.FilterByOrg(org)
	}

	order, err := g.UpgradeOrder()
	if err != nil {
		return fmt.Errorf("failed to compute upgrade order: %w", err)
	}

	format := viper.GetString("format")
	switch format {
	case "json":
		data, err := json.MarshalIndent(order, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		if len(order.Cycles) > 0 {
			fmt.Println("WARNING: Cycles detected!")
			for _, cycle := range order.Cycles {
				fmt.Printf("  Cycle: %v\n", cycle.Modules)
			}
			fmt.Println()
		}

		fmt.Println("Upgrade Order (upgrade in this sequence):")
		fmt.Println()
		for i, m := range order.Modules {
			fmt.Printf("  %d. %s\n", i+1, m.Name)
		}
	}

	return nil
}

func runGraphStale(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	dependency := args[0]
	minVersion := viper.GetString("graph.min-version")

	g, err := loadOrBuildGraph(ctx)
	if err != nil {
		return err
	}

	stale := g.StaleModules(dependency, minVersion)

	format := viper.GetString("format")
	switch format {
	case "json":
		data, err := json.MarshalIndent(stale, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		if len(stale) == 0 {
			fmt.Printf("No modules found using version older than %s of %s\n", minVersion, dependency)
			return nil
		}
		fmt.Printf("Modules using outdated %s (need >= %s):\n\n", dependency, minVersion)
		for _, s := range stale {
			fmt.Printf("  - %s: using %s\n", s.Module.Name, s.Current)
		}
	}

	return nil
}

func runGraphStats(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	g, err := loadOrBuildGraph(ctx)
	if err != nil {
		return err
	}

	// Type assert to get Stats method
	dg, ok := g.(*graph.DependencyGraph)
	if !ok {
		return fmt.Errorf("cannot get stats from this graph type")
	}

	stats := dg.Stats()

	format := viper.GetString("format")
	switch format {
	case "json":
		data, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		fmt.Println("Graph Statistics:")
		fmt.Println()
		fmt.Printf("  Total Modules:    %d\n", stats.TotalModules)
		fmt.Printf("  Managed Modules:  %d\n", stats.ManagedModules)
		fmt.Printf("  External Modules: %d\n", stats.ExternalModules)
		fmt.Printf("  Total Edges:      %d\n", stats.TotalEdges)
		fmt.Println()
		fmt.Println("  By Language:")
		for lang, count := range stats.ByLanguage {
			fmt.Printf("    %s: %d\n", lang, count)
		}
		fmt.Println()
		fmt.Println("  By Organization:")
		for org, count := range stats.ByOrg {
			fmt.Printf("    %s: %d\n", org, count)
		}
	}

	return nil
}

// loadOrBuildGraph loads a cached graph or builds a new one.
func loadOrBuildGraph(ctx context.Context) (graph.Graph, error) {
	token := viper.GetString("token")
	if token == "" {
		return nil, fmt.Errorf("GitHub token required. Set GITHUB_TOKEN or use --token flag")
	}

	orgs := viper.GetStringSlice("orgs")
	if len(orgs) == 0 {
		return nil, fmt.Errorf("at least one organization required (--orgs)")
	}

	portfolio := graph.Portfolio{
		Name:      "cli-portfolio",
		Orgs:      expandOrgs(orgs),
		Languages: []string{"go"},
	}

	// Setup cache if enabled
	var cache *graph.Cache
	if !viper.GetBool("graph.no-cache") {
		cacheConfig := graph.CacheConfig{
			Dir: viper.GetString("graph.cache-dir"),
			TTL: viper.GetDuration("graph.cache-ttl"),
		}
		var err error
		cache, err = graph.NewCache(cacheConfig)
		if err != nil {
			// Log warning but continue without cache
			fmt.Fprintf(os.Stderr, "Warning: failed to create cache: %v\n", err)
		}
	}

	// Build with configuration
	builder := graph.NewBuilderWithConfig(graph.BuilderConfig{
		Token: token,
		Cache: cache,
	})

	return builder.Build(ctx, portfolio)
}

// expandOrgs expands org names to full github.com paths.
func expandOrgs(orgs []string) []string {
	result := make([]string, len(orgs))
	for i, org := range orgs {
		if !strings.Contains(org, "/") {
			result[i] = "github.com/" + org
		} else {
			result[i] = org
		}
	}
	return result
}

// formatModulesTable formats modules as a table.
func formatModulesTable(modules []graph.Module) string {
	if len(modules) == 0 {
		return "No modules found.\n"
	}

	var rows []report.TableRow
	for _, m := range modules {
		managed := "No"
		if m.IsManaged {
			managed = "Yes"
		}
		rows = append(rows, report.TableRow{
			Cells: []string{
				string(m.Language),
				m.Name,
				m.Version,
				managed,
				fmt.Sprintf("%d", len(m.Dependencies)),
			},
		})
	}

	table := report.Table{
		Headers: []string{"Language", "Module", "Version", "Managed", "Dependencies"},
		Rows:    rows,
	}

	return table.Render()
}

func runGraphVisualize(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	g, err := loadOrBuildGraph(ctx)
	if err != nil {
		return err
	}

	// Type assert to get visualization methods
	dg, ok := g.(*graph.DependencyGraph)
	if !ok {
		return fmt.Errorf("cannot visualize this graph type")
	}

	vizFormat, _ := cmd.Flags().GetString("viz-format")
	showExternal, _ := cmd.Flags().GetBool("show-external")
	showVersions, _ := cmd.Flags().GetBool("show-versions")
	cluster, _ := cmd.Flags().GetBool("cluster")
	direction, _ := cmd.Flags().GetString("direction")

	switch vizFormat {
	case "mermaid":
		cfg := graph.MermaidConfig{
			Direction:    direction,
			ShowExternal: showExternal,
		}
		output := dg.ToMermaid(cfg)
		fmt.Print(output)

	case "dot":
		fallthrough
	default:
		cfg := graph.DOTConfig{
			Title:         "Dependency Graph",
			RankDir:       direction,
			ShowExternal:  showExternal,
			ShowVersions:  showVersions,
			ClusterByOrg:  cluster,
			ColorManaged:  "#4CAF50",
			ColorExternal: "#9E9E9E",
		}
		output := dg.ToDOT(cfg)
		fmt.Print(output)
	}

	return nil
}
