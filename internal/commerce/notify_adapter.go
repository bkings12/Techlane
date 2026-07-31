package commerce

import (
	"context"
	"fmt"
	"strings"

	"github.com/techlane/techlane/internal/notify"
)

// NotifyAdapter wires storefront order alerts into the notify outbox + staff inbox.
type NotifyAdapter struct {
	Svc *notify.Service
}

func (a NotifyAdapter) NotifyOnlineOrderPlaced(ctx context.Context, in OnlineOrderNotify) error {
	if a.Svc == nil {
		return nil
	}
	fulfilment := "pickup"
	if in.FulfilmentType == "delivery" {
		fulfilment = "delivery"
	}
	shortID := strings.ToUpper(in.OrderID.String())
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	payload := map[string]any{
		"shop_name":         in.ShopName,
		"customer_name":     in.CustomerName,
		"customer_phone":    in.CustomerPhone,
		"order_id":          in.OrderID.String(),
		"order_ref":         "ORD-" + shortID,
		"total":             fmt.Sprintf("%.0f", in.Total),
		"currency":          in.Currency,
		"fulfilment":        fulfilment,
		"item_count":        fmt.Sprintf("%d", in.ItemCount),
		"collection_code":   in.CollectionCode,
		"delivery_address":  in.DeliverySummary,
		"delivery_line":     "",
	}
	if in.DeliverySummary != "" {
		payload["delivery_line"] = "Deliver to: " + in.DeliverySummary + "."
	}

	branchID := in.BranchID
	title := "New online order"
	body := fmt.Sprintf("%s · %s %s · %s · %s",
		payload["order_ref"], in.Currency, payload["total"], fulfilment, in.CustomerName)
	if in.CustomerPhone != "" {
		body += " · " + in.CustomerPhone
	}
	_ = a.Svc.PostStaffInbox(ctx, in.TenantID, &branchID, title, body, "order.placed", payload)

	phone := strings.TrimSpace(in.OwnerPhone)
	if phone == "" {
		return nil
	}
	_, err := a.Svc.Enqueue(ctx, notify.EnqueueInput{
		TenantID:    in.TenantID,
		Channel:     notify.ChannelSMS,
		Recipient:   phone,
		TemplateKey: "order.placed",
		Payload:     payload,
	})
	return err
}
