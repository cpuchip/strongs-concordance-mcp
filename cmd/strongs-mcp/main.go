// strongs-mcp is an MCP server providing Strong's Concordance — Hebrew/Greek
// word-study data keyed to the King James Bible — for scripture study. It is
// the original-language companion to webster-mcp.
//
// The lexicon data is embedded in the binary, so it runs self-contained from
// any working directory (Claude Code spawns it via .mcp.json; the
// pg-ai-stewards bridge spawns it from a stewards.mcp_servers row).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	strongs "github.com/cpuchip/strongs-concordance-mcp"
	"github.com/cpuchip/strongs-concordance-mcp/internal/concordance"
	"github.com/cpuchip/strongs-concordance-mcp/internal/mcpserver"
)

func main() {
	showStats := flag.Bool("stats", false, "Print lexicon stats and exit")
	flag.Parse()

	// Never write to stdout except via the MCP transport (stdio).
	log.SetOutput(os.Stderr)

	lex, err := concordance.Load(strongs.LexiconGZ)
	if err != nil {
		log.Fatalf("failed to load embedded lexicon: %v", err)
	}
	verses, err := concordance.LoadVerses(strongs.KJVGZ)
	if err != nil {
		log.Fatalf("failed to load embedded KJV tagging: %v", err)
	}

	if *showStats {
		fmt.Printf("strongs-concordance-mcp: %d lexicon entries, %d tagged KJV verses loaded\n", lex.Count(), verses.Count())
		return
	}

	log.Printf("strongs-concordance-mcp: %d lexicon entries, %d KJV verses loaded; serving on stdio", lex.Count(), verses.Count())
	srv := mcpserver.New(lex, verses)
	if err := srv.Serve(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
