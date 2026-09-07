package business

import (
	"fmt"
	"sync"
	"time"
)

// Store is the mock business API. In production this is the customer's real
// CRM/billing system behind an interface; the agent's read tools hit this,
// and post-call writes land here too. Swapping in a real API is a matter of
// implementing the same interface.
type Store interface {
	GetCustomer(id string) (*Customer, error)
	GetOpenInvoice(customerID string) (*Invoice, error)
	ListOverdueCustomers() ([]*Customer, error)
	MarkPromise(customerID, invoiceID string, promiseDate time.Time) error
	Escalate(customerID, reason string) error
	RecordContact(customerID, channel, outcome string) error
}

// Customer is the prefetched record injected into every call session.
type Customer struct {
	ID        string
	Name      string
	Phone     string
	Email     string
	Locale    string
	Region    string
	Plan      string
	JoinedAt  time.Time
}

// Invoice is the billing record for the invoice.due event.
type Invoice struct {
	ID           string
	CustomerID   string
	AmountCents  int64
	Currency     string
	DueDate      time.Time
	DaysOverdue  int
	LastPayment  *time.Time
	Status       string // open | paid | promised
}

type mockStore struct {
	mu        sync.RWMutex
	customers map[string]*Customer
	invoices  map[string]*Invoice
	escalated []Escalation
	contacts  []Contact
}

// Escalation records a needs_human outcome.
type Escalation struct {
	CustomerID string
	Reason     string
	At         time.Time
}

// Contact is an audit-log entry for every attempted contact channel.
type Contact struct {
	CustomerID string
	Channel    string // phone | email | sms
	Outcome    string
	At         time.Time
}

// NewMockStore seeds a demo dataset: the three original demo customers plus a
// 30-person campaign audience with open invoices across regions. Replace
// with a real Store in production.
func NewMockStore() Store {
	joined := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	paid := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)

	customers := map[string]*Customer{
		"cus_1001": {ID: "cus_1001", Name: "Priya Sharma", Phone: "+919900000001", Email: "priya@example.com", Locale: "en-IN", Region: "IN", Plan: "Pro", JoinedAt: joined},
		"cus_1002": {ID: "cus_1002", Name: "Daniel Okafor", Phone: "+14155550002", Email: "daniel@example.com", Locale: "en-US", Region: "US", Plan: "Starter", JoinedAt: joined},
		"cus_1003": {ID: "cus_1003", Name: "Mei Chen", Phone: "+6590000003", Email: "mei@example.com", Locale: "en-SG", Region: "SG", Plan: "Pro", JoinedAt: joined},
	}
	invoices := map[string]*Invoice{
		"inv_5001": {ID: "inv_5001", CustomerID: "cus_1001", AmountCents: 14900, Currency: "INR", DueDate: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC), DaysOverdue: 7, LastPayment: &paid, Status: "open"},
		"inv_5002": {ID: "inv_5002", CustomerID: "cus_1002", AmountCents: 4900, Currency: "USD", DueDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), DaysOverdue: 5, LastPayment: nil, Status: "open"},
	}

	// Campaign audience: 30 customers with overdue invoices, mixed regions.
	names := []string{"Aarav Patel", "Sofia Reyes", "Liam O'Brien", "Ananya Iyer", "Noah Kim", "Isabella Rossi", "Arjun Nair", "Emma Schmidt", "Kaito Tanaka", "Fatima Al-Sayed",
		"Rohan Mehta", "Olivia Brown", "Vikram Singh", "Sophie Dubois", "Ethan Walsh", "Divya Reddy", "Marcus Webb", "Yuki Nakamura", "Leila Haddad", "Carlos Mendes",
		"Neha Kulkarni", "Jack Thompson", "Meera Pillai", "Chloe Martin", "Aditya Rao", "Ryan Kelly", "Sana Kapoor", "Lucas Meyer", "Ishaan Verma", "Grace Liu"}
	regions := []struct{ region, locale, phonePrefix, currency string }{
		{"IN", "en-IN", "+9199", "INR"},
		{"US", "en-US", "+1415", "USD"},
		{"SG", "en-SG", "+6590", "SGD"},
	}
	for i, name := range names {
		id := fmt.Sprintf("cus_%d", 2001+i)
		r := regions[i%3]
		phone := fmt.Sprintf("%s%07d", r.phonePrefix, 1000000+i)
		customers[id] = &Customer{
			ID: id, Name: name, Phone: phone,
			Email: fmt.Sprintf("user%d@example.com", 2001+i),
			Locale: r.locale, Region: r.region,
			Plan: "Starter", JoinedAt: joined,
		}
		overdue := 3 + i%21
		amount := int64(2900 + 100*(i%17))
		invoices[fmt.Sprintf("inv_%d", 6001+i)] = &Invoice{
			ID: fmt.Sprintf("inv_%d", 6001+i), CustomerID: id,
			AmountCents: amount, Currency: r.currency,
			DueDate: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
			DaysOverdue: overdue, Status: "open",
		}
	}

	return &mockStore{customers: customers, invoices: invoices}
}

func (m *mockStore) GetCustomer(id string) (*Customer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.customers[id]
	if !ok {
		return nil, fmt.Errorf("customer %q not found", id)
	}
	cp := *c
	return &cp, nil
}

func (m *mockStore) GetOpenInvoice(customerID string) (*Invoice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, inv := range m.invoices {
		if inv.CustomerID == customerID && inv.Status != "paid" {
			cp := *inv
			if inv.LastPayment != nil {
				t := *inv.LastPayment
				cp.LastPayment = &t
			}
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("no open invoice for customer %q", customerID)
}

// ListOverdueCustomers returns every customer with an open invoice — the
// default campaign audience for invoice.due.
func (m *mockStore) ListOverdueCustomers() ([]*Customer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []*Customer
	seen := map[string]bool{}
	for _, inv := range m.invoices {
		if inv.Status == "open" && !seen[inv.CustomerID] {
			if c, ok := m.customers[inv.CustomerID]; ok {
				seen[inv.CustomerID] = true
				cp := *c
				out = append(out, &cp)
			}
		}
	}
	return out, nil
}

func (m *mockStore) MarkPromise(customerID, invoiceID string, promiseDate time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	inv, ok := m.invoices[invoiceID]
	if !ok || inv.CustomerID != customerID {
		return fmt.Errorf("invoice %q not found for customer %q", invoiceID, customerID)
	}
	inv.Status = "promised"
	return nil
}

func (m *mockStore) Escalate(customerID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.escalated = append(m.escalated, Escalation{CustomerID: customerID, Reason: reason, At: time.Now().UTC()})
	return nil
}

func (m *mockStore) RecordContact(customerID, channel, outcome string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contacts = append(m.contacts, Contact{CustomerID: customerID, Channel: channel, Outcome: outcome, At: time.Now().UTC()})
	return nil
}
