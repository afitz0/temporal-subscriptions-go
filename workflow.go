package starter

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

var (
	EVENT_HISTORY_THRESHOLD      = 1000
	HISTORY_CHECK_PERIOD_SECONDS = 5
)

type Messages struct {
	UpdateInfo UpdateSignal
}

func (s *Messages) HandleSignals(ctx workflow.Context, canChannel workflow.Channel) {
	logger := workflow.GetLogger(ctx)
	selector := workflow.NewSelector(ctx)

	for {
		selector.AddReceive(workflow.GetSignalChannel(ctx, "update"), func(c workflow.ReceiveChannel, _ bool) {
			var updateInfo UpdateSignal
			c.Receive(ctx, &updateInfo)
			logger.Info("Received update signal", "data", updateInfo)
			// TODO: apply updates
		})

		selector.AddReceive(workflow.GetSignalChannel(ctx, "cancel"), func(c workflow.ReceiveChannel, _ bool) {
			var cancel UpdateSignal
			c.Receive(ctx, &cancel)
			logger.Info("Received cancel signal", "data", cancel)

			if cancel.CancelSubscription {
				// TODO: cancel subscription
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

	logger := workflow.GetLogger(ctx)
	logger.Info("Subscription workflow started", "customer", customer, "subscription", subscription)

	// Immediately kick off update/cancel handlers, since those can come in at
	// any time. Channel used to signal that Continue-As-New is needed
	var m Messages
	continueAsNew := workflow.NewChannel(ctx)
	workflow.Go(ctx, func(ctx workflow.Context) {
		m.HandleSignals(ctx, continueAsNew)
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
			logger.Info("History size too big. time to CAN")
			// TODO continue-as-new
		}
	})

	trialSelector.Select(ctx)

	workflow.Go(ctx, func(gCtx workflow.Context) {
		for {
			_ = workflow.Sleep(gCtx, time.Duration(HISTORY_CHECK_PERIOD_SECONDS*int(time.Second)))

			logger.Info("Current history length", "History Length", info.GetCurrentHistoryLength())
			if info.GetCurrentHistoryLength() > EVENT_HISTORY_THRESHOLD {
				continueAsNew.Send(gCtx, true)
			}
		}
	})

	for !m.UpdateInfo.CancelSubscription {
		// start of billing cycle, charge a subscription
		err = workflow.ExecuteActivity(ctx, a.ChargeSubscription, customer, subscription).Get(ctx, nil)
		if err != nil {
			logger.Error("ChargeSubscription Activity failed.", "Error", err)
			return err
		}

		billingTimer := workflow.NewTimer(ctx, time.Duration(subscription.BillingPeriodDays*24*float64(time.Hour)))

		workflow.NewSelector(ctx).
			AddFuture(billingTimer, func(f workflow.Future) {
				// start timer for billing period. Must be async as update/cancel signals may come in.
				logger.Info("Billing cycle is up! Time to charge...")
			}).
			AddReceive(continueAsNew, func(c workflow.ReceiveChannel, _ bool) {
				var canMessage bool
				c.Receive(ctx, &canMessage)
				if canMessage {
					logger.Info("History size too big. time to CAN")
					// TODO continue-as-new
				}
			}).
			Select(ctx)
	}

	logger.Info("Subscription workflow completed.")
	return nil
}
