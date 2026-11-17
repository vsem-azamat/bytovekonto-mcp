package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
	"github.com/vsem-azamat/bytovekonto-mcp/internal/text"
)

const (
	dateLayout       = "2006-01-02"
	defaultListLimit = 50
	maxListLimit     = 500
	// maxCostsRange bounds a costs or chart query, so one call cannot ask for years of hours.
	maxCostsRange = 400 * 24 * time.Hour
)

// Scopes and states of list_tickets.
const (
	scopeMine     = "mine"
	scopeBuilding = "building"
	scopeAll      = "all"
	stateOpen     = "open"
	stateClosed   = "closed"
	stateAny      = "all"
)

type noArgs struct{}

type listTicketsIn struct {
	Scope string `json:"scope,omitempty" jsonschema:"mine (default): tickets the user created; building: the building's public tickets of other residents about common areas; all: both"`
	State string `json:"state,omitempty" jsonschema:"open, closed or all (default)"`
	Query string `json:"query,omitempty" jsonschema:"free text matched against subject, description and last comment"`
	Limit int    `json:"limit,omitempty" jsonschema:"at most this many tickets, newest first (default 50)"`
}

type idIn struct {
	ID int64 `json:"id" jsonschema:"the id from the list tool"`
}

type paymentsIn struct {
	Filter string `json:"filter,omitempty" jsonschema:"to_pay (default): documents still to pay; overpaid; all: every document (use year and month to narrow)"`
	Year   int    `json:"year,omitempty" jsonschema:"only documents of this year"`
	Month  int    `json:"month,omitempty" jsonschema:"only documents of this month (1-12, with year)"`
}

type costsIn struct {
	From        string `json:"from,omitempty" jsonschema:"first day, YYYY-MM-DD (default 1 January of the year of to)"`
	To          string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD (default today)"`
	Granularity string `json:"granularity,omitempty" jsonschema:"day, week or month (default)"`
}

type meterIn struct {
	ID          int64  `json:"id" jsonschema:"the meter id from list_meters"`
	From        string `json:"from,omitempty" jsonschema:"first day of the consumption chart, YYYY-MM-DD (default 1 January of the year of to)"`
	To          string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD (default today)"`
	Granularity string `json:"granularity,omitempty" jsonschema:"hour, day or month (default)"`
}

type documentsIn struct {
	Query string `json:"query,omitempty" jsonschema:"free text matched against the name, file name and category"`
}

type documentIn struct {
	ID string `json:"id" jsonschema:"the document id from list_documents"`
}

func (s *Server) addReadTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "home_overview",
		Description: "The resident's situation in one call: the tenancy (unit, period, people, monthly payment), the building's management contacts, " +
			"current announcements, money past due and to pay, the user's open tickets with their last message, and unread notifications.",
		InputSchema: schemaFor[noArgs](nil),
		Annotations: readOnly("Home overview"),
	}, guard(s.log, "overview", s.overview))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "building_info",
		Description: "The building's owner and manager companies (with bank accounts), the management's contacts for residents (emergency line, technician, accounting), " +
			"announcements and the building's FAQ.",
		InputSchema: schemaFor[noArgs](nil),
		Annotations: readOnly("Building info"),
	}, guard(s.log, "buildingInfo", s.buildingInfo))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_tickets",
		Description: "List tickets (fault reports, requests, questions) with state, category, unit and the last message, newest first. " +
			"By default only the user's own; scope building shows other residents' public tickets about common areas.",
		InputSchema: schemaFor[listTicketsIn](map[string][]any{
			"scope": {scopeMine, scopeBuilding, scopeAll},
			"state": {stateOpen, stateClosed, stateAny},
		}),
		Annotations: readOnly("List tickets"),
	}, guard(s.log, "listTickets", s.listTickets))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_ticket",
		Description: "One ticket with its description, members, the whole conversation (oldest first) and its change history.",
		InputSchema: schemaFor[idIn](nil),
		Annotations: readOnly("Get ticket"),
	}, guard(s.log, "getTicket", s.getTicket))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "ticket_options",
		Description: "The categories, areas and units (own flat, cellar, entrance, building) a new ticket can use, as create_ticket accepts them.",
		InputSchema: schemaFor[noArgs](nil),
		Annotations: readOnly("Ticket options"),
	}, guard(s.log, "ticketOptions", s.ticketOptions))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_payments",
		Description: "Accounting documents (monthly prescriptions, invoices, settlements) with amount, paid and remaining amounts, due date and status. " +
			"Default: the documents still to pay.",
		InputSchema: schemaFor[paymentsIn](map[string][]any{"filter": {"to_pay", "overpaid", "all"}}),
		Annotations: readOnly("List payments"),
	}, guard(s.log, "listPayments", s.listPayments))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_payment",
		Description: "One accounting document with its items and the payment details (variable and specific symbol, due date).",
		InputSchema: schemaFor[idIn](nil),
		Annotations: readOnly("Get payment"),
	}, guard(s.log, "getPayment", s.getPayment))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "payments_overview",
		Description: "Totals of the account: past due (and since when), due, paid, overpaid, planned; and the same per month.",
		InputSchema: schemaFor[noArgs](nil),
		Annotations: readOnly("Payments overview"),
	}, guard(s.log, "paymentsOverview", s.paymentsOverview))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "costs",
		Description: "Costs of the account per delivery (rent, heat, hot water, water and sewage, electricity) and period, with consumption, " +
			"and the current rent. Costs of an unsettled period are estimates.",
		InputSchema: schemaFor[costsIn](map[string][]any{"granularity": {"day", "week", "month"}}),
		Annotations: readOnly("Costs"),
	}, guard(s.log, "costs", s.costs))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_meters",
		Description: "The utility meters of the account's units (electricity, heat, cold and hot water) with ids, serial names and whether they are active.",
		InputSchema: schemaFor[noArgs](nil),
		Annotations: readOnly("List meters"),
	}, guard(s.log, "listMeters", s.listMeters))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_meter",
		Description: "One meter's last readings per channel and its consumption per period between two dates.",
		InputSchema: schemaFor[meterIn](map[string][]any{"granularity": {"hour", "day", "month"}}),
		Annotations: readOnly("Get meter"),
	}, guard(s.log, "getMeter", s.getMeter))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_documents",
		Description: "Documents the management shares with the building's residents (house rules, manuals, notices), with category, file name and size.",
		InputSchema: schemaFor[documentsIn](nil),
		Annotations: readOnly("List documents"),
	}, guard(s.log, "listDocuments", s.listDocuments))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "download_document",
		Description: "Download one shared document. The file is saved on the server's host (the result names the path) and, when small enough, " +
			"also returned inline as an embedded resource.",
		InputSchema: schemaFor[documentIn](nil),
		Annotations: readOnly("Download document"),
	}, guard(s.log, "downloadDocument", s.downloadDocument))

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "notifications",
		Description: "The user's in-app notifications (new comments, ticket changes, new documents) as the API sends them, and the unread count.",
		InputSchema: schemaFor[noArgs](nil),
		Annotations: readOnly("Notifications"),
	}, guard(s.log, "notifications", s.notifications))
}

// views

type contactView struct {
	Name     string `json:"name"`
	Role     string `json:"role,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Email    string `json:"email,omitempty"`
	Note     string `json:"note,omitempty"`
	Validity string `json:"valid,omitempty"`
}

func viewContact(c api.Contact) contactView {
	v := contactView{Name: c.Name, Role: c.RelationshipName, Kind: c.SubType, Note: strings.TrimSpace(deref(c.Note))}
	if v.Role == c.Name {
		v.Role = ""
	}
	if c.Phone != nil {
		v.Phone = c.Phone.Value
	}
	if c.Email != nil {
		v.Email = c.Email.Value
	}
	if c.ValidFrom != nil || c.ValidTo != nil {
		v.Validity = strings.TrimSpace(datePart(deref(c.ValidFrom)) + " – " + datePart(deref(c.ValidTo)))
	}
	return v
}

type ticketView struct {
	ID          int64    `json:"id"`
	Subject     string   `json:"subject"`
	State       string   `json:"state"`
	Category    string   `json:"category"`
	Area        string   `json:"area,omitempty"`
	Unit        string   `json:"unit,omitempty"`
	Own         bool     `json:"own"`
	CreatedAt   string   `json:"created_at"`
	ModifiedAt  string   `json:"modified_at,omitempty"`
	ClosedAt    string   `json:"closed_at,omitempty"`
	Description string   `json:"description,omitempty"`
	LastComment *message `json:"last_comment,omitempty"`
}

type message struct {
	At     string `json:"at"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

func viewTicket(t api.Ticket, withDescription bool) ticketView {
	v := ticketView{
		ID: t.ID, Subject: strings.TrimSpace(t.Subject), State: t.State, Category: t.Category, Area: t.Area,
		Own: t.HasTag(api.TagUserIsCreator), CreatedAt: t.CreatedAt, ModifiedAt: t.ModifiedAt, ClosedAt: deref(t.ClosedAt),
	}
	for _, r := range t.Relations {
		if r.EntityType == api.EntityObject {
			v.Unit = deref(r.Label)
		}
	}
	if withDescription {
		v.Description = strings.TrimSpace(t.Description)
	}
	if t.LastCommentText != nil {
		v.LastComment = &message{At: deref(t.LastCommentCreatedAt), Author: deref(t.LastCommentAuthor), Text: strings.TrimSpace(*t.LastCommentText)}
	}
	return v
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func datePart(s string) string {
	if len(s) >= len(dateLayout) {
		return s[:len(dateLayout)]
	}
	return s
}

func announcements(boxes []api.Infobox) []string {
	out := []string{}
	for _, b := range boxes {
		if t := text.HTML(b.HTMLContent); t != "" {
			if b.LinkURL != nil && *b.LinkURL != "" {
				t += " (" + *b.LinkURL + ")"
			}
			out = append(out, t)
		}
	}
	return out
}

// tools

type tenancyView struct {
	Name     string        `json:"name"`
	Kind     string        `json:"kind"`
	Active   bool          `json:"active"`
	From     string        `json:"from"`
	To       string        `json:"to"`
	Units    []string      `json:"units"`
	People   []contactView `json:"people"`
	Monthly  any           `json:"monthly_payment,omitempty"`
	Building string        `json:"building,omitempty"`
}

func (s *Server) overview(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	var (
		accounts []api.HomeAccount
		detail   *api.HomeAccountDetail
		contacts []api.Contact
		boxes    []api.Infobox
		tickets  []api.Ticket
		pay      *api.PaymentsOverview
		toPay    []api.Payment
		unread   int
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { accounts, err = s.api.HomeAccounts(gctx); return })
	g.Go(func() (err error) { detail, err = s.api.HomeAccount(gctx); return })
	g.Go(func() (err error) { contacts, err = s.api.Contacts(gctx); return })
	g.Go(func() (err error) { boxes, err = s.api.Infoboxes(gctx); return })
	g.Go(func() (err error) { tickets, err = s.api.Tickets(gctx); return })
	g.Go(func() (err error) { pay, err = s.api.PaymentsOverview(gctx); return })
	g.Go(func() (err error) {
		toPay, err = s.api.Payments(gctx, api.PaymentQuery{Filter: api.PaymentsToPay})
		return
	})
	g.Go(func() (err error) { unread, err = s.api.NotificationCount(gctx); return })
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	ten := tenancyView{Name: detail.Name, Kind: detail.SubType, Active: detail.Active, From: datePart(detail.ActiveFrom), To: datePart(detail.ActiveTo), Units: []string{}, People: []contactView{}}
	for _, a := range accounts {
		if a.ID == detail.ID {
			ten.Building = a.ProjectName
		}
	}
	for _, o := range detail.Objects {
		ten.Units = append(ten.Units, strings.TrimSpace(o.Name+" "+o.Address))
	}
	for _, p := range detail.Subjects {
		ten.People = append(ten.People, viewContact(p))
	}
	if p := detail.Prescription; p != nil {
		ten.Monthly = map[string]any{
			"amount": p.TotalAmount, "currency": p.Currency, "issued_on_day": p.IssueDay, "due_days": p.DueDays,
			"to_account": p.BankAccountNumber + "/" + p.BankCode, "variable_symbol": p.VS, "specific_symbol": p.SS, "payee": p.SupplierName,
		}
	}

	open := []ticketView{}
	for _, t := range tickets {
		if t.HasTag(api.TagUserIsCreator) && t.Open() {
			open = append(open, viewTicket(t, false))
		}
	}
	sortTickets(open)

	toPayView := make([]paymentView, 0, len(toPay))
	for _, p := range toPay {
		toPayView = append(toPayView, viewPayment(p))
	}
	cv := make([]contactView, 0, len(contacts))
	for _, c := range contacts {
		cv = append(cv, viewContact(c))
	}
	out := map[string]any{
		"tenancy":              ten,
		"management_contacts":  cv,
		"announcements":        announcements(boxes),
		"money":                map[string]any{"past_due": pay.Overview.PastDue, "oldest_past_due_date": deref(pay.Overview.OldestPastDueDate), "due": pay.Overview.Due, "overpaid": pay.Overview.Overpaid, "to_pay": toPayView},
		"open_tickets":         open,
		"unread_notifications": unread,
	}
	return nil, out, nil
}

func (s *Server) buildingInfo(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	var (
		parties  []api.ProjectParty
		contacts []api.Contact
		boxes    []api.Infobox
		faq      []api.FAQ
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { parties, err = s.api.ProjectParties(gctx); return })
	g.Go(func() (err error) { contacts, err = s.api.Contacts(gctx); return })
	g.Go(func() (err error) { boxes, err = s.api.Infoboxes(gctx); return })
	g.Go(func() (err error) { faq, err = s.api.FAQ(gctx); return })
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}
	type company struct {
		Name     string        `json:"name"`
		Role     string        `json:"role"`
		Address  string        `json:"address,omitempty"`
		ID       string        `json:"company_id,omitempty"`
		VATID    string        `json:"vat_id,omitempty"`
		Accounts []string      `json:"bank_accounts,omitempty"`
		Contacts []contactView `json:"contacts,omitempty"`
	}
	cs := make([]company, 0, len(parties))
	for _, p := range parties {
		c := company{Name: p.Name, Role: partyRole(p.RelatedToUnitAs), Address: p.Address, ID: p.Identifier, VATID: p.VATID}
		for _, b := range p.BankConnections {
			acc := b.AccountIdentifier + "/" + b.BankCode
			if b.IBAN != nil && *b.IBAN != "" {
				acc += " (IBAN " + *b.IBAN + ")"
			}
			c.Accounts = append(c.Accounts, acc)
		}
		for _, x := range p.Contacts {
			c.Contacts = append(c.Contacts, viewContact(x))
		}
		cs = append(cs, c)
	}
	cv := make([]contactView, 0, len(contacts))
	for _, c := range contacts {
		cv = append(cv, viewContact(c))
	}
	return nil, map[string]any{"companies": cs, "management_contacts": cv, "announcements": announcements(boxes), "faq": faq}, nil
}

func partyRole(as string) string {
	switch as {
	case "PRIMARY":
		return "owner"
	case "IDENTITY":
		return "manager"
	}
	return strings.ToLower(as)
}

func (s *Server) listTickets(ctx context.Context, _ *mcp.CallToolRequest, in listTicketsIn) (*mcp.CallToolResult, any, error) {
	scope := strings.ToLower(strings.TrimSpace(in.Scope))
	if scope == "" {
		scope = scopeMine
	}
	state := strings.ToLower(strings.TrimSpace(in.State))
	if state == "" {
		state = stateAny
	}
	if !slices.Contains([]string{scopeMine, scopeBuilding, scopeAll}, scope) {
		return nil, nil, fmt.Errorf("scope must be %s, %s or %s", scopeMine, scopeBuilding, scopeAll)
	}
	if !slices.Contains([]string{stateOpen, stateClosed, stateAny}, state) {
		return nil, nil, fmt.Errorf("state must be %s, %s or %s", stateOpen, stateClosed, stateAny)
	}
	limit, err := listLimit(in.Limit)
	if err != nil {
		return nil, nil, err
	}
	tickets, err := s.api.Tickets(ctx)
	if err != nil {
		return nil, nil, err
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	out := []ticketView{}
	for _, t := range tickets {
		own := t.HasTag(api.TagUserIsCreator)
		if (scope == scopeMine && !own) || (scope == scopeBuilding && own) {
			continue
		}
		if (state == stateOpen && !t.Open()) || (state == stateClosed && t.Open()) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(t.Subject+"\n"+t.Description+"\n"+deref(t.LastCommentText)), q) {
			continue
		}
		out = append(out, viewTicket(t, false))
	}
	sortTickets(out)
	total := len(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return nil, map[string]any{"tickets": out, "total": total}, nil
}

func listLimit(n int) (int, error) {
	switch {
	case n == 0:
		return defaultListLimit, nil
	case n < 0 || n > maxListLimit:
		return 0, fmt.Errorf("limit must be 1..%d", maxListLimit)
	}
	return n, nil
}

func sortTickets(ts []ticketView) {
	slices.SortStableFunc(ts, func(a, b ticketView) int { return strings.Compare(b.CreatedAt, a.CreatedAt) })
}

func (s *Server) getTicket(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, any, error) {
	if in.ID <= 0 {
		return nil, nil, errors.New("id is required")
	}
	var (
		t  *api.Ticket
		lc *api.TicketLifecycle
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { t, err = s.api.Ticket(gctx, in.ID); return })
	g.Go(func() (err error) { lc, err = s.api.TicketLifecycle(gctx, in.ID); return })
	if err := g.Wait(); err != nil {
		return nil, nil, notFound(err, "ticket", in.ID)
	}
	v := viewTicket(*t, true)
	members := []map[string]string{}
	for _, m := range t.Members {
		e := map[string]string{"name": m.Label, "role": strings.ToLower(m.RoleType)}
		if m.Phone != nil {
			e["phone"] = *m.Phone
		}
		if m.Email != nil {
			e["email"] = *m.Email
		}
		members = append(members, e)
	}
	conv := []message{}
	for _, c := range lc.Comments {
		if c.DeletedAt != nil {
			continue
		}
		conv = append(conv, message{At: c.CreatedAt, Author: c.CreatorName, Text: strings.TrimSpace(c.Text)})
	}
	slices.SortStableFunc(conv, func(a, b message) int { return strings.Compare(a.At, b.At) })
	hist := []map[string]any{}
	for _, h := range lc.History {
		e := map[string]any{"at": h.Metadata.Created, "by": h.Metadata.SubjectName, "event": h.Metadata.EventType}
		if len(h.Changes) > 0 {
			e["changes"] = h.Changes
		}
		hist = append(hist, e)
	}
	out := map[string]any{
		"ticket": v, "priority": t.Priority, "private": t.Private, "members": members,
		"conversation": conv, "history": hist, "attachments": len(t.Attachments),
		"can_comment": !t.NoComments,
	}
	if t.DueDate != nil {
		out["due_date"] = *t.DueDate
	}
	return nil, out, nil
}

// notFound turns a 404 or 403 into a plain message for the agent.
func notFound(err error, what string, id any) error {
	if api.IsStatus(err, 404) || api.IsStatus(err, 403) {
		return fmt.Errorf("no %s %v is visible to this account", what, id)
	}
	return err
}

func (s *Server) ticketOptions(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	o, err := s.api.TicketOptions(ctx)
	if err != nil {
		return nil, nil, err
	}
	cats := []map[string]any{}
	for _, c := range o.Categories {
		if slices.Contains(ticketCategories, c.Code) {
			cats = append(cats, map[string]any{"code": c.Code, "name": c.Name})
		}
	}
	areas := []map[string]any{}
	for _, a := range o.Areas {
		areas = append(areas, map[string]any{"code": a.Code, "name": a.Name})
	}
	units := []map[string]any{}
	for _, u := range o.Objects {
		units = append(units, map[string]any{"id": u.ID, "name": u.Name, "type": u.Type, "own": u.Own})
	}
	return nil, map[string]any{"categories": cats, "areas": areas, "units": units, "default_area": defaultArea}, nil
}

type paymentView struct {
	ID        int64   `json:"id"`
	Number    string  `json:"number"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
	Paid      float64 `json:"paid"`
	Remaining float64 `json:"remaining"`
	Currency  string  `json:"currency"`
	Created   string  `json:"created"`
	Due       string  `json:"due,omitempty"`
	PaidOn    string  `json:"paid_on,omitempty"`
	Supplier  string  `json:"supplier,omitempty"`
}

func viewPayment(p api.Payment) paymentView {
	name := p.PrescriptionName
	if name == "" {
		name = deref(p.SettlementName)
	}
	if name == "" {
		name = p.DocumentName
	}
	return paymentView{
		ID: p.ID, Number: p.Number, Name: name, Type: p.DocumentName, Status: p.Status, Amount: p.TotalAmount, Paid: p.TotalAmountPaid,
		Remaining: p.RemainingUnpaid, Currency: p.Currency, Created: datePart(p.CreatedDate), Due: datePart(deref(p.DueDate)),
		PaidOn: datePart(deref(p.PaymentDate)), Supplier: p.SupplierName,
	}
}

func (s *Server) listPayments(ctx context.Context, _ *mcp.CallToolRequest, in paymentsIn) (*mcp.CallToolResult, any, error) {
	q := api.PaymentQuery{Year: in.Year, Month: in.Month}
	switch strings.ToLower(strings.TrimSpace(in.Filter)) {
	case "", "to_pay":
		q.Filter = api.PaymentsToPay
	case "overpaid":
		q.Filter = api.PaymentsOverpaid
	case "all":
	default:
		return nil, nil, errors.New("filter must be to_pay, overpaid or all")
	}
	if in.Month < 0 || in.Month > 12 || (in.Month > 0 && in.Year == 0) {
		return nil, nil, errors.New("month must be 1..12 and needs a year")
	}
	ps, err := s.api.Payments(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	out := make([]paymentView, 0, len(ps))
	for _, p := range ps {
		out = append(out, viewPayment(p))
	}
	return nil, map[string]any{"payments": out}, nil
}

func (s *Server) getPayment(ctx context.Context, _ *mcp.CallToolRequest, in idIn) (*mcp.CallToolResult, any, error) {
	if in.ID <= 0 {
		return nil, nil, errors.New("id is required")
	}
	p, err := s.api.Payment(ctx, in.ID)
	if err != nil {
		return nil, nil, notFound(err, "payment", in.ID)
	}
	items := []map[string]any{}
	for _, it := range p.Items {
		e := map[string]any{"amount": it.SumWithVAT}
		if d := strings.TrimSpace(deref(it.Description)); d != "" {
			e["description"] = d
		}
		if d := strings.TrimSpace(deref(it.DeliveryName)); d != "" {
			e["delivery"] = d
		}
		if it.ItemCount != 0 {
			e["count"] = it.ItemCount
			e["unit"] = deref(it.ItemUnit)
		}
		if it.PeriodFrom != nil || it.PeriodTo != nil {
			e["period"] = datePart(deref(it.PeriodFrom)) + " – " + datePart(deref(it.PeriodTo))
		}
		items = append(items, e)
	}
	name := deref(p.PrescriptionName)
	if name == "" {
		name = deref(p.SettlementName)
	}
	out := map[string]any{
		"id": p.ID, "number": p.Number, "name": name, "type": p.DocumentName, "status": p.Status, "document_status": p.DocumentStatus,
		"payment_type": p.PaymentType, "amount": p.TotalAmount, "paid": p.TotalAmountPaid, "remaining": p.RemainingUnpaid,
		"currency": p.Currency, "created": datePart(p.CreatedDate), "due": datePart(deref(p.DueDate)), "paid_on": datePart(deref(p.PaymentDate)),
		"variable_symbol": deref(p.VS), "specific_symbol": deref(p.SS), "constant_symbol": deref(p.KS),
		"period": strings.Trim(datePart(deref(p.PeriodFrom))+" – "+datePart(deref(p.PeriodTo)), " –"),
		"note":   strings.TrimSpace(deref(p.NotePublic)), "items": items,
	}
	return nil, out, nil
}

func (s *Server) paymentsOverview(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	o, err := s.api.PaymentsOverview(ctx)
	if err != nil {
		return nil, nil, err
	}
	return nil, o, nil
}

// dateRange parses from and to, defaulting to 1 January of this year and today.
func (s *Server) dateRange(from, to string) (string, string, error) {
	if to == "" {
		to = s.opts.Now().Format(dateLayout)
	}
	t, err := time.Parse(dateLayout, to)
	if err != nil {
		return "", "", errors.New("to must be YYYY-MM-DD")
	}
	if from == "" {
		from = time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.UTC).Format(dateLayout)
	}
	f, err := time.Parse(dateLayout, from)
	if err != nil {
		return "", "", errors.New("from must be YYYY-MM-DD")
	}
	if t.Before(f) {
		return "", "", errors.New("to is before from")
	}
	if t.Sub(f) > maxCostsRange {
		return "", "", errors.New("the range is limited to about 13 months")
	}
	return from, to, nil
}

func costsGranularity(g string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(g)) {
	case "", "month":
		return api.GranularityMonth, nil
	case "week":
		return api.GranularityWeek, nil
	case "day":
		return api.GranularityDay, nil
	}
	return "", errors.New("granularity must be day, week or month")
}

func (s *Server) costs(ctx context.Context, _ *mcp.CallToolRequest, in costsIn) (*mcp.CallToolResult, any, error) {
	gran, err := costsGranularity(in.Granularity)
	if err != nil {
		return nil, nil, err
	}
	from, to, err := s.dateRange(in.From, in.To)
	if err != nil {
		return nil, nil, err
	}
	var (
		c    *api.Costs
		rent *api.Rent
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { c, err = s.api.Costs(gctx, from, to, gran); return })
	g.Go(func() (err error) { rent, err = s.api.Rent(gctx); return })
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}
	labels := make([]string, 0, len(c.Periods))
	for _, p := range c.Periods {
		labels = append(labels, datePart(p.From))
	}
	dels := []map[string]any{}
	for _, d := range c.Deliveries {
		e := map[string]any{
			"name": strings.TrimSpace(d.Label), "category": d.Category, "total_cost": d.TotalCost, "cost_basis": strings.ToLower(d.TotalCostType),
			"per_period_cost": d.PeriodCost,
		}
		if d.ConsumptionUnit != nil {
			e["consumption"] = d.Consumption
			e["unit"] = *d.ConsumptionUnit
			e["per_period_consumption"] = d.PeriodConsumption
		}
		dels = append(dels, e)
	}
	out := map[string]any{
		"currency": c.Currency, "from": from, "to": to, "period_starts": labels, "deliveries": dels,
		"settled": c.PeriodType != "UNSETTLED",
	}
	if c.DataAvailableTo != "" {
		out["data_until"] = c.DataAvailableTo
	}
	if rent.Current != nil {
		out["rent"] = map[string]any{"amount": rent.Current.Value, "currency": rent.Currency, "since": datePart(rent.Current.From)}
	}
	return nil, out, nil
}

type meterView struct {
	ID     int64    `json:"id"`
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	Model  string   `json:"model,omitempty"`
	Units  []string `json:"units"`
	Active bool     `json:"active"`
}

func viewMeter(m api.Meter) meterView {
	return meterView{ID: m.ID, Kind: strings.ToLower(m.TypeCode), Name: m.Name, Model: m.ModelName, Units: m.ObjectNames, Active: m.Active}
}

func (s *Server) listMeters(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	ms, err := s.api.Meters(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make([]meterView, 0, len(ms))
	for _, m := range ms {
		out = append(out, viewMeter(m))
	}
	return nil, map[string]any{"meters": out}, nil
}

func (s *Server) getMeter(ctx context.Context, _ *mcp.CallToolRequest, in meterIn) (*mcp.CallToolResult, any, error) {
	if in.ID <= 0 {
		return nil, nil, errors.New("id is required")
	}
	gran := api.ChartMonthly
	switch strings.ToLower(strings.TrimSpace(in.Granularity)) {
	case "", "month":
	case "day":
		gran = api.ChartDaily
	case "hour":
		gran = api.ChartHourly
	default:
		return nil, nil, errors.New("granularity must be hour, day or month")
	}
	from, to, err := s.dateRange(in.From, in.To)
	if err != nil {
		return nil, nil, err
	}
	var (
		m     *api.Meter
		chart *api.MeterChart
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { m, err = s.api.Meter(gctx, in.ID); return })
	g.Go(func() (err error) { chart, err = s.api.MeterChart(gctx, in.ID, from, to, gran); return })
	if err := g.Wait(); err != nil {
		return nil, nil, notFound(err, "meter", in.ID)
	}
	last := []map[string]any{}
	for name, v := range m.LastValues {
		if v.LastValue == nil {
			continue
		}
		last = append(last, map[string]any{"channel": name, "value": *v.LastValue, "unit": v.Unit, "at": deref(v.LastValueTime)})
	}
	slices.SortFunc(last, func(a, b map[string]any) int {
		return strings.Compare(fmt.Sprint(a["channel"]), fmt.Sprint(b["channel"]))
	})
	channels := map[string]any{}
	for name, c := range chart.DataByChannel {
		channels[name] = map[string]any{"total": c.TotalConsumption, "unit": c.Unit, "per_period": c.Consumptions}
	}
	out := map[string]any{
		"meter": viewMeter(*m), "last_readings": last, "last_received": deref(m.LastValueReceived),
		"consumption": map[string]any{"from": from, "to": to, "labels": chart.Labels, "channels": channels},
	}
	return nil, out, nil
}

type documentView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	File     string `json:"file"`
	Type     string `json:"content_type"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

func viewDocument(d api.Document) documentView {
	v := documentView{ID: d.ID, Name: d.Name, File: d.FileMeta.Filename, Type: d.FileMeta.ContentType, Size: d.FileMeta.Size, Modified: datePart(d.ModifiedAt)}
	if d.Category != nil {
		v.Category = d.Category.Name
	}
	return v
}

func (s *Server) listDocuments(ctx context.Context, _ *mcp.CallToolRequest, in documentsIn) (*mcp.CallToolResult, any, error) {
	ds, err := s.api.Documents(ctx)
	if err != nil {
		return nil, nil, err
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	out := []documentView{}
	for _, d := range ds {
		v := viewDocument(d)
		if q != "" && !strings.Contains(strings.ToLower(v.Name+"\n"+v.File+"\n"+v.Category), q) {
			continue
		}
		out = append(out, v)
	}
	slices.SortStableFunc(out, func(a, b documentView) int { return strings.Compare(b.Modified, a.Modified) })
	return nil, map[string]any{"documents": out}, nil
}

func (s *Server) notifications(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	var (
		n      *api.Notifications
		unread int
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { n, err = s.api.Notifications(gctx); return })
	g.Go(func() (err error) { unread, err = s.api.NotificationCount(gctx); return })
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}
	list := n.Notifications
	if list == nil {
		list = []json.RawMessage{}
	}
	return nil, map[string]any{"unread": unread, "notifications": list}, nil
}
