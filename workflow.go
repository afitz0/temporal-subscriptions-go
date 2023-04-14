package starter

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

var (
	EVENT_HISTORY_THRESHOLD = 20000
)

func HandleSignals(ctx workflow.Context, canChannel workflow.Channel) {
	logger := workflow.GetLogger(ctx)
	selector := workflow.NewSelector(ctx)
	info := workflow.GetInfo(ctx)

	for {
		selector.AddReceive(workflow.GetSignalChannel(ctx, "update"), func(c workflow.ReceiveChannel, _ bool) {
			var updateInfo UpdateSignal
			c.Receive(ctx, &updateInfo)
			logger.Info("Received update signal", "data", updateInfo)
			// TODO: apply updates

			if info.GetCurrentHistoryLength() > EVENT_HISTORY_THRESHOLD {
				canChannel.Send(ctx, true)
			}
		})

		selector.AddReceive(workflow.GetSignalChannel(ctx, "cancel"), func(c workflow.ReceiveChannel, _ bool) {
			var cancel UpdateSignal
			c.Receive(ctx, &cancel)
			logger.Info("Received cancel signal", "data", cancel)

			if cancel.CancelSubscription {
				// TODO: cancel subscription
			}

			if info.GetCurrentHistoryLength() > EVENT_HISTORY_THRESHOLD {
				canChannel.Send(ctx, true)
			}
		})

		selector.Select(ctx)
	}
}

func SubscriptionWorkflow(ctx workflow.Context, customer CustomerInfo, subscription SubscriptionInfo) error {
	info := workflow.GetInfo(ctx)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	trialSelector := workflow.NewSelector(ctx)
	canceled := false

	logger := workflow.GetLogger(ctx)
	logger.Info("Subscription workflow started", "customer", customer, "subscription", subscription)

	// Immediately kick off update/cancel handlers, since those can come in at
	// any time. Channel used to signal that Continue-As-New is needed
	continueAsNew := workflow.NewChannel(ctx)
	workflow.Go(ctx, func(ctx workflow.Context) {
		HandleSignals(ctx, continueAsNew)
	})

	// Start trial period timer, async
	trialTimer := workflow.NewTimer(ctx, time.Duration(subscription.TrialPeriodDays*24*float64(time.Hour)))
	trialSelector.AddFuture(trialTimer, func(f workflow.Future) {
		// TODO: trial has ended
		logger.Info("Trial has ended")
	})

	var a *Activities
	err := workflow.ExecuteActivity(ctx, a.SendWelcomeEmail, customer).Get(ctx, nil)
	if err != nil {
		logger.Error("Welcome Email Activity failed.", "Error", err)
		return err
	}

	// wait for update/cancel signals, Continue-As-New channel, or Trial timer.
	trialSelector.AddReceive(continueAsNew, func(c workflow.ReceiveChannel, _ bool) {
		var canMessage bool
		c.Receive(ctx, &canMessage)
		if canMessage {
			// TODO continue-as-new
		}
	})

	trialSelector.Select(ctx)

	if info.GetCurrentHistoryLength() > EVENT_HISTORY_THRESHOLD {
		continueAsNew.Send(ctx, true)
	}

	for !canceled {
		// start of billing cycle, charge a subscription
		err = workflow.ExecuteActivity(ctx, a.ChargeSubscription, customer, subscription).Get(ctx, nil)
		if err != nil {
			logger.Error("ChargeSubscription Activity failed.", "Error", err)
			return err
		}

		// start timer for billing period. Must be async as update/cancel signals may come in.
		err = workflow.Sleep(ctx, time.Duration(subscription.BillingPeriodDays*24*float64(time.Hour)))
		// TODO: billing cycle is up! time to charge
		logger.Info("Billing cycle is up!")

		logger.Info("Current history length", "History Length", info.GetCurrentHistoryLength())
		if info.GetCurrentHistoryLength() > EVENT_HISTORY_THRESHOLD {
			continueAsNew.Send(ctx, true)
		}
	}

	logger.Info("Subscription workflow completed.")
	return nil
}
