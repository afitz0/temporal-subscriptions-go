package starter

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

type Activities struct{}

func (a *Activities) SendWelcomeEmail(ctx context.Context, customer CustomerInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending welcome email to " + customer.Email)
	return nil
}

func (a *Activities) ChargeSubscription(ctx context.Context, customer CustomerInfo, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Charging " + customer.Email + " the " +
		fmt.Sprintf("%.2f", subscription.Amount) + " subscription fee.")
	return nil
}
