# Razorpay test-mode foundation

The API accepts a complete optional Razorpay configuration only when the key ID begins with `rzp_test_`. `RAZORPAY_KEY_SECRET` and `RAZORPAY_WEBHOOK_SECRET` are separate server-only secrets. Keep them in ignored local environment files or Secret Manager; `.env.example` contains blank names only. Missing credentials leave payments disabled. Live-mode key IDs are rejected at startup and by the provider client.

`backend/internal/payments` provides a bounded-timeout REST client for creating Razorpay orders and fetching payments, validates provider responses against the requested amount, currency, receipt or ID, and implements constant-time HMAC-SHA256 verification for checkout callbacks and webhook raw bodies. The standard-checkout signature message is `razorpay_order_id + "|" + razorpay_payment_id`; webhook signatures use the exact request bytes and the separate webhook secret. A browser callback or valid signature alone does not mark an order paid; the API must fetch authoritative payment state and require the expected provider order, amount, INR currency and captured status before fulfillment.

The current checkout route still creates a PostgreSQL `pending_payment` order and explicitly returns `paymentAvailable: false`. No payment attempt is inserted, no order is sent to Razorpay, no browser checkout is launched, and no webhook is registered or processed yet. This foundation is verified with isolated HTTP contract tests, not a real Razorpay account. Do not describe it as an end-to-end payment integration.

## Local setup

1. Obtain Razorpay **test-mode** API key ID and API key secret from an authorized merchant dashboard.
2. Configure a distinct test webhook secret after registering a test webhook URL for the events used by the implementation.
3. Add these values only to ignored `backend/.env.local`; never put API or webhook secrets in frontend configuration.
4. Restart the Go API. It rejects partial configuration and any live-mode key ID.
5. The real checkout lifecycle, webhook endpoint and database transaction integration must be implemented and verified before test checkout can be enabled.

The client follows Razorpay's [Orders API](https://razorpay.com/payment-gateway/) and [standard integration signature guidance](https://razorpay.com/docs/server-integration/python/test-app/). Webhook verification must use the [raw body and webhook signature](https://razorpay.com/blog/payment-gateway-webhooks-india/) before payload parsing.
