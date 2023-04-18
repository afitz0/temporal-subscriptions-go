package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

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

	w := worker.New(c, "subscriptions", worker.Options{})

	a := &subs.Activities{}
	w.RegisterWorkflow(subs.SubscriptionWorkflow)
	w.RegisterActivity(a)

	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}
