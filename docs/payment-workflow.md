# Razorpay test-mode foundation

The API accepts a complete optional Razorpay configuration only when the key ID begins with `rzp_test_`. `RAZORPAY_KEY_SECRET` and `RAZORPAY_WEBHOOK_SECRET` are separate server-only secrets. Keep them in ignored local environment files or Secret Manager; `.env.example` contains blank names only. Missing credentials leave payments disabled. Live-mode key IDs are rejected at startup and by the provider client.

`backend/internal/payments` provides a bounded-timeout REST client for creating Razorpay orders and fetching payments, validates provider responses against the requested amount, currency, receipt or ID, and implements constant-time HMAC-SHA256 verification for checkout callbacks and webhook raw bodies. The standard-checkout signature message is `razorpay_order_id + "|" + razorpay_payment_id`; webhook signatures use the exact request bytes and the separate webhook secret. A browser callback or valid signature alone does not mark an order paid; the API fetches authoritative payment state and requires the expected provider order, amount, INR currency and captured status before fulfillment. Razorpay's [official integration guide](https://razorpay.com/docs/server-integration/python/test-app/) requires server signature verification and checking for captured status before fulfillment.

Checkout remains disabled by default. To enable the test flow, configure all three Razorpay **test-mode** secrets and set `CHECKOUT_PRICING_FINAL=true` only if the database subtotal is the complete payable amount for the merchant's shipping and tax rules. Then the API exposes the checkout payment routes, inserts an idempotent local payment attempt before creating a Razorpay order, and returns only the public test key plus the server-created provider order and amount. The web app opens Razorpay Standard Checkout and posts its callback to the Go API. The API verifies the signature, fetches the payment from Razorpay, and changes the order to `paid` and consumes any active inventory reservations only after an amount-matched `captured` result.

An authorized or created payment remains unpaid. A failed provider request leaves its local attempt unresolved; retries with the same key do not silently create another Razorpay order. The reservation worker therefore holds stock for attempts requiring reconciliation. Reconciliation and durable webhook ingestion/deduplication are not implemented yet, so a lost/late callback can require operator review. Refund initiation is also outstanding. This milestone is verified with a fake HTTP provider and a guarded PostgreSQL integration test, not a real Razorpay merchant account; do not describe it as end-to-end payment acceptance.

## Local setup

1. Obtain Razorpay **test-mode** API key ID and API key secret from an authorized merchant dashboard.
2. Configure a distinct test webhook secret after registering a test webhook URL for the events used by the implementation.
3. Add these values only to ignored `backend/.env.local`; never put API or webhook secrets in frontend configuration.
4. Set `CHECKOUT_PRICING_FINAL=true` only after the current subtotal is confirmed as the complete amount to charge; this code does not calculate shipping or tax.
5. Restart the Go API. It rejects partial configuration and live-mode key IDs. With the pricing flag false or secrets missing, payment routes remain unavailable.
6. Use the test-mode checkout to verify order creation and callback handling. A real account checkout and webhook replay/reconciliation still require merchant test credentials and webhook configuration.

The client follows Razorpay's [Orders API](https://razorpay.com/payment-gateway/) and [standard integration signature guidance](https://razorpay.com/docs/server-integration/python/test-app/). Webhook verification must use the [raw body and webhook signature](https://razorpay.com/blog/payment-gateway-webhooks-india/) before payload parsing.
