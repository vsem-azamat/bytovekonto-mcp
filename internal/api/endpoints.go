package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
)

// HomeAccounts lists the user's tenancies and ownerships.
func (c *Client) HomeAccounts(ctx context.Context) ([]HomeAccount, error) {
	var out []HomeAccount
	return out, c.get(ctx, "/home-account", nil, &out)
}

// HomeAccount is the current home account: its people, units and monthly payment.
func (c *Client) HomeAccount(ctx context.Context) (*HomeAccountDetail, error) {
	var out HomeAccountDetail
	return &out, c.get(ctx, "/home-account/detail", nil, &out)
}

// ProjectParties are the building's owner and manager with their bank accounts and contacts.
func (c *Client) ProjectParties(ctx context.Context) ([]ProjectParty, error) {
	var out []ProjectParty
	return out, c.get(ctx, "/project/info", nil, &out)
}

// Contacts are the management's contacts for residents (emergency line, technician, …).
func (c *Client) Contacts(ctx context.Context) ([]Contact, error) {
	var out []Contact
	return out, c.get(ctx, "/project/contacts", nil, &out)
}

// Infoboxes are the building's announcement banners.
func (c *Client) Infoboxes(ctx context.Context) ([]Infobox, error) {
	var out []Infobox
	return out, c.get(ctx, "/project/infoboxes", nil, &out)
}

// FAQ is the building's questions and answers.
func (c *Client) FAQ(ctx context.Context) ([]FAQ, error) {
	var out []FAQ
	return out, c.get(ctx, "/project/faq", nil, &out)
}

// Tickets lists every ticket the user can see: their own and the building's public ones.
func (c *Client) Tickets(ctx context.Context) ([]Ticket, error) {
	var out []Ticket
	return out, c.get(ctx, "/tickets/list-with-last-comment", nil, &out)
}

// Ticket is one ticket.
func (c *Client) Ticket(ctx context.Context, id int64) (*Ticket, error) {
	var out Ticket
	return &out, c.get(ctx, "/tickets/"+strconv.FormatInt(id, 10), nil, &out)
}

// TicketLifecycle is a ticket's comments and change history.
func (c *Client) TicketLifecycle(ctx context.Context, id int64) (*TicketLifecycle, error) {
	var out TicketLifecycle
	return &out, c.get(ctx, "/tickets/"+strconv.FormatInt(id, 10)+"/lifecycle", nil, &out)
}

// TicketOptions are the categories, areas and units a new ticket can use.
func (c *Client) TicketOptions(ctx context.Context) (*TicketOptions, error) {
	var out TicketOptions
	return &out, c.get(ctx, "/tickets/options", nil, &out)
}

// CreateTicket sends a new ticket to the management and returns it as created.
func (c *Client) CreateTicket(ctx context.Context, t NewTicket) (*Ticket, error) {
	if t.Attachments == nil {
		t.Attachments = []string{}
	}
	if t.Actions == nil {
		t.Actions = []string{}
	}
	var out Ticket
	return &out, c.post(ctx, "/tickets", t, &out)
}

// AddComment posts a message to a ticket and returns the new comment.
func (c *Client) AddComment(ctx context.Context, ticketID int64, message string) (*Comment, error) {
	body := NewComment{Message: message, Attachments: []string{}}
	var out Comment
	return &out, c.post(ctx, "/tickets/"+strconv.FormatInt(ticketID, 10)+"/comment", body, &out)
}

// PaymentQuery selects payments: a filter (PaymentsToPay, PaymentsOverpaid, or "" for all), and
// optionally a year and a month.
type PaymentQuery struct {
	Filter string
	Year   int
	Month  int
}

// Payments lists accounting documents.
func (c *Client) Payments(ctx context.Context, q PaymentQuery) ([]Payment, error) {
	v := url.Values{}
	if q.Filter != "" {
		v.Set("filter", q.Filter)
	}
	if q.Year > 0 {
		v.Set("year", strconv.Itoa(q.Year))
	}
	if q.Month > 0 {
		v.Set("month", strconv.Itoa(q.Month))
	}
	var out []Payment
	return out, c.get(ctx, "/payments", v, &out)
}

// Payment is one accounting document with its items.
func (c *Client) Payment(ctx context.Context, id int64) (*PaymentDetail, error) {
	var out PaymentDetail
	return &out, c.get(ctx, "/payments/"+strconv.FormatInt(id, 10), nil, &out)
}

// PaymentsOverview sums the payments: past due, due, paid, overpaid, and per month.
func (c *Client) PaymentsOverview(ctx context.Context) (*PaymentsOverview, error) {
	var out PaymentsOverview
	return &out, c.get(ctx, "/payments/overview", nil, &out)
}

// Costs are the account's costs per delivery between two dates (YYYY-MM-DD).
func (c *Client) Costs(ctx context.Context, from, to, granularity string) (*Costs, error) {
	v := url.Values{"from": {from}, "to": {to}, "granularity": {granularity}}
	var out Costs
	return &out, c.get(ctx, "/costs", v, &out)
}

// Rent is the current rent.
func (c *Client) Rent(ctx context.Context) (*Rent, error) {
	var out Rent
	return &out, c.get(ctx, "/costs/rent", nil, &out)
}

// Meters lists the utility meters of the account's units.
func (c *Client) Meters(ctx context.Context) ([]Meter, error) {
	var out []Meter
	return out, c.get(ctx, "/metering/", nil, &out)
}

// Meter is one meter with its last readings.
func (c *Client) Meter(ctx context.Context, id int64) (*Meter, error) {
	var out Meter
	return &out, c.get(ctx, "/metering/"+strconv.FormatInt(id, 10), nil, &out)
}

// MeterChart is a meter's consumption between two dates (YYYY-MM-DD). The query parameter is
// spelled "granurality", as the API expects it.
func (c *Client) MeterChart(ctx context.Context, id int64, from, to, granularity string) (*MeterChart, error) {
	v := url.Values{"from": {from}, "to": {to}, "granurality": {granularity}}
	var out MeterChart
	return &out, c.get(ctx, "/metering/"+strconv.FormatInt(id, 10)+"/chartData", v, &out)
}

// Documents lists the building's shared documents.
func (c *Client) Documents(ctx context.Context) ([]Document, error) {
	var out []Document
	return out, c.get(ctx, "/documents/myProject", nil, &out)
}

// Download is a downloaded file.
type Download struct {
	Body        io.ReadCloser
	ContentType string
}

// DownloadDocument opens a shared document's file; the caller closes Body.
func (c *Client) DownloadDocument(ctx context.Context, d Document) (*Download, error) {
	if d.ID == "" || d.FileMeta.Filename == "" {
		return nil, errors.New("the document has no file")
	}
	resp, err := c.send(ctx, http.MethodGet, //nolint:bodyclose // the caller closes Download.Body
		"/documents/myProject/download/"+url.PathEscape(d.ID)+"/"+url.PathEscape(d.FileMeta.Filename), nil, nil, "")
	if err != nil {
		return nil, err
	}
	ct := resp.Header.Get("Content-Type")
	if mt, _, err := mime.ParseMediaType(ct); err == nil {
		ct = mt
	}
	return &Download{Body: resp.Body, ContentType: ct}, nil
}

// Notifications are the user's in-app notifications.
func (c *Client) Notifications(ctx context.Context) (*Notifications, error) {
	var out Notifications
	return &out, c.get(ctx, "/notifications/list", nil, &out)
}

// NotificationCount is the number of unread notifications.
func (c *Client) NotificationCount(ctx context.Context) (int, error) {
	var n int
	if err := c.get(ctx, "/notifications/count", nil, &n); err != nil {
		return 0, fmt.Errorf("notifications: %w", err)
	}
	return n, nil
}
