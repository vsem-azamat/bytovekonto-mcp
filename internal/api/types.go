package api

import "encoding/json"

// The types below hold the fields of the API answers the tools use; the API sends more.

// HomeAccount is one tenancy or ownership the user can act in (GET /app/home-account).
type HomeAccount struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	SubType        string `json:"subType"`
	ProjectName    string `json:"projectName"`
	ActiveFromDate string `json:"activeFromDate"`
	ActiveToDate   string `json:"activeToDate"`
	Active         bool   `json:"active"`
}

// HomeAccountDetail is the current home account (GET /app/home-account/detail).
type HomeAccountDetail struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	SubType      string          `json:"subType"`
	Active       bool            `json:"active"`
	ActiveFrom   string          `json:"activeFrom"`
	ActiveTo     string          `json:"activeTo"`
	Subjects     []Contact       `json:"subjects"`
	Objects      []AccountObject `json:"objects"`
	Prescription *Prescription   `json:"prescriptionPaymentData"`
}

// AccountObject is a unit of the home account (a flat, a cellar).
type AccountObject struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Address string `json:"address"`
}

// Prescription is the regular monthly payment of the home account.
type Prescription struct {
	Status            string  `json:"status"`
	IssueDay          int     `json:"issueDay"`
	DueDays           int     `json:"dueDays"`
	VS                string  `json:"vs"`
	SS                string  `json:"ss"`
	SupplierName      string  `json:"supplierName"`
	BankAccountNumber string  `json:"bankAccountNumber"`
	BankCode          string  `json:"bankCode"`
	TotalAmount       float64 `json:"totalAmount"`
	Currency          string  `json:"currency"`
}

// Contact is a person or company related to the home account or the building.
type Contact struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Type             string     `json:"type"`
	RelationshipName string     `json:"relationshipName"`
	RelationshipType string     `json:"relationshipType"`
	SubType          string     `json:"subType"`
	Identifier       *string    `json:"identifier"`
	ValidFrom        *string    `json:"validFrom"`
	ValidTo          *string    `json:"validTo"`
	Responsible      bool       `json:"responsible"`
	Note             *string    `json:"note"`
	Email            *FlexValue `json:"email"`
	Phone            *FlexValue `json:"phone"`
}

// FlexValue is a contact value the API sends either as a string or as {"id":…, "value":…}.
type FlexValue struct{ Value string }

// UnmarshalJSON accepts a string, an object with a value, or null.
func (v *FlexValue) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		v.Value = s
		return nil
	}
	var o struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	v.Value = o.Value
	return nil
}

// MarshalJSON writes the plain value.
func (v FlexValue) MarshalJSON() ([]byte, error) { return json.Marshal(v.Value) }

// ProjectParty is a company related to the building: its owner or its manager
// (GET /app/project/info).
type ProjectParty struct {
	ID              int64            `json:"id"`
	RelatedToUnitAs string           `json:"relatedToUnitAs"`
	Type            string           `json:"type"`
	Name            string           `json:"name"`
	Address         string           `json:"address"`
	Identifier      string           `json:"identifier"`
	VATID           string           `json:"dic"`
	BankConnections []BankConnection `json:"bankConnections"`
	Contacts        []Contact        `json:"contacts"`
}

// BankConnection is a bank account of a company.
type BankConnection struct {
	PaymentName       string  `json:"paymentName"`
	AccountIdentifier string  `json:"accountIdentifier"`
	BankCode          string  `json:"bankCode"`
	IBAN              *string `json:"iban"`
	Currency          string  `json:"currency"`
	Preferred         bool    `json:"preferred"`
}

// Infobox is an announcement banner of the building (GET /app/project/infoboxes).
type Infobox struct {
	ID          int64   `json:"id"`
	ProjectID   int64   `json:"projectId"`
	ActiveFrom  *string `json:"activeFrom"`
	ActiveTo    *string `json:"activeTo"`
	Priority    int     `json:"priority"`
	HTMLContent string  `json:"htmlContent"`
	LinkURL     *string `json:"linkUrl"`
}

// FAQ is one question and answer of the building's FAQ (GET /app/project/faq).
type FAQ struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// Ticket states.
const (
	StateNew        = "NEW"
	StateInProgress = "IN_PROGRESS"
	StateOnHold     = "ON_HOLD"
	StateSolved     = "SOLVED"
	StateCanceled   = "CANCELED"
	StateDuplicate  = "DUPLICATE"
)

// Ticket tags the API sets.
const (
	TagUserIsCreator      = "USER_IS_CREATOR"
	TagInvolvesCommonArea = "INVOLVES_COMMON_AREA"
)

// Related entity types of a ticket.
const (
	EntityProject = "PROJECT"
	EntityObject  = "OBJECT"
)

// Ticket is a request, fault report, question or discussion with the building's management
// (an item of GET /app/tickets/list-with-last-comment, or GET /app/tickets/{id}).
type Ticket struct {
	ID                   int64             `json:"id"`
	CreatedAt            string            `json:"createdAt"`
	ModifiedAt           string            `json:"modifiedAt"`
	ClosedAt             *string           `json:"closedAt"`
	Subject              string            `json:"subject"`
	Description          string            `json:"description"`
	State                string            `json:"ticketState"`
	PossibleStates       []string          `json:"possibleTicketStates"`
	Priority             string            `json:"ticketPriority"`
	DueDate              *string           `json:"dueDate"`
	CategoryID           int64             `json:"ticketCategoryId"`
	Category             string            `json:"ticketCategory"`
	AreaID               int64             `json:"ticketAreaId"`
	Area                 string            `json:"ticketArea"`
	ProjectName          string            `json:"projectName"`
	Tags                 []string          `json:"tags"`
	Members              []TicketMember    `json:"ticketMembersContactInfo"`
	Relations            []RelatedEntity   `json:"relations"`
	Private              bool              `json:"privateTicket"`
	NoComments           bool              `json:"noComments"`
	MemberCount          int               `json:"ticketMemberCount"`
	LastCommentText      *string           `json:"lastCommentText"`
	LastCommentAuthor    *string           `json:"lastCommentAuthor"`
	LastCommentCreatedAt *string           `json:"lastCommentCreatedAt"`
	Attachments          []json.RawMessage `json:"attachments"`
}

// HasTag reports whether the ticket carries the tag.
func (t *Ticket) HasTag(tag string) bool {
	for _, x := range t.Tags {
		if x == tag {
			return true
		}
	}
	return false
}

// Open reports whether the ticket is still being dealt with.
func (t *Ticket) Open() bool {
	switch t.State {
	case StateSolved, StateCanceled, StateDuplicate:
		return false
	}
	return true
}

// TicketMember is a participant of a ticket.
type TicketMember struct {
	Anonymized bool    `json:"anonymized"`
	RoleType   string  `json:"roleType"`
	SubjectID  int64   `json:"subjectId"`
	Label      string  `json:"label"`
	Phone      *string `json:"phone"`
	Email      *string `json:"email"`
}

// RelatedEntity links a ticket to the building (PROJECT) or a unit (OBJECT).
type RelatedEntity struct {
	EntityType string  `json:"entityType"`
	EntityID   string  `json:"entityId"`
	Label      *string `json:"label"`
}

// TicketLifecycle is the conversation and the change history of a ticket
// (GET /app/tickets/{id}/lifecycle).
type TicketLifecycle struct {
	Comments []Comment      `json:"comments"`
	History  []HistoryEvent `json:"history"`
}

// Comment is a message in a ticket.
type Comment struct {
	ID          int64   `json:"id"`
	CreatedAt   string  `json:"createdAt"`
	CreatorID   int64   `json:"creatorSubjectId"`
	CreatorName string  `json:"creatorSubjectName"`
	Text        string  `json:"text"`
	ModifiedAt  *string `json:"modifiedAt"`
	DeletedAt   *string `json:"deletedAt"`
}

// HistoryEvent is a change of a ticket: its state, category, members and so on.
type HistoryEvent struct {
	Metadata struct {
		EventType   string `json:"eventType"`
		SubjectName string `json:"subjectName"`
		Created     string `json:"created"`
	} `json:"metadata"`
	// Changes are passed on as the API sends them.
	Changes []json.RawMessage `json:"changes"`
}

// TicketOptions are the categories, areas and units a new ticket can use
// (GET /app/tickets/options).
type TicketOptions struct {
	Categories []TicketOption `json:"categories"`
	Areas      []TicketOption `json:"areas"`
	Objects    []TicketObject `json:"objects"`
}

// TicketOption is a ticket category or area.
type TicketOption struct {
	ID   int64  `json:"id"`
	Code string `json:"code"`
	Type string `json:"type"`
	Name string `json:"name"`
}

// TicketObject is a building, entrance or unit a ticket can be about; Own marks the user's units.
type TicketObject struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Own  bool   `json:"own"`
}

// NewTicket is the body of POST /app/tickets, as the apps send it: the unit is the only related
// entity, the API adds the building.
type NewTicket struct {
	Subject         string          `json:"subject"`
	Description     string          `json:"description"`
	Priority        *string         `json:"ticketPriority"`
	DueDate         *string         `json:"dueDate"`
	CategoryID      int64           `json:"categoryId"`
	AreaID          int64           `json:"areaId"`
	RelatedEntities []RelatedEntity `json:"relatedEntities"`
	Attachments     []string        `json:"attachments"`
	Actions         []string        `json:"actions"`
	Private         bool            `json:"privateTicket"`
}

// NewComment is the body of POST /app/tickets/{id}/comment.
type NewComment struct {
	Message                     string   `json:"message"`
	Attachments                 []string `json:"attachments"`
	SetIgnoreMemberCountWarning *bool    `json:"setIgnoreMemberCountWarning"`
}

// Payment filters of GET /app/payments.
const (
	PaymentsToPay    = "TO_PAY"
	PaymentsOverpaid = "OVERPAID"
)

// Payment is an accounting document: a prescription, an invoice or a settlement
// (an item of GET /app/payments).
type Payment struct {
	ID               int64   `json:"id"`
	TotalAmount      float64 `json:"totalAmount"`
	TotalAmountPaid  float64 `json:"totalAmountPaid"`
	RemainingUnpaid  float64 `json:"remainingUnpaidAmount"`
	Status           string  `json:"paymentStatus"`
	Currency         string  `json:"documentCurrency"`
	DocumentName     string  `json:"documentDefinitionName"`
	DocumentType     string  `json:"documentDefinitionType"`
	Number           string  `json:"sequentialNumber"`
	CreatedDate      string  `json:"formalCreatedDate"`
	PaymentDate      *string `json:"formalPaymentDate"`
	DueDate          *string `json:"formalPaymentDueDate"`
	PeriodType       string  `json:"paymentPeriodType"`
	PrescriptionName string  `json:"prescriptionName"`
	SettlementName   *string `json:"settlementName"`
	SupplierName     string  `json:"supplierName"`
	PaymentAvailable bool    `json:"paymentAvailable"`
}

// PaymentDetail is one accounting document with its items (GET /app/payments/{id}).
type PaymentDetail struct {
	ID               int64         `json:"id"`
	Number           string        `json:"sequentialNumber"`
	DocumentName     string        `json:"documentDefinitionName"`
	DocumentType     string        `json:"documentDefinitionType"`
	Status           string        `json:"paymentStatus"`
	DocumentStatus   string        `json:"documentStatus"`
	PaymentType      string        `json:"paymentType"`
	TotalAmount      float64       `json:"totalAmount"`
	TotalAmountPaid  float64       `json:"totalAmountPaid"`
	RemainingUnpaid  float64       `json:"remainingUnpaidAmount"`
	Currency         string        `json:"documentCurrency"`
	VS               *string       `json:"bankPaymentVS"`
	SS               *string       `json:"bankPaymentSS"`
	KS               *string       `json:"bankPaymentKS"`
	CreatedDate      string        `json:"formalCreatedDate"`
	DueDate          *string       `json:"formalPaymentDueDate"`
	PaymentDate      *string       `json:"formalPaymentDate"`
	PeriodFrom       *string       `json:"globalPeriodFrom"`
	PeriodTo         *string       `json:"globalPeriodTo"`
	NotePublic       *string       `json:"notePublic"`
	PrescriptionName *string       `json:"prescriptionName"`
	SettlementName   *string       `json:"settlementName"`
	Items            []PaymentItem `json:"items"`
}

// PaymentItem is one line of an accounting document.
type PaymentItem struct {
	Description  *string `json:"description"`
	DeliveryName *string `json:"deliveryName"`
	Amount       float64 `json:"amount"`
	SumWithVAT   float64 `json:"sumWithVat"`
	ItemCount    float64 `json:"itemCount"`
	ItemUnit     *string `json:"itemUnit"`
	VATRate      float64 `json:"vatRate"`
	PeriodFrom   *string `json:"periodValidFrom"`
	PeriodTo     *string `json:"periodValidTo"`
}

// PaymentsOverview sums the account's payments by month (GET /app/payments/overview).
type PaymentsOverview struct {
	Overview struct {
		PastDue               float64 `json:"pastDue"`
		OldestPastDueDate     *string `json:"oldestPastDueDate"`
		Due                   float64 `json:"due"`
		NearestDueDate        *string `json:"nearestDueDate"`
		Paid                  float64 `json:"paid"`
		Overpaid              float64 `json:"overpaid"`
		Planned               float64 `json:"planned"`
		UnpaidPreviousPeriods float64 `json:"unpaidFromPreviousPeriods"`
	} `json:"overview"`
	Months []struct {
		Month   string  `json:"month"`
		Total   float64 `json:"total"`
		Paid    float64 `json:"paid"`
		PastDue float64 `json:"pastDue"`
		Due     float64 `json:"due"`
		Actual  int     `json:"actualDocumentCount"`
		Planned int     `json:"plannedDocumentCount"`
	} `json:"monthsData"`
}

// Costs granularities of GET /app/costs.
const (
	GranularityDay   = "DAY"
	GranularityWeek  = "WEEK"
	GranularityMonth = "MONTH"
)

// Costs are the account's costs per delivery (rent, heat, water, electricity) and period
// (GET /app/costs).
type Costs struct {
	Currency        string `json:"currency"`
	PeriodType      string `json:"periodType"`
	DataAvailableTo string `json:"dataAvailableTo"`
	Periods         []struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Label string `json:"label"`
	} `json:"periods"`
	Deliveries []struct {
		Label              string    `json:"label"`
		Category           string    `json:"category"`
		CategoryDefinition string    `json:"categoryDefinition"`
		Complete           bool      `json:"complete"`
		TotalCost          float64   `json:"totalCost"`
		TotalCostType      string    `json:"totalCostType"`
		PrescriptionCost   float64   `json:"prescriptionCost"`
		Consumption        float64   `json:"consumption"`
		ConsumptionUnit    *string   `json:"consumptionUnit"`
		PeriodCost         []float64 `json:"periodCost"`
		PeriodConsumption  []float64 `json:"periodConsumption"`
	} `json:"deliveries"`
}

// Rent is the current rent (GET /app/costs/rent).
type Rent struct {
	Currency string `json:"currency"`
	Current  *struct {
		From  string  `json:"from"`
		To    *string `json:"to"`
		Value float64 `json:"value"`
	} `json:"current"`
}

// Meter is a utility meter of the account's units (an item of GET /app/metering/, or
// GET /app/metering/{id} with its last values).
type Meter struct {
	ID                int64                `json:"id"`
	Type              string               `json:"type"`
	TypeCode          string               `json:"typeCode"`
	Category          string               `json:"category"`
	ModelName         string               `json:"modelName"`
	Name              string               `json:"name"`
	ObjectNames       []string             `json:"objectNames"`
	ActiveFrom        *string              `json:"activeFrom"`
	ActiveTo          *string              `json:"activeTo"`
	Active            bool                 `json:"active"`
	LastValues        map[string]LastValue `json:"lastValues"`
	LastValueReceived *string              `json:"lastValueReceived"`
	ReceptionStatus   *string              `json:"receptionStatus"`
}

// LastValue is the latest reading of a meter channel.
type LastValue struct {
	Unit          string   `json:"unit"`
	Channel       string   `json:"channel"`
	LastValue     *float64 `json:"lastValue"`
	LastValueTime *string  `json:"lastValueTime"`
}

// Chart granularities of GET /app/metering/{id}/chartData.
const (
	ChartHourly  = "HOURLY"
	ChartDaily   = "DAILY"
	ChartMonthly = "MONTHLY"
)

// MeterChart is a meter's consumption per period (GET /app/metering/{id}/chartData).
type MeterChart struct {
	Labels        []string                `json:"labels"`
	StartTime     string                  `json:"startTimeWithOffset"`
	EndTime       string                  `json:"endTimeWithOffset"`
	DataByChannel map[string]ChannelChart `json:"dataByChannel"`
}

// ChannelChart is the consumption of one meter channel.
type ChannelChart struct {
	TotalConsumption float64   `json:"totalConsumption"`
	Consumptions     []float64 `json:"consumptions"`
	StartValue       *float64  `json:"startValue"`
	EndValue         *float64  `json:"endValue"`
	Unit             string    `json:"unit"`
}

// Document is a file the management shares with the building's residents
// (an item of GET /app/documents/myProject).
type Document struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	FileID   string `json:"fileId"`
	FileMeta struct {
		Filename    string `json:"filename"`
		Size        int64  `json:"size"`
		ContentType string `json:"contentType"`
	} `json:"fileMeta"`
	Category *struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"category"`
	CreatedAt  string `json:"createdAt"`
	ModifiedAt string `json:"modifiedAt"`
}

// Notifications are the user's in-app notifications (GET /app/notifications/list).
type Notifications struct {
	Notifications []json.RawMessage `json:"notifications"`
}
