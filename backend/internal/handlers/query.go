package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"erpagent-backend/internal/domain"
)

func queryInt(raw string, fallback, max int) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return fallback
	}
	if max > 0 && n > max {
		return max
	}
	if n == 0 && fallback > 0 {
		return fallback
	}
	return n
}

func queryBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

// normalizeName is the single comparison rule for entity resolution and row
// verification: casefold, strip punctuation, collapse whitespace.
func normalizeName(s string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(strings.Join(strings.Fields(b.String()), " "))
}

// resolveCustomerID maps a name to exactly one ID. Zero or multiple matches are
// returned to the caller so it can ask the user instead of guessing.
func resolveCustomerID(ctx context.Context, connector domain.ERPConnector, name string) (string, []domain.Customer, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil, nil
	}
	customers, err := connector.GetCustomers(ctx, domain.CustomerFilter{Search: name, Limit: 20})
	if err != nil {
		return "", nil, err
	}
	target := normalizeName(name)
	var exact []domain.Customer
	for _, c := range customers {
		if normalizeName(c.Name) == target {
			exact = append(exact, c)
		}
	}
	switch len(exact) {
	case 1:
		return exact[0].ID, nil, nil
	case 0:
		if len(customers) == 1 {
			return customers[0].ID, nil, nil
		}
		return "", customers, nil
	default:
		return "", exact, nil
	}
}

func verifySaleOrders(rows []domain.SaleOrder, filter domain.SaleOrderFilter) error {
	for _, row := range rows {
		if filter.CustomerID != "" && row.CustomerID != filter.CustomerID {
			return fmt.Errorf("read verification failed: order %s belongs to customer %s, expected %s", row.OrderNo, row.CustomerID, filter.CustomerID)
		}
		if filter.State != "" && !strings.EqualFold(row.State, filter.State) {
			return fmt.Errorf("read verification failed: order %s state=%s, expected %s", row.OrderNo, row.State, filter.State)
		}
		if filter.Search != "" {
			needle := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(row.OrderNo), needle) &&
				!strings.Contains(strings.ToLower(row.CustomerName), needle) {
				return fmt.Errorf("read verification failed: order %s does not match search %q", row.OrderNo, filter.Search)
			}
		}
	}
	return nil
}

func verifyInvoices(rows []domain.Invoice, filter domain.InvoiceFilter, now time.Time) error {
	for _, row := range rows {
		if filter.CustomerID != "" && row.CustomerID != filter.CustomerID {
			return fmt.Errorf("read verification failed: invoice %s belongs to customer %s, expected %s", row.InvoiceNo, row.CustomerID, filter.CustomerID)
		}
		if filter.Status != "" && !strings.EqualFold(row.Status, filter.Status) {
			return fmt.Errorf("read verification failed: invoice %s state=%s, expected %s", row.InvoiceNo, row.Status, filter.Status)
		}
		if filter.PaymentState != "" && !strings.EqualFold(row.PaymentState, filter.PaymentState) {
			return fmt.Errorf("read verification failed: invoice %s payment_state=%s, expected %s", row.InvoiceNo, row.PaymentState, filter.PaymentState)
		}
		if filter.Overdue {
			if row.AmountDue <= 0 {
				return fmt.Errorf("read verification failed: invoice %s is not outstanding (amount_due=%.2f)", row.InvoiceNo, row.AmountDue)
			}
			if !row.DueDate.Before(now) {
				return fmt.Errorf("read verification failed: invoice %s is not overdue (due %s)", row.InvoiceNo, row.DueDate.Format("2006-01-02"))
			}
		}
	}
	return nil
}
