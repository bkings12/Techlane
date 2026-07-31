package repair

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/techlane/techlane/internal/notify"
)

// NotifyAdapter implements notify.RepairLookup for event-driven notifications.
type NotifyAdapter struct {
	Svc *Service
}

func (a NotifyAdapter) RepairNotifyInfo(ctx context.Context, tenantID, repairID uuid.UUID) (notify.RepairNotifyInfo, error) {
	var info notify.RepairNotifyInfo
	job, err := a.Svc.GetRepair(ctx, tenantID, repairID)
	if err != nil {
		return info, err
	}
	info.JobCode = job.JobCode
	if info.JobCode == "" {
		info.JobCode = job.ID.String()[:8]
	}
	info.PickupCode = strings.TrimSpace(job.PickupCode)
	info.Status = job.Status
	info.ProblemSummary = strings.TrimSpace(job.ProblemSummary)
	if info.ProblemSummary == "" {
		info.ProblemSummary = "repair"
	}
	info.LaborAmount = job.LaborAmount
	info.Currency = "KES"
	if job.Customer != nil {
		info.CustomerName = strings.TrimSpace(job.Customer.FullName)
		if job.Customer.Phone != nil {
			info.Phone = strings.TrimSpace(*job.Customer.Phone)
		}
	}
	if info.CustomerName == "" {
		info.CustomerName = "there"
	}
	info.DeviceLabel = deviceLabel(job.Device)
	_ = a.Svc.pool.QueryRow(ctx, `SELECT name FROM identity.tenants WHERE id = $1`, tenantID).Scan(&info.ShopName)
	if info.ShopName == "" {
		info.ShopName = "TechLane"
	}
	_ = a.Svc.pool.QueryRow(ctx, `
		SELECT name, COALESCE(location, '')
		FROM identity.branches WHERE tenant_id = $1 AND id = $2`,
		tenantID, job.BranchID).Scan(&info.BranchName, &info.BranchLocation)
	if info.BranchName == "" {
		info.BranchName = "the shop"
	}
	info.PickupPlace = formatPickupPlace(info.ShopName, info.BranchLocation, info.BranchName)
	if tax := a.Svc.loadShopTaxProfile(ctx, tenantID); tax.CurrencyCode != "" {
		info.Currency = tax.CurrencyCode
	}
	if due, err := a.Svc.outstandingRepairBalance(ctx, tenantID, repairID); err == nil {
		info.Balance = due
	}
	if job.PromisedBy != nil {
		info.PromisedBy = job.PromisedBy.In(time.Local).Format("2 Jan 3:04 PM")
	}
	info.CustomerWaiting = job.CustomerWaiting
	if job.EstimatedWaitMin != nil && *job.EstimatedWaitMin > 0 {
		info.WaitMinutes = *job.EstimatedWaitMin
		info.WaitLine = fmt.Sprintf("Please wait at the wait bench — about %d minutes.", *job.EstimatedWaitMin)
	} else if job.CustomerWaiting {
		info.WaitLine = "Please wait at the wait bench."
	}
	if job.LaborAmount > 0 {
		info.PricingLine = fmt.Sprintf("Quoted %s %.0f.", info.Currency, job.LaborAmount)
	} else {
		info.PricingLine = "We'll send an estimate after diagnosis."
	}
	if info.PromisedBy != "" && !job.CustomerWaiting {
		info.PricingLine += " Promised by " + info.PromisedBy + "."
	}
	return info, nil
}

// RepairNotifyContext keeps a slim triple for older call sites.
func (a NotifyAdapter) RepairNotifyContext(ctx context.Context, tenantID, repairID uuid.UUID) (jobCode, phone, shopName string, err error) {
	info, err := a.RepairNotifyInfo(ctx, tenantID, repairID)
	if err != nil {
		return "", "", "", err
	}
	return info.JobCode, info.Phone, info.ShopName, nil
}

func (a NotifyAdapter) PaymentNotifyContext(ctx context.Context, tenantID, paymentID uuid.UUID) (repairID *uuid.UUID, jobCode, phone, amount, currency string, err error) {
	var payableType string
	var payableID uuid.UUID
	var amt float64
	err = a.Svc.pool.QueryRow(ctx, `
		SELECT a.payable_type, a.payable_id, p.amount::float8, p.currency
		FROM payments.payment_allocations a
		JOIN payments.payments p ON p.id = a.payment_id
		WHERE p.tenant_id = $1 AND p.id = $2 LIMIT 1`, tenantID, paymentID).
		Scan(&payableType, &payableID, &amt, &currency)
	if err != nil {
		return nil, "", "", "", "", err
	}
	amount = fmt.Sprintf("%.0f", amt)
	if payableType != "repair" {
		return nil, "", "", amount, currency, nil
	}
	repairID = &payableID
	info, lErr := a.RepairNotifyInfo(ctx, tenantID, payableID)
	if lErr != nil {
		return repairID, "", "", amount, currency, lErr
	}
	return repairID, info.JobCode, info.Phone, amount, currency, nil
}

func deviceLabel(d *Device) string {
	if d == nil {
		return "device"
	}
	brand := ""
	model := ""
	if d.Brand != nil {
		brand = strings.TrimSpace(*d.Brand)
	}
	if d.Model != nil {
		model = strings.TrimSpace(*d.Model)
	}
	// Drop placeholder junk some intakes leave in brand (e.g. ".").
	if brand == "." || brand == "-" || brand == "·" {
		brand = ""
	}
	switch {
	case brand != "" && model != "":
		// Prefer a plain name for SMS — "phone · …" often becomes "phone . …" on handsets.
		return brand + " " + model
	case model != "":
		return model
	case brand != "":
		return brand
	case strings.TrimSpace(d.Kind) != "" && d.Kind != "other":
		return strings.TrimSpace(d.Kind)
	default:
		return "device"
	}
}

// formatPickupPlace builds "ShopName & location" for customer SMS.
// Falls back to branch name when location is empty so messages stay useful.
func formatPickupPlace(shopName, location, branchName string) string {
	shopName = strings.TrimSpace(shopName)
	location = strings.TrimSpace(location)
	branchName = strings.TrimSpace(branchName)
	if shopName == "" {
		shopName = "the shop"
	}
	place := location
	if place == "" && branchName != "" && branchName != "the shop" {
		place = branchName
	}
	if place == "" {
		return shopName
	}
	return shopName + " & " + place
}
