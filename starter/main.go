package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"

	subs "github.com/afitz0/temporal-subscriptions-go"
	"github.com/afitz0/temporal-subscriptions-go/zapadapter"
)

func main() {
	c, err := client.Dial(client.Options{
		Logger: zapadapter.NewZapAdapter(
			zapadapter.NewZapLogger()),
	})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	uid := uuid.New()

	customer := subs.CustomerInfo{
		Name:  "Fitz",
		Email: "a@a.com",
		UID:   uid.String(),
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "cid-" + customer.UID,
		TaskQueue: "subscriptions",
	}

	// SUPER short periods to allow for easier testing.
	sub := subs.SubscriptionInfo{
		TrialPeriodExpiration: time.Now().Add(time.Duration(time.Second * 5)),
		AmountCents:           1000,
		BillingPeriodDuration: time.Duration(time.Second * 5),
		Customer:              customer,
	}

	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, subs.SubscriptionWorkflow, sub)
	if err != nil {
		log.Fatalln("Unable to start workflow", err)
	}

	log.Println("Started workflow", "WorkflowID", we.GetID(), "RunID", we.GetRunID())

	/*
		// Synchronously wait for the workflow completion.
		err = we.Get(context.Background(), nil)
		if err != nil {
			log.Fatalln("Unable get workflow result", err)
		}
		log.Println("Workflow Done!")
	*/
}
