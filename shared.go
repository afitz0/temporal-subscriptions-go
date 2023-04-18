package subscription

import "time"

type SubscriptionInfo struct {
	Amount   float32
	Customer CustomerInfo

	TrialPeriodDays        float64
	TrialPeriodRemaining   time.Duration
	BillingPeriodDays      float64
	BillingPeriodRemaining time.Duration
}

type CustomerInfo struct {
	Name  string
	Email string
	UID   string
}

type UpdateSignal struct {
	Customer           CustomerInfo
	Subscription       SubscriptionInfo
	CancelSubscription bool
}
