// Package server is the MCP server: the tools agents call, on top of the Bytové konto API
// client. Reads pass the API's data on in a compact form; the two writes (a new ticket, a
// comment) return a preview first and are sent only when confirmed, with an audit record.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
)

// Options configures the server.
type Options struct {
	Logger  *slog.Logger
	Version string
	Now     func() time.Time
	// DownloadDir is where download_document saves files; "" disables saving to disk.
	DownloadDir string
	// MaxInlineBytes is the largest document download_document also returns inline
	// (default 4 MiB).
	MaxInlineBytes int64
	// AttachDirs are the absolute directories create_ticket and comment_ticket may attach files
	// from by path; none disables attaching by path (inline content still works).
	AttachDirs []string
}

// Server serves one resident's Bytové konto.
type Server struct {
	api  *api.Client
	opts Options
	log  *slog.Logger
}

const defaultMaxInline = 4 << 20

// New returns a server.
func New(c *api.Client, opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Version == "" {
		opts.Version = "dev"
	}
	if opts.MaxInlineBytes <= 0 {
		opts.MaxInlineBytes = defaultMaxInline
	}
	return &Server{api: c, opts: opts, log: opts.Logger}
}

// MCP builds the MCP server with all tools.
func (s *Server) MCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "bytovekonto-mcp", Version: s.opts.Version}, &mcp.ServerOptions{
		Instructions: instructions,
	})
	s.addReadTools(srv)
	s.addMoneyTools(srv)
	s.addWriteTools(srv)
	return srv
}

const instructions = `Reads one resident's Bytové konto (the BeiT resident app of a rented or owned flat): the tenancy, the building's management and contacts, tickets (fault reports, requests, questions) and their conversations, payments, costs, utility meters, shared documents and notifications.
Start with home_overview. Texts from the management are usually in Czech; quote them and translate when the user speaks another language. Amounts are in the account's currency (usually CZK).
Tickets: list_tickets shows the user's own tickets by default; scope "building" adds the building's public tickets of other residents about common areas. get_ticket shows the whole conversation.
create_ticket and comment_ticket send a message to the building's management in the user's name. Call them first without confirm: the answer is a preview of exactly what would be sent. Show the preview to the user, and call again with confirm true only after the user approved that text. Never send on your own initiative. Write to the management in Czech unless the user asks otherwise. Photos and PDFs can be attached; the preview lists them.`

// guard turns a panic in a tool into an error answer, so one bug does not end the server.
func guard[In any](log *slog.Logger, name string, h mcp.ToolHandlerFor[In, any]) mcp.ToolHandlerFor[In, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in In) (res *mcp.CallToolResult, out any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("tool panicked", "tool", name, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
				res, out, err = nil, nil, errors.New("internal error in the server")
			}
		}()
		return h(ctx, req, in)
	}
}

// schemaFor infers the input schema of T and sets enums on the named properties.
func schemaFor[T any](enums map[string][]any) *jsonschema.Schema {
	sch, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	for prop, vals := range enums {
		p := sch.Properties[prop]
		if p == nil {
			panic("schemaFor: no property " + prop)
		}
		p.Enum = vals
	}
	return sch
}

func readOnly(title string) *mcp.ToolAnnotations {
	t := true
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &t}
}

func clientName(req *mcp.CallToolRequest) string {
	if req == nil || req.Session == nil {
		return ""
	}
	p := req.Session.InitializeParams()
	if p == nil || p.ClientInfo == nil {
		return ""
	}
	return p.ClientInfo.Name
}
