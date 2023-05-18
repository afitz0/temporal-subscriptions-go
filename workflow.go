package subscription

import (
	"errors"
	"time"

	"go.temporal.io/sdk/workflow"
)

// This threshold is purposefully short to allow for easier
// testing and demonstration of Continue-As-New. The Temporal Events in
// this application are small enough that it's unlikely to hit the event
// *size* limit, and so in the "real" version of this,
// EventHistoryThreshold should be much closer to 50k.
const EventHistoryThreshold = 50

func SubscriptionWorkflow(ctx workflow.Context, subscription SubscriptionInfo) (err error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	logger := workflow.GetLogger(ctx)
	logger.Info("Subscription workflow started", "customer", subscription.Customer, "subscription", subscription)

	err = subscription.trialPeriod(ctx)
	var continueAsNewErr *workflow.ContinueAsNewError
	if err != nil && errors.As(err, &continueAsNewErr) {
		return subscription.maybeContinueAsNew(ctx, err)
	} else if err != nil {
		logger.Error("Error while handling trial period", "Error", err)
		return err
	}

	err = subscription.billingCycle(ctx)
	if err != nil && errors.As(err, &continueAsNewErr) {
		return subscription.maybeContinueAsNew(ctx, err)
	} else if err != nil {
		logger.Error("Error while handling billing cycle", "Error", err)
		return err
	}

	err = subscription.cancelSubscription(ctx)
	if err != nil {
		logger.Error("Canceling subscription failed.", "Error", err)
		return err
	}

	logger.Info("Subscription workflow completed.")
	return nil
}

func (s *SubscriptionInfo) trialPeriod(ctx workflow.Context) (err error) {
	logger := workflow.GetLogger(ctx)
	selector := workflow.NewSelector(ctx)
	info := workflow.GetInfo(ctx)

	var a *Activities
	var cancelSub bool

	if !s.TrialPeriodExpiration.IsZero() && s.TrialPeriodExpiration.After(workflow.Now(ctx)) {
		err = workflow.ExecuteActivity(ctx, a.SendWelcomeEmail, s).Get(ctx, nil)
		if err != nil {
			logger.Error("Welcome Email Activity failed.", "Error", err)
			return err
		}

		// Start trial period timer, async
		trialDuration := s.TrialPeriodExpiration.Sub(workflow.Now(ctx))
		trialTimer := workflow.NewTimer(ctx, trialDuration)

		var timerError error
		trialOver := false
		selector.AddFuture(trialTimer, func(f workflow.Future) {
			logger.Info("Trial has ended. Sending 'trial is over' email, then entering normal billing cycle.")
			trialOver = true
			timerError = f.Get(ctx, nil)
		})

		for !trialOver {
			selector.
				// Signal Handler for updating billing address or other customer info
				AddReceive(workflow.GetSignalChannel(ctx, "update"), func(c workflow.ReceiveChannel, _ bool) {
					var updateInfo UpdateSignal
					c.Receive(ctx, &updateInfo)
					logger.Info("Received update signal.", "Data", updateInfo)
					s = &updateInfo.Subscription
				}).
				// Signal handler for canceling the subscription
				AddReceive(workflow.GetSignalChannel(ctx, "cancel"), func(c workflow.ReceiveChannel, _ bool) {
					var cancel UpdateSignal
					c.Receive(ctx, &cancel)
					logger.Info("Received cancel signal.", "Data", cancel)

					cancelSub = cancel.CancelSubscription
				})

			err = workflow.Await(ctx, func() bool {
				logger.Info("Current history length", "History Length", info.GetCurrentHistoryLength())
				return info.GetCurrentHistoryLength() > EventHistoryThreshold || selector.HasPending()
			})
			if err != nil {
				logger.Error("Awaiting history or selector failed.", "Error", err)
				return err
			}

			if info.GetCurrentHistoryLength() > EventHistoryThreshold {
				canError := workflow.NewContinueAsNewError(ctx, SubscriptionWorkflow, *s)
				return canError
			}

			selector.Select(ctx)

			if timerError != nil {
				logger.Error("Trial timer failed.", "Error", err)
				return err
			}

			if cancelSub {
				logger.Info("Received request to cancel subscription. Not continuing into normal billing cycle. Workflow done.")
				return nil
			}
		}

		s.TrialPeriodExpiration = time.Time{}
		err = workflow.ExecuteActivity(ctx, a.SendTrialExpiredEmail, s).Get(ctx, nil)
		if err != nil {
			logger.Error("SendTrialExpiredEmail Activity failed.", "Error", err)
			return err
		}
	}
	return nil
}

func (s *SubscriptionInfo) billingCycle(ctx workflow.Context) (err error) {
	logger := workflow.GetLogger(ctx)
	selector := workflow.NewSelector(ctx)
	info := workflow.GetInfo(ctx)

	var a *Activities
	cancelSub := false

	for !cancelSub {
		// start of billing cycle, charge a subscription
		err = workflow.ExecuteActivity(ctx, a.ChargeSubscription, s).Get(ctx, nil)
		if err != nil {
			logger.Error("ChargeSubscription Activity failed.", "Error", err)
			return err
		}

		timerDuration := s.BillingPeriodDuration
		if !s.BillingPeriodExpiration.IsZero() {
			timerDuration = s.BillingPeriodExpiration.Sub(workflow.Now(ctx))
		} else {
			s.BillingPeriodExpiration = workflow.Now(ctx).Add(timerDuration)
		}
		billingTimer := workflow.NewTimer(ctx, timerDuration)
		var timerError error

		selector.
			AddFuture(billingTimer, func(f workflow.Future) {
				// start timer for billing period. Must be async as update/cancel signals may come in.
				logger.Info("Billing cycle is up! Time to charge.")
				timerError = f.Get(ctx, nil)
			}).
			// Signal Handler for updating billing address or other customer info
			AddReceive(workflow.GetSignalChannel(ctx, "update"), func(c workflow.ReceiveChannel, _ bool) {
				var updateInfo UpdateSignal
				c.Receive(ctx, &updateInfo)
				logger.Info("Received update signal.", "Data", updateInfo)
				s = &updateInfo.Subscription
			}).
			// Signal handler for canceling the subscription
			AddReceive(workflow.GetSignalChannel(ctx, "cancel"), func(c workflow.ReceiveChannel, _ bool) {
				var cancel UpdateSignal
				c.Receive(ctx, &cancel)
				logger.Info("Received cancel signal.", "Data", cancel)

				cancelSub = cancel.CancelSubscription
			})

		err = workflow.Await(ctx, func() bool {
			logger.Info("Current history length", "HistoryLength", info.GetCurrentHistoryLength())
			return info.GetCurrentHistoryLength() > EventHistoryThreshold || selector.HasPending()
		})
		if err != nil {
			logger.Error("Awaiting history or selector failed.", "Error", err)
			return err
		}

		if info.GetCurrentHistoryLength() > EventHistoryThreshold {
			canError := workflow.NewContinueAsNewError(ctx, SubscriptionWorkflow, *s)
			return canError
		}

		selector.Select(ctx)

		if timerError != nil {
			logger.Error("Billing timer failed.", "Error", err)
			return err
		}
	}

	return nil
}

func (s *SubscriptionInfo) drainSignals(ctx workflow.Context) (*SubscriptionInfo, error) {
	// If there are *any* cancel signals,
	signalChannel := workflow.GetSignalChannel(ctx, "cancel")
	var cancel UpdateSignal
	ok := signalChannel.ReceiveAsync(&cancel)
	if ok && cancel.CancelSubscription {
		workflow.GetLogger(ctx).Info("Received cancel signal.")
		return &SubscriptionInfo{}, errors.New("cancel")
	}

	for {
		var update UpdateSignal
		signalChannel = workflow.GetSignalChannel(ctx, "update")
		ok := signalChannel.ReceiveAsync(&update)
		if !ok {
			break
		}
		workflow.GetLogger(ctx).Info("Received update signal.", "SubscriptionInfo", update.Subscription)
		// overwriting, but keep whatever the last one was.
		s = &update.Subscription
	}

	return s, nil
}

func (s *SubscriptionInfo) cancelSubscription(ctx workflow.Context) (err error) {
	var a *Activities
	err = workflow.ExecuteActivity(ctx, a.SendCancelationEmail, s).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("SendCancelationEmail Activity failed.", "Error", err)
		return err
	}
	return nil
}

func (s *SubscriptionInfo) maybeContinueAsNew(ctx workflow.Context, canError error) error {
	workflow.GetLogger(ctx).Info("Received Continue-As-New, draining and then possibly returning as expected.")
	s, err := s.drainSignals(ctx)

	// if draining found a cancel signal, don't actually continue-as-new, but finish out canceling
	if err != nil && err.Error() == "cancel" {
		err = s.cancelSubscription(ctx)
		if err != nil {
			workflow.GetLogger(ctx).Error("Canceling subscription failed.", "Error", err)
			return err
		}
		return nil
	}
	workflow.GetLogger(ctx).Info("Returning Continue-As-New.")
	return canError
}
