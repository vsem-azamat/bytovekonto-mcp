package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sync/errgroup"

	"github.com/vsem-azamat/bytovekonto-mcp/internal/api"
)

// maxQRDocuments bounds the documents of one payment_qr call.
const maxQRDocuments = 50

type paymentQRIn struct {
	IDs []int64 `json:"ids,omitempty" jsonschema:"the documents to pay, ids from list_payments (default: every document still to pay)"`
}

type compareCostsIn struct {
	From        string `json:"from,omitempty" jsonschema:"first day, YYYY-MM-DD (default the first day of the month of to, or of last month)"`
	To          string `json:"to,omitempty" jsonschema:"last day, YYYY-MM-DD (default the last day of the month of from)"`
	Granularity string `json:"granularity,omitempty" jsonschema:"day, week or month (default)"`
}

func (s *Server) addMoneyTools(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name: "payment_qr",
		Description: "A QR code (Czech QR payment) to pay accounting documents with a banking app: the PNG image inline and saved on the server's host, " +
			"with the documents and the amount it pays. Pays nothing by itself.",
		InputSchema: schemaFor[paymentQRIn](nil),
		Annotations: readOnly("Payment QR code"),
	}, guard(s.log, "paymentQR", s.paymentQR))

	mcp.AddTool(srv, &mcp.Tool{
		Name: "compare_costs",
		Description: "The account's cost and consumption per delivery (heat, hot water, water, electricity) in a period, compared with the previous period, " +
			"the same period a year ago, the building's average and the average of all buildings, where the API has the data.",
		InputSchema: schemaFor[compareCostsIn](map[string][]any{"granularity": {"day", "week", "month"}}),
		Annotations: readOnly("Compare costs"),
	}, guard(s.log, "compareCosts", s.compareCosts))
}

func (s *Server) paymentQR(ctx context.Context, _ *mcp.CallToolRequest, in paymentQRIn) (*mcp.CallToolResult, any, error) {
	ids := slices.Clone(in.IDs)
	if len(ids) == 0 {
		ps, err := s.api.Payments(ctx, api.PaymentQuery{Filter: api.PaymentsToPay})
		if err != nil {
			return nil, nil, err
		}
		for _, p := range ps {
			if p.RemainingUnpaid > 0 {
				ids = append(ids, p.ID)
			}
		}
		if len(ids) == 0 {
			return nil, map[string]any{"codes": []any{}, "note": "nothing is left to pay"}, nil
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) > maxQRDocuments {
		return nil, nil, fmt.Errorf("at most %d documents at once: give ids from list_payments", maxQRDocuments)
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, nil, errors.New("ids must be document ids from list_payments")
		}
	}
	qrs, err := s.api.PaymentQRCodes(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	if len(qrs) == 0 {
		return nil, nil, errors.New("the API made no QR code: the documents may be paid already or not payable by bank transfer")
	}
	res := &mcp.CallToolResult{}
	codes := make([]map[string]any, 0, len(qrs))
	for _, q := range qrs {
		png, err := base64.StdEncoding.DecodeString(q.PNGBase64)
		if err != nil || len(png) == 0 {
			return nil, nil, errors.New("the API sent an unreadable QR code")
		}
		docs := make([]paymentView, 0, len(q.Documents))
		var total float64
		var docIDs []string
		currency := ""
		for _, d := range q.Documents {
			docs = append(docs, viewPayment(d))
			total += d.RemainingUnpaid
			docIDs = append(docIDs, strconv.FormatInt(d.ID, 10))
			currency = d.Currency
		}
		code := map[string]any{"documents": docs, "amount": round2(total), "currency": currency}
		if s.opts.DownloadDir != "" {
			path, err := s.saveFile(qrFileName(docIDs), png)
			if err != nil {
				return nil, nil, err
			}
			code["saved_to"] = path
		}
		codes = append(codes, code)
		res.Content = append(res.Content, &mcp.ImageContent{Data: png, MIMEType: "image/png"})
	}
	out := map[string]any{"codes": codes}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, nil, fmt.Errorf("encode result: %w", err)
	}
	res.Content = append([]mcp.Content{&mcp.TextContent{Text: string(b)}}, res.Content...)
	return res, out, nil
}

// maxQRNameIDs is how many document ids a QR code's file name lists before it uses a hash.
const maxQRNameIDs = 5

// qrFileName names a QR code's file after its documents, or a hash of them when there are many.
func qrFileName(ids []string) string {
	name := strings.Join(ids, "-")
	if len(ids) > maxQRNameIDs {
		sum := sha256.Sum256([]byte(name))
		name = hex.EncodeToString(sum[:6])
	}
	return safeFileName("qr", name+".png")
}

// lastMonth is the first and last day of the month before now.
func lastMonth(now time.Time) (string, string) {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	return first.Format(dateLayout), first.AddDate(0, 1, -1).Format(dateLayout)
}

var comparisonNames = map[string]string{
	api.ComparePreviousPeriod: "previous_period",
	api.CompareYearAgo:        "year_ago",
	api.CompareProjectAverage: "building_average",
	api.CompareGlobalAverage:  "all_buildings_average",
}

func (s *Server) compareCosts(ctx context.Context, _ *mcp.CallToolRequest, in compareCostsIn) (*mcp.CallToolResult, any, error) {
	gran, err := costsGranularity(in.Granularity)
	if err != nil {
		return nil, nil, err
	}
	from, to := strings.TrimSpace(in.From), strings.TrimSpace(in.To)
	switch {
	case from == "" && to == "":
		from, to = lastMonth(s.opts.Now())
	case to == "":
		f, err := time.Parse(dateLayout, from)
		if err != nil {
			return nil, nil, errors.New("from must be YYYY-MM-DD")
		}
		to = time.Date(f.Year(), f.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format(dateLayout)
	case from == "":
		t, err := time.Parse(dateLayout, to)
		if err != nil {
			return nil, nil, errors.New("to must be YYYY-MM-DD")
		}
		from = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC).Format(dateLayout)
	}
	from, to, err = s.dateRange(from, to)
	if err != nil {
		return nil, nil, err
	}
	var (
		c   *api.Costs
		cmp *api.CostComparison
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) { c, err = s.api.Costs(gctx, from, to, gran); return })
	g.Go(func() (err error) { cmp, err = s.api.CompareCosts(gctx, from, to, gran); return })
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}
	type delivery struct {
		ID          int64          `json:"-"`
		Name        string         `json:"name"`
		Category    string         `json:"category"`
		Cost        float64        `json:"cost"`
		Complete    bool           `json:"data_complete"`
		Consumption *float64       `json:"consumption,omitempty"`
		Unit        string         `json:"unit,omitempty"`
		Compared    map[string]any `json:"compared_with"`
	}
	var dels []*delivery
	byID := map[int64]*delivery{}
	for _, d := range c.Deliveries {
		x := &delivery{ID: d.DeliveryID, Name: strings.TrimSpace(d.Label), Category: d.Category, Cost: d.TotalCost, Complete: d.Complete, Compared: map[string]any{}}
		if d.ConsumptionUnit != nil {
			v := d.Consumption
			x.Consumption, x.Unit = &v, *d.ConsumptionUnit
		}
		dels = append(dels, x)
		byID[d.DeliveryID] = x
	}
	for _, k := range cmp.Comparisons {
		x := byID[k.DeliveryID]
		if x == nil {
			x = &delivery{ID: k.DeliveryID, Name: "delivery " + strconv.FormatInt(k.DeliveryID, 10), Compared: map[string]any{}}
			dels = append(dels, x)
			byID[k.DeliveryID] = x
		}
		name := comparisonNames[k.Type]
		if name == "" {
			name = strings.ToLower(k.Type)
		}
		if !k.Available {
			e := map[string]any{"available": false}
			if n := strings.TrimSpace(deref(k.Note)); n != "" {
				e["note"] = n
			}
			x.Compared[name] = e
			continue
		}
		e := map[string]any{"from": datePart(k.From), "to": datePart(k.To), "cost": round2(k.TotalCost)}
		if k.ConsumptionUnit != nil {
			e["consumption"] = round2(k.TotalConsumption)
			e["unit"] = *k.ConsumptionUnit
		}
		x.Compared[name] = e
	}
	out := map[string]any{"currency": c.Currency, "from": from, "to": to, "deliveries": dels, "settled": c.PeriodType != "UNSETTLED"}
	return nil, out, nil
}

func round2(f float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(f, 'f', 2, 64), 64)
	return v
}
