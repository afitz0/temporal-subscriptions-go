package subscription

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

type Activities struct{}

func (a *Activities) SendWelcomeEmail(ctx context.Context, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending welcome email to " + subscription.Customer.Email)
	return nil
}

func (a *Activities) SendTrialExpiredEmail(ctx context.Context, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending trial is over email to " + subscription.Customer.Email)
	return nil
}

func (a *Activities) SendCancelationEmail(ctx context.Context, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending 'sorry to see you go' email to " + subscription.Customer.Email)
	return nil
}

func (a *Activities) ChargeSubscription(ctx context.Context, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Charging " + subscription.Customer.Email + " the " +
		fmt.Sprintf("%.2f", subscription.Amount) + " subscription fee.")
	return nil
}
