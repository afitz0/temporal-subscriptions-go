package subscription

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"
)

type Activities struct{}

func (a *Activities) SendWelcomeEmail(ctx context.Context, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending welcome email.", "CustomerEmail", subscription.Customer.Email)
	return nil
}

func (a *Activities) SendTrialExpiredEmail(ctx context.Context, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending trial is over email.", "CustomerEmail", subscription.Customer.Email)
	return nil
}

func (a *Activities) SendCancelationEmail(ctx context.Context, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info("Sending 'sorry to see you go' email.", "CustomerEmail", subscription.Customer.Email)
	return nil
}

func (a *Activities) ChargeSubscription(ctx context.Context, subscription SubscriptionInfo) error {
	logger := activity.GetLogger(ctx)
	logger.Info(fmt.Sprintf("Charging the subscription fee.", "CustomerEmail", subscription.Customer.Email, "Amount", subscription.AmountCents))
	return nil
}
