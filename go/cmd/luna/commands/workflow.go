package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// summarizeTagRE strips HTML tags for a plain-text approximation of a
// fetched page's body -- the same best-effort approach
// internal/codeproject.sniffStructure documents (no third-party HTML
// parser is reachable in this build environment's network policy).
var summarizeTagRE = regexp.MustCompile(`(?s)<[^>]*>`)

func stripHTMLTags(html string) string {
	return strings.TrimSpace(summarizeTagRE.ReplaceAllString(html, " "))
}

func newPlanCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "plan <goal...>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aiplan"},
		Short:      "Bikin rencana kerja langkah demi langkah (legacy: aiplan)",
		Args:       cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Workflow.Plan(cmd.Context(), strings.Join(args, " "))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n(saved: %s)\n", res.Content, res.Outfile)
			return nil
		},
	}
}

func newPromptCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "prompt <task...>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aiprompt"},
		Short:      "Generate prompt siap-pakai buat LUNA lain (legacy: aiprompt)",
		Args:       cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Workflow.Prompt(cmd.Context(), strings.Join(args, " "), app.Clipboard)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n(saved: %s, clipboard: %v)\n", res.Content, res.Outfile, res.CopiedBack)
			return nil
		},
	}
}

func newSpecCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "spec <app-description...>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aispec"},
		Short:      "Generate spesifikasi teknis aplikasi (legacy: aispec)",
		Args:       cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Workflow.Spec(cmd.Context(), strings.Join(args, " "), app.Clipboard)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n(saved: %s, clipboard: %v)\n", res.Content, res.Outfile, res.CopiedBack)
			return nil
		},
	}
}

func newSummarizeCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "summarize <file-or-url>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aisummarize"},
		Short:      "Ringkas isi file atau halaman web (legacy: aisummarize)",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := resolveSummarizeSource(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			res, err := app.Workflow.Summarize(cmd.Context(), content)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Summary)
			return nil
		},
	}
	return cmd
}

// resolveSummarizeSource mirrors aisummarize's own file-vs-URL content
// resolution step (deliberately left to the caller by
// workflow.Summarize's own doc comment -- see that function).
func resolveSummarizeSource(ctx context.Context, src string) (string, error) {
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return stripHTMLTags(string(body)), nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
