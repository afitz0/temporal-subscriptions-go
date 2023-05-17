package subscription

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

var (
	// These thresholds/periods are purposefully short to allow for easier
	// testing and demonstration of Continue-As-New. The Temporal Events in
	// this application are small enough that it's unlikely to hit the event
	// *size* limit, and so in the "real" version of this,
	// EVENT_HISTORY_THRESHOLD should be much closer to 50k.
	EVENT_HISTORY_THRESHOLD      = 1000
	HISTORY_CHECK_PERIOD_SECONDS = 5

	// TODO making this a global feels icky. What's the better pattern, given workflow.Go doesn't accept additional args?
	CONTINUE_AS_NEW_CHANNEL workflow.Channel
)

type Messages struct {
	UpdateInfo UpdateSignal
}

func (s *Messages) HandleSignals(ctx workflow.Context, canChannel workflow.Channel) {
	logger := workflow.GetLogger(ctx)

	for {
		workflow.NewSelector(ctx).
			AddReceive(workflow.GetSignalChannel(ctx, "update"), func(c workflow.ReceiveChannel, _ bool) {
				var updateInfo UpdateSignal
				c.Receive(ctx, &updateInfo)
				logger.Info("Received update signal", "data", updateInfo)
				// TODO: apply updates
			}).
			AddReceive(workflow.GetSignalChannel(ctx, "cancel"), func(c workflow.ReceiveChannel, _ bool) {
				var cancel UpdateSignal
				c.Receive(ctx, &cancel)
				logger.Info("Received cancel signal", "data", cancel)

				if cancel.CancelSubscription {
					// TODO: cancel subscription
				}
			}).
			Select(ctx)
	}
}

func pollHistorySize(ctx workflow.Context) {
	logger := workflow.GetLogger(ctx)
	info := workflow.GetInfo(ctx)

	for {
		_ = workflow.Sleep(ctx, time.Duration(HISTORY_CHECK_PERIOD_SECONDS*int(time.Second)))

		logger.Info("Current history length", "History Length", info.GetCurrentHistoryLength())
		if info.GetCurrentHistoryLength() > EVENT_HISTORY_THRESHOLD {
			CONTINUE_AS_NEW_CHANNEL.Send(ctx, true)
		}
	}
}

func SubscriptionWorkflow(ctx workflow.Context, subscription SubscriptionInfo) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	logger.Info("Subscription workflow started", "customer", subscription.Customer, "subscription", subscription)

	// Immediately kick off update/cancel handlers, since those can come in at
	// any time. Channel used to signal that Continue-As-New is needed
	var m Messages
	CONTINUE_AS_NEW_CHANNEL = workflow.NewChannel(ctx)
	workflow.GoNamed(ctx, "signalPoller", func(ctx workflow.Context) {
		m.HandleSignals(ctx, CONTINUE_AS_NEW_CHANNEL)
	})

	continueAsNew := false
	continueHandler := func(c workflow.ReceiveChannel, _ bool) {
		var canMessage bool
		c.Receive(ctx, &canMessage)
		if canMessage {
			logger.Info("History size too big. time to CAN")
			// TODO continue-as-new
			continueAsNew = true
		}
	}
	workflow.GoNamed(ctx, "historyPoller", pollHistorySize)

	var a *Activities
	err := workflow.ExecuteActivity(ctx, a.SendWelcomeEmail, subscription).Get(ctx, nil)
	if err != nil {
		logger.Error("Welcome Email Activity failed.", "Error", err)
		return err
	}

	// Start trial period timer, async
	trialDuration := time.Duration(subscription.TrialPeriodDays * 24 * float64(time.Hour))
	trialTimer := workflow.NewTimer(ctx, trialDuration)
	expectedTimerFire := workflow.Now(ctx).Add(trialDuration)

	var timerError error
	workflow.NewSelector(ctx).
		AddFuture(trialTimer, func(f workflow.Future) {
			logger.Info("Trial has ended. Sending 'trial is over' email, then entering normal billing cycle.")
			timerError = f.Get(ctx, nil)
		}).
		AddReceive(CONTINUE_AS_NEW_CHANNEL, continueHandler).
		// wait for update/cancel signals, Continue-As-New channel, or Trial timer.
		Select(ctx)
	if timerError != nil {
		logger.Error("Trial timer failed.", "Error", err)
	}

	// if we're here because of Continue-As-New, then do that. Otherwise continue as normal.
	if continueAsNew {
		remainingTime := expectedTimerFire.Sub(workflow.Now(ctx))
		logger.Info("Continuing as New.", "Remaining time in trial period", remainingTime)

		subscription.TrialPeriodRemaining = remainingTime
		canError := workflow.NewContinueAsNewError(ctx, SubscriptionWorkflow, subscription)
		return canError
	}

	subscription.TrialPeriodRemaining = time.Duration(0)
	err = workflow.ExecuteActivity(ctx, a.SendTrialExpiredEmail, subscription).Get(ctx, nil)
	if err != nil {
		logger.Error("SendTrialExpiredEmail Activity failed.", "Error", err)
		return err
	}

	for !m.UpdateInfo.CancelSubscription {
		// start of billing cycle, charge a subscription
		err = workflow.ExecuteActivity(ctx, a.ChargeSubscription, subscription).Get(ctx, nil)
		if err != nil {
			logger.Error("ChargeSubscription Activity failed.", "Error", err)
			return err
		}

		timerDuration := time.Duration(subscription.BillingPeriodDays * 24 * float64(time.Hour))
		billingTimer := workflow.NewTimer(ctx, timerDuration)
		expectedTimerFire = workflow.Now(ctx).Add(timerDuration)

		workflow.NewSelector(ctx).
			AddFuture(billingTimer, func(f workflow.Future) {
				// start timer for billing period. Must be async as update/cancel signals may come in.
				logger.Info("Billing cycle is up! Time to charge...")
			}).
			AddReceive(CONTINUE_AS_NEW_CHANNEL, continueHandler).
			Select(ctx)

		// if we're here because of Continue-As-New, then do that. Otherwise continue as normal.
		if continueAsNew {
			remainingTime := expectedTimerFire.Sub(workflow.Now(ctx))
			logger.Info("Continuing as New.", "Remaining time in billing period", remainingTime)

			subscription.BillingPeriodRemaining = remainingTime
			canError := workflow.NewContinueAsNewError(ctx, SubscriptionWorkflow, subscription)
			return canError
		}
	}

	err = workflow.ExecuteActivity(ctx, a.SendCancelationEmail, subscription).Get(ctx, nil)
	if err != nil {
		logger.Error("SendCancelationEmail Activity failed.", "Error", err)
		return err
	}

	logger.Info("Subscription workflow completed.")
	return nil
}
