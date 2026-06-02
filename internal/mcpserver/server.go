// Package mcpserver wires the concordance lexicon to MCP tools over stdio.
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/cpuchip/strongs-concordance-mcp/internal/concordance"
)

// Server wraps the MCP server and the lexicon.
type Server struct {
	mcp *server.MCPServer
	lex *concordance.Lexicon
}

// New builds an MCP server exposing the Strong's tools.
func New(lex *concordance.Lexicon) *Server {
	m := server.NewMCPServer(
		"strongs-concordance-mcp",
		"0.1.0",
		server.WithToolCapabilities(true),
	)
	s := &Server{mcp: m, lex: lex}
	s.register()
	return s
}

// Serve runs the server over stdio (blocks).
func (s *Server) Serve() error { return server.ServeStdio(s.mcp) }

func (s *Server) register() {
	s.mcp.AddTool(
		mcp.NewTool("strongs_define",
			mcp.WithDescription("Look up a Strong's number (e.g. H7225, G26) in Strong's Concordance. Returns the original Hebrew/Greek lemma, transliteration, Strong's 1890 definition + KJV-usage gloss + derivation, AND the modern STEPBible (BDB/Abbott-Smith) gloss + definition side by side. The Hebrew/Greek companion to webster_define for KJV scripture word-study."),
			mcp.WithString("number",
				mcp.Required(),
				mcp.Description("Strong's number, e.g. H7225 (Hebrew) or G26 (Greek)"),
			),
		),
		s.handleDefine,
	)

	s.mcp.AddTool(
		mcp.NewTool("strongs_search",
			mcp.WithDescription("Find the Strong's number(s) behind a KJV English word, gloss, or transliteration (reverse lookup). E.g. 'charity' or 'agape' -> G26. Returns ranked matches with brief glosses."),
			mcp.WithString("word",
				mcp.Required(),
				mcp.Description("A KJV English word, gloss, or transliteration to look up"),
			),
			mcp.WithNumber("max_results",
				mcp.Description("Maximum number of results to return (default 20)"),
			),
		),
		s.handleSearch,
	)
}

func (s *Server) handleDefine(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	num, err := req.RequireString("number")
	if err != nil {
		return mcp.NewToolResultError("number parameter is required (e.g. H7225 or G26)"), nil
	}
	e, ok := s.lex.Define(num)
	if !ok {
		return mcp.NewToolResultText(fmt.Sprintf("No Strong's entry found for %q. Use a number like H7225 (Hebrew) or G26 (Greek).", num)), nil
	}
	return mcp.NewToolResultText(formatEntry(e)), nil
}

func (s *Server) handleSearch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	word, err := req.RequireString("word")
	if err != nil {
		return mcp.NewToolResultError("word parameter is required"), nil
	}
	max := 20
	if mr, ok := req.GetArguments()["max_results"].(float64); ok && mr > 0 {
		max = int(mr)
	}
	hits := s.lex.Search(word, max)
	if len(hits) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No Strong's entries found for %q.", word)), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d match(es) for %q:\n\n", len(hits), word)
	for _, e := range hits {
		gloss := e.StepGloss
		if gloss == "" {
			gloss = e.KJVDef
		}
		fmt.Fprintf(&b, "- %s  %s (%s) — %s\n", e.Number, e.Lemma, e.Translit, gloss)
	}
	return mcp.NewToolResultText(b.String()), nil
}

func formatEntry(e *concordance.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s)\n", e.Number, e.Lang)
	if e.Lemma != "" {
		fmt.Fprintf(&b, "Lemma: %s", e.Lemma)
		if e.Translit != "" {
			fmt.Fprintf(&b, "  (%s", e.Translit)
			if e.Pron != "" {
				fmt.Fprintf(&b, ", %s", e.Pron)
			}
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	if e.StrongsDef != "" {
		fmt.Fprintf(&b, "\nStrong's (1890): %s\n", e.StrongsDef)
	}
	if e.KJVDef != "" {
		fmt.Fprintf(&b, "KJV usage: %s\n", e.KJVDef)
	}
	if e.Derivation != "" {
		fmt.Fprintf(&b, "Derivation: %s\n", e.Derivation)
	}
	if e.StepGloss != "" || e.StepDef != "" {
		b.WriteString("\nSTEPBible (modern, BDB/Abbott-Smith")
		if e.StepMorph != "" {
			fmt.Fprintf(&b, ", morph %s", e.StepMorph)
		}
		b.WriteString("):\n")
		if e.StepGloss != "" {
			fmt.Fprintf(&b, "  gloss: %s\n", e.StepGloss)
		}
		if e.StepDef != "" {
			fmt.Fprintf(&b, "  %s\n", e.StepDef)
		}
	}
	return b.String()
}
