package starter

type SubscriptionInfo struct {
	TrialPeriodDays   float64
	Amount            float32
	BillingPeriodDays float64
}

type CustomerInfo struct {
	Name  string
	Email string
}

type UpdateSignal struct {
	Customer           CustomerInfo
	Subscription       SubscriptionInfo
	CancelSubscription bool
}
