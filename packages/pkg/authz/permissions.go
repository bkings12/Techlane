package authz

// PermissionDef describes a catalog entry.
type PermissionDef struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// SystemPermissionCatalog is the built-in permission set apps can assign to roles.
func SystemPermissionCatalog() []PermissionDef {
	return []PermissionDef{
		{Code: "users.read", Description: "View staff", Category: "identity"},
		{Code: "users.write", Description: "Create and update staff", Category: "identity"},
		{Code: "roles.assign", Description: "Assign roles to users", Category: "identity"},
		{Code: "roles.write", Description: "Create and edit roles and permissions", Category: "identity"},
		{Code: "branches.read", Description: "View branches", Category: "identity"},
		{Code: "branches.write", Description: "Manage branches", Category: "identity"},
		{Code: "devices.manage", Description: "Register and revoke devices", Category: "identity"},

		{Code: "customers.read", Description: "View customers", Category: "customers"},
		{Code: "customers.write", Description: "Create and update customers", Category: "customers"},
		{Code: "devices.read", Description: "View customer devices", Category: "customers"},
		{Code: "devices.write", Description: "Register customer devices", Category: "customers"},

		{Code: "repairs.read", Description: "View repair jobs", Category: "repairs"},
		{Code: "repairs.create", Description: "Create repair jobs", Category: "repairs"},
		{Code: "repairs.assign", Description: "Assign technicians", Category: "repairs"},
		{Code: "repairs.status.update", Description: "Update repair status", Category: "repairs"},
		{Code: "repairs.price.override", Description: "Override repair prices", Category: "repairs"},
		{Code: "repairs.edit", Description: "Correct repair job details after creation", Category: "repairs"},
		{Code: "repairs.collect", Description: "Release a device to the customer", Category: "repairs"},
		{Code: "repairs.release_unverified", Description: "Release a device without the owner confirming a code", Category: "repairs"},
		{Code: "repairs.close", Description: "Cancel a job or write it off as unrepairable", Category: "repairs"},
		{Code: "repairs.authorize_work", Description: "Authorize bench work without a customer-approved estimate", Category: "repairs"},
		{Code: "repairs.passcode.read", Description: "Reveal an encrypted device passcode", Category: "repairs"},

		{Code: "inventory.read", Description: "View inventory", Category: "inventory"},
		{Code: "inventory.adjust", Description: "Adjust stock", Category: "inventory"},
		{Code: "inventory.transfer", Description: "Transfer stock", Category: "inventory"},
		{Code: "parts.request", Description: "Request parts", Category: "inventory"},
		{Code: "parts.approve", Description: "Approve part requests", Category: "inventory"},
		{Code: "parts.collect", Description: "Collect supplier parts", Category: "inventory"},
		{Code: "suppliers.read", Description: "View suppliers", Category: "inventory"},
		{Code: "suppliers.write", Description: "Manage suppliers", Category: "inventory"},
		{Code: "supplier_credit.reconcile", Description: "Reconcile supplier credit", Category: "inventory"},

		{Code: "sales.create", Description: "Create POS sales", Category: "sales"},
		{Code: "sales.read", Description: "View sales", Category: "sales"},
		{Code: "sales.void", Description: "Void sales", Category: "sales"},
		{Code: "orders.confirm_paid", Description: "Manually confirm online orders as paid", Category: "sales"},

		{Code: "payments.initiate", Description: "Initiate payments", Category: "payments"},
		{Code: "payments.read", Description: "View payments", Category: "payments"},
		{Code: "payments.manual_confirm", Description: "Reconcile M-Pesa/bank payments via provider query", Category: "payments"},
		{Code: "cash.receive", Description: "Receive cash", Category: "payments"},
		{Code: "store_credit.manage", Description: "Issue and apply store credit", Category: "payments"},
		{Code: "cash.handover.request", Description: "Request cash handover", Category: "payments"},
		{Code: "cash.handover.confirm", Description: "Confirm cash handover", Category: "payments"},
		{Code: "refunds.create", Description: "Create refunds", Category: "payments"},
		{Code: "refunds.approve", Description: "Approve refunds", Category: "payments"},

		{Code: "commissions.read", Description: "View commissions", Category: "commissions"},
		{Code: "commissions.write", Description: "Configure commissions", Category: "commissions"},
		{Code: "commissions.approve", Description: "Approve and pay commissions", Category: "commissions"},

		{Code: "audit.read", Description: "View audit log", Category: "risk"},
		{Code: "risk.read", Description: "View risk alerts", Category: "risk"},
		{Code: "risk.ack", Description: "Acknowledge risk alerts", Category: "risk"},
		{Code: "reports.read", Description: "View reports", Category: "reports"},

		{Code: "loyalty.read", Description: "View customer loyalty point balances", Category: "marketing"},
		{Code: "loyalty.manage", Description: "Configure loyalty point earn rates", Category: "marketing"},
		{Code: "webhooks.manage", Description: "Manage outbound marketing/integration webhooks", Category: "marketing"},
	}
}

// IsKnownPermission reports whether code exists in the system catalog.
func IsKnownPermission(code string) bool {
	if code == "*" {
		return true
	}
	for _, p := range SystemPermissionCatalog() {
		if p.Code == code {
			return true
		}
	}
	return false
}
