package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/synapbus/synapbus/internal/storage"
	"github.com/synapbus/synapbus/internal/wiki"
)

func addWikiCommands(rootCmd *cobra.Command) {
	wikiCmd := &cobra.Command{
		Use:   "wiki",
		Short: "Wiki export/import for backup and restore",
	}

	var exportDataDir, exportOutput string
	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export all wiki articles as markdown files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWikiExport(exportDataDir, exportOutput)
		},
	}
	exportCmd.Flags().StringVar(&exportDataDir, "data", "./data", "Data directory containing the SQLite database")
	exportCmd.Flags().StringVar(&exportOutput, "output", "", "Output directory for exported markdown files")
	exportCmd.MarkFlagRequired("output")

	var importDataDir, importInput string
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Import wiki articles from markdown files",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWikiImport(importDataDir, importInput)
		},
	}
	importCmd.Flags().StringVar(&importDataDir, "data", "./data", "Data directory containing the SQLite database")
	importCmd.Flags().StringVar(&importInput, "input", "", "Input directory containing markdown files to import")
	importCmd.MarkFlagRequired("input")

	wikiCmd.AddCommand(exportCmd, importCmd)
	rootCmd.AddCommand(wikiCmd)
}

func runWikiExport(dataDir, outputDir string) error {
	ctx := context.Background()

	db, err := storage.New(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(ctx, db.DB); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	store := wiki.NewStore(db.DB)
	summaries, err := store.ListArticles(ctx, "", 500)
	if err != nil {
		return fmt.Errorf("list articles: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	for _, s := range summaries {
		article, err := store.GetArticle(ctx, s.Slug)
		if err != nil {
			fmt.Printf("Warning: could not read %s: %v\n", s.Slug, err)
			continue
		}
		content := formatArticleExport(article)
		path := filepath.Join(outputDir, article.Slug+".md")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}

	indexContent := generateIndex(summaries)
	if err := os.WriteFile(filepath.Join(outputDir, "_index.md"), []byte(indexContent), 0o644); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	fmt.Printf("Exported %d articles to %s\n", len(summaries), outputDir)
	return nil
}

func formatArticleExport(a *wiki.Article) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", a.Title))
	sb.WriteString(fmt.Sprintf("slug: %s\n", a.Slug))
	sb.WriteString(fmt.Sprintf("author: %s\n", a.UpdatedBy))
	sb.WriteString(fmt.Sprintf("revision: %d\n", a.Revision))
	sb.WriteString(fmt.Sprintf("created: %s\n", a.CreatedAt.UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("updated: %s\n", a.UpdatedAt.UTC().Format(time.RFC3339)))
	sb.WriteString("---\n\n")
	sb.WriteString(a.Body)
	if !strings.HasSuffix(a.Body, "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

func generateIndex(articles []wiki.ArticleSummary) string {
	var sb strings.Builder
	sb.WriteString("# Wiki Index\n\n")
	if len(articles) == 0 {
		sb.WriteString("No articles.\n")
		return sb.String()
	}
	sorted := make([]wiki.ArticleSummary, len(articles))
	copy(sorted, articles)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Title < sorted[j].Title })

	sb.WriteString("| Title | Revision | Words | Updated |\n")
	sb.WriteString("|-------|----------|-------|---------|\n")
	for _, a := range sorted {
		sb.WriteString(fmt.Sprintf("| [%s](%s.md) | %d | %d | %s |\n",
			a.Title, a.Slug, a.Revision, a.WordCount, a.UpdatedAt.UTC().Format("2006-01-02")))
	}
	return sb.String()
}

func runWikiImport(dataDir, inputDir string) error {
	ctx := context.Background()

	db, err := storage.New(ctx, dataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(ctx, db.DB); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	store := wiki.NewStore(db.DB)

	entries, err := os.ReadDir(inputDir)
	if err != nil {
		return fmt.Errorf("read input directory: %w", err)
	}

	var imported, updated, skipped int
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "_index.md" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(inputDir, entry.Name()))
		if err != nil {
			fmt.Printf("Warning: could not read %s: %v\n", entry.Name(), err)
			skipped++
			continue
		}

		slug, title, body := parseFrontmatter(string(data))
		if slug == "" {
			slug = strings.TrimSuffix(entry.Name(), ".md")
		}
		if title == "" {
			title = slug
		}

		existing, _ := store.GetArticle(ctx, slug)
		if existing != nil {
			if _, err := store.UpdateArticle(ctx, slug, title, body, "wiki-import"); err != nil {
				fmt.Printf("Warning: could not update %s: %v\n", slug, err)
				skipped++
				continue
			}
			updated++
		} else {
			if _, err := store.CreateArticle(ctx, slug, title, body, "wiki-import"); err != nil {
				fmt.Printf("Warning: could not create %s: %v\n", slug, err)
				skipped++
				continue
			}
			imported++
		}
	}

	fmt.Printf("Import complete: %d created, %d updated, %d skipped\n", imported, updated, skipped)
	return nil
}

func parseFrontmatter(content string) (slug, title, body string) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", "", content
	}
	rest := strings.TrimLeft(content[3:], "\r\n")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", "", content
	}
	frontmatter := rest[:idx]
	body = strings.TrimLeft(rest[idx+4:], "\r\n")

	for _, line := range strings.Split(frontmatter, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		switch key {
		case "slug":
			slug = val
		case "title":
			title = val
		}
	}
	return slug, title, body
}
