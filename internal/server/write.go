package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
)

// ticketCategories are the categories a resident's ticket may have; the API also lists
// management-only ones (announcement, discussion).
var ticketCategories = []string{"FAULT", "ACCIDENT", "REQUEST", "QUESTION", "COMPLAINT", "SUGGESTION", "ORDER", "SERVICE", "CATEGORY_OTHER"}

const (
	defaultArea      = "AREA_OTHER"
	maxSubjectRunes  = 200
	maxTextRunes     = 10000
	ownUnitPreferred = "FLAT"
)

type createTicketIn struct {
	Category    string `json:"category" jsonschema:"what kind of ticket: FAULT (something is broken), ACCIDENT (damage or danger), REQUEST, QUESTION, COMPLAINT, SUGGESTION, ORDER, SERVICE, CATEGORY_OTHER; code or name as ticket_options lists them"`
	Area        string `json:"area,omitempty" jsonschema:"the area code or name from ticket_options (default AREA_OTHER), e.g. EQUIPMENT, ENERGIES, PAYMENTS"`
	Unit        string `json:"unit,omitempty" jsonschema:"what the ticket is about: a unit id or name from ticket_options (default: the user's own flat), e.g. the cellar, the entrance or the building"`
	Subject     string `json:"subject" jsonschema:"a short title"`
	Description string `json:"description" jsonschema:"the message to the management, with a greeting and the user's name as signature"`
	Private     bool   `json:"private,omitempty" jsonschema:"only the user and the management see the ticket (default false, as the app sends it)"`
	Confirm     bool   `json:"confirm,omitempty" jsonschema:"false (default): return a preview and send nothing; true: send it. Set true only after the user approved the preview"`
}

type commentIn struct {
	TicketID int64  `json:"ticket_id" jsonschema:"the ticket to answer"`
	Message  string `json:"message" jsonschema:"the message to post in the ticket's conversation"`
	Confirm  bool   `json:"confirm,omitempty" jsonschema:"false (default): return a preview and send nothing; true: send it. Set true only after the user approved the preview"`
}

func sendTool(title string) *mcp.ToolAnnotations {
	f, t := false, true
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: &f, IdempotentHint: false, OpenWorldHint: &t}
}

func (s *Server) addWriteTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "create_ticket",
		Description: "Send a new ticket (fault report, request, question…) to the building's management in the user's name. " +
			"Without confirm it only returns a preview of exactly what would be sent; show it to the user and call again with confirm true after they approve. " +
			"The management sees the user's name and unit.",
		InputSchema: schemaFor[createTicketIn](nil),
		Annotations: sendTool("Create ticket"),
	}, guard(s.log, "createTicket", s.createTicket))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "comment_ticket",
		Description: "Post a message to a ticket's conversation in the user's name (an answer to the management, or more details). " +
			"Without confirm it only returns a preview; show it to the user and call again with confirm true after they approve.",
		InputSchema: schemaFor[commentIn](nil),
		Annotations: sendTool("Comment on ticket"),
	}, guard(s.log, "commentTicket", s.commentTicket))
}

// writeFailure tells a refused write (the API answered 4xx: nothing was stored) from one whose
// outcome is unknown (a timeout, a lost or unreadable answer, a server error), which must be
// checked before it is sent again.
func writeFailure(err error, refused, checkWith string) (outcome, message string) {
	var apiErr *api.Error
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		return "refused", refused
	}
	return "unknown", "the outcome is unknown, it may have been sent: check with " + checkWith + " before sending again"
}

func checkText(field, s string, limit int) (string, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return "", fmt.Errorf("%s is required", field)
	case !utf8.ValidString(s):
		return "", fmt.Errorf("%s is not valid UTF-8", field)
	case utf8.RuneCountInString(s) > limit:
		return "", fmt.Errorf("%s is longer than %d characters", field, limit)
	}
	return s, nil
}

func matchOption(opts []api.TicketOption, want string) (api.TicketOption, bool) {
	w := strings.TrimSpace(want)
	for _, o := range opts {
		if strings.EqualFold(o.Code, w) || strings.EqualFold(o.Name, w) {
			return o, true
		}
	}
	return api.TicketOption{}, false
}

func optionCodes(opts []api.TicketOption, allowed []string) string {
	var cs []string
	for _, o := range opts {
		if allowed == nil || slices.Contains(allowed, o.Code) {
			cs = append(cs, o.Code)
		}
	}
	return strings.Join(cs, ", ")
}

func resolveUnit(objs []api.TicketObject, want string) (api.TicketObject, error) {
	w := strings.TrimSpace(want)
	if w == "" {
		var own []api.TicketObject
		for _, o := range objs {
			if o.Own {
				own = append(own, o)
			}
		}
		for _, o := range own {
			if o.Type == ownUnitPreferred {
				return o, nil
			}
		}
		if len(own) == 1 {
			return own[0], nil
		}
		return api.TicketObject{}, errors.New("unit is required: ticket_options lists the units")
	}
	var byName []api.TicketObject
	for _, o := range objs {
		if strconv.FormatInt(o.ID, 10) == w {
			return o, nil
		}
		if strings.EqualFold(o.Name, w) {
			byName = append(byName, o)
		}
	}
	switch len(byName) {
	case 1:
		return byName[0], nil
	case 0:
		return api.TicketObject{}, fmt.Errorf("no unit %q: ticket_options lists the units", want)
	}
	return api.TicketObject{}, fmt.Errorf("several units are named %q: give the unit id from ticket_options", want)
}

func (s *Server) createTicket(ctx context.Context, req *mcp.CallToolRequest, in createTicketIn) (*mcp.CallToolResult, any, error) {
	subject, err := checkText("subject", in.Subject, maxSubjectRunes)
	if err != nil {
		return nil, nil, err
	}
	desc, err := checkText("description", in.Description, maxTextRunes)
	if err != nil {
		return nil, nil, err
	}
	opts, err := s.api.TicketOptions(ctx)
	if err != nil {
		return nil, nil, err
	}
	cat, ok := matchOption(opts.Categories, in.Category)
	if !ok || !slices.Contains(ticketCategories, cat.Code) {
		return nil, nil, fmt.Errorf("category must be one of %s", optionCodes(opts.Categories, ticketCategories))
	}
	areaName := in.Area
	if strings.TrimSpace(areaName) == "" {
		areaName = defaultArea
	}
	area, ok := matchOption(opts.Areas, areaName)
	if !ok {
		return nil, nil, fmt.Errorf("area must be one of %s", optionCodes(opts.Areas, nil))
	}
	unit, err := resolveUnit(opts.Objects, in.Unit)
	if err != nil {
		return nil, nil, err
	}
	body := api.NewTicket{
		Subject: subject, Description: desc, CategoryID: cat.ID, AreaID: area.ID, Private: in.Private,
		RelatedEntities: []api.RelatedEntity{{EntityType: api.EntityObject, EntityID: strconv.FormatInt(unit.ID, 10)}},
	}
	preview := map[string]any{
		"category": cat.Name, "area": area.Name, "unit": unit.Name, "unit_id": unit.ID, "subject": subject, "description": desc, "private": in.Private,
	}
	client := clientName(req)
	if !in.Confirm {
		return nil, map[string]any{
			"sent": false, "preview": preview,
			"next": "show this preview to the user; after they approve it, call create_ticket again with the same arguments and confirm true",
		}, nil
	}
	t, err := s.api.CreateTicket(ctx, body)
	if err != nil {
		outcome, msg := writeFailure(err, "the ticket was not created", "list_tickets")
		s.log.Warn("write", "tool", "create_ticket", "client", client, "unit", unit.Name, "subject", subject, "outcome", outcome, "err", err.Error())
		return nil, nil, fmt.Errorf("%s: %w", msg, err)
	}
	s.log.Info("write", "tool", "create_ticket", "client", client, "unit", unit.Name, "category", cat.Code, "subject", subject, "ticket", t.ID, "outcome", "ok")
	return nil, map[string]any{"sent": true, "ticket": viewTicket(*t, true)}, nil
}

func (s *Server) commentTicket(ctx context.Context, req *mcp.CallToolRequest, in commentIn) (*mcp.CallToolResult, any, error) {
	if in.TicketID <= 0 {
		return nil, nil, errors.New("ticket_id is required")
	}
	msg, err := checkText("message", in.Message, maxTextRunes)
	if err != nil {
		return nil, nil, err
	}
	t, err := s.api.Ticket(ctx, in.TicketID)
	if err != nil {
		return nil, nil, notFound(err, "ticket", in.TicketID)
	}
	if t.NoComments {
		return nil, nil, errors.New("this ticket does not take comments")
	}
	v := viewTicket(*t, false)
	preview := map[string]any{"ticket": map[string]any{"id": v.ID, "subject": v.Subject, "state": v.State, "own": v.Own}, "message": msg}
	if !v.Own {
		preview["note"] = "this is another resident's ticket: the message is visible to its members"
	}
	client := clientName(req)
	if !in.Confirm {
		return nil, map[string]any{
			"sent": false, "preview": preview,
			"next": "show this preview to the user; after they approve it, call comment_ticket again with the same arguments and confirm true",
		}, nil
	}
	c, err := s.api.AddComment(ctx, in.TicketID, msg)
	if err != nil {
		outcome, text := writeFailure(err, "the comment was not posted", "get_ticket")
		s.log.Warn("write", "tool", "comment_ticket", "client", client, "ticket", in.TicketID, "outcome", outcome, "err", err.Error())
		return nil, nil, fmt.Errorf("%s: %w", text, err)
	}
	s.log.Info("write", "tool", "comment_ticket", "client", client, "ticket", in.TicketID, "comment", c.ID, "outcome", "ok")
	return nil, map[string]any{"sent": true, "ticket_id": in.TicketID, "comment": message{At: c.CreatedAt, Author: c.CreatorName, Text: strings.TrimSpace(c.Text)}}, nil
}
