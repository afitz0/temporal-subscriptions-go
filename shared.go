package subscription

import "time"

// SubscriptionInfo contains the necessary values for mananging a full subscription
// lifecycle, including if the Workflow is Continue-As-New'd.
type SubscriptionInfo struct {
	// AmountCents to charge the customer at the end of the trial period and then after every
	// `BillingPeriodDuration` time.
	AmountCents int

	// The customer's information to send emails and billing info to.
	Customer CustomerInfo

	// When should the trial expire. Must be set by the starter program to ensure that
	// the customer's trial starts as close as possible to when they themselves initiated it
	// (rather than when the Workflow started).
	TrialPeriodExpiration time.Time

	// How long should billing periods last after the trial has finished?
	BillingPeriodDuration time.Duration

	// Used for Continuing-As-New to account for any delays with terminating and re-starting the
	// workflow. This will minimize the amount of variation in actual billing period durations.
	BillingPeriodExpiration time.Time
}

// CustomerInfo contains all of the relevant information for billing the customer and sending
// appropriate emails.
type CustomerInfo struct {
	// The customer's name.
	Name string

	// The customer's email address.
	Email string

	// Unique Identifier for the customer.
	UID string
}

// UpdateSignal is a wrapper used for Signals into the workflow, either signalling to cancel
// the subscription, or to update the terms of the subscription.
type UpdateSignal struct {
	// The [optionally] new subscription information.
	Subscription SubscriptionInfo

	// The [optional] flag to cancel the whole Workflow.
	CancelSubscription bool
}
