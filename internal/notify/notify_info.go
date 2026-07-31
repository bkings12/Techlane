package notify

// RepairNotifyInfo is everything an SMS template may need about a job.
type RepairNotifyInfo struct {
	JobCode         string
	PickupCode      string
	Phone           string
	ShopName        string
	CustomerName    string
	DeviceLabel     string
	ProblemSummary  string
	Status          string
	BranchName      string
	BranchLocation  string
	PickupPlace     string // "ShopName & location" for SMS "see you at …"
	Currency        string
	LaborAmount     float64
	Balance         float64
	PromisedBy      string // human-readable local-ish time, or empty
	PricingLine     string // ready-made sentence for intake SMS
	CustomerWaiting bool
	WaitMinutes     int    // estimated wait when CustomerWaiting
	WaitLine        string // ready-made wait-bench sentence
}
