package main

import (
	"context"
	"fmt"
	"log"

	"github.com/pborman/uuid"
	"go.temporal.io/sdk/client"

	subs "github.com/afitz0/temporal-subscriptions-go"
	"github.com/afitz0/temporal-subscriptions-go/zapadapter"
)

func main() {
	c, err := client.NewLazyClient(client.Options{
		Logger: zapadapter.NewZapAdapter(
			zapadapter.NewZapLogger()),
	})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	customer := subs.CustomerInfo{
		Name:  "Fitz",
		Email: "a@a.com",
		UID:   uuid.New(),
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "cid-" + customer.UID,
		TaskQueue: "subscriptions",
	}

	// SUPER short periods to allow for easier testing.
	sub := subs.SubscriptionInfo{
		TrialPeriodDays:   float64(10.0 / (24 * 60 * 60)),
		Amount:            10.00,
		BillingPeriodDays: float64(0.5 / (24 * 60 * 60)),
		Customer:          customer,
	}

	fmt.Println("trial: ", sub.TrialPeriodDays)

	we, err := c.ExecuteWorkflow(context.Background(), workflowOptions, subs.SubscriptionWorkflow, sub)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
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
