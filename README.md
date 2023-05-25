# Temporal Subscription Workflow

A workflow representing a customer's subscription to a service. This project is in Go, but is
conceptually similar to the [TypeScript subscription project](https://learn.temporal.io/tutorials/typescript/subscriptions/).

The lifecycle of this subscription workflow is:

1. Initiate subscription
    - Creates a new workflow for this customer.
    - Send welcome email
    - Wait for the given trial period to end
2. Loop on Billing Period
    - For the configured billing period, wait for it
    - Then charge the fee

Meanwhile, a couple other things happen in parallel:

1. Watch for a signal to cancel the subscription
2. Watch for a signal to update the customer or subscription information
3. Check the history length to determine if a "Continue-As-New" is necessary.

Roughly, the Workflow follows the steps outlind in this diagram:

![Process Diagram for the Workflow](./process%20diagram.png)
