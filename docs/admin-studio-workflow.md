# Studio administration workflow

## Access setup

The studio queues are available at `/admin/appointments` and `/admin/custom-orders`. Every API request requires an active session and the `admin` database role. Customer registrations receive only the `customer` role; there is no public role-upgrade endpoint.

To create a first local administrator, apply migrations and set the setup values only for the current PowerShell process. The password prompt is hidden, the command removes the password variable as soon as it reads it, and it never upgrades an existing account:

```powershell
$env:ADMIN_SETUP_EMAIL = 'studio-owner@example.com'
$env:ADMIN_SETUP_NAME = 'Studio Owner'
$secureAdminPassword = Read-Host 'Choose a new administrator password' -AsSecureString
try {
    $env:ADMIN_SETUP_PASSWORD = [System.Net.NetworkCredential]::new('', $secureAdminPassword).Password
    Push-Location backend
    go run ./cmd/seed-admin
} finally {
    Pop-Location
    Remove-Item Env:ADMIN_SETUP_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item Env:ADMIN_SETUP_EMAIL -ErrorAction SilentlyContinue
    Remove-Item Env:ADMIN_SETUP_NAME -ErrorAction SilentlyContinue
    $secureAdminPassword.Dispose()
}
```

Use a unique password and do not put it in command arguments, a tracked file, or logs. `cmd/seed-admin` requires the existing PostgreSQL commerce schema and creates a new Argon2id-backed account with the admin role in one transaction. It intentionally refuses an existing email.

## Request queues and transitions

The lists support status filtering, name/contact search, and bounded pagination. A status change includes the status the operator last read, so a stale browser cannot overwrite a newer staff decision. The database locks the request row, validates the current status, updates the request, and inserts an audit event in the same transaction.

| Request | Allowed staff transitions |
| --- | --- |
| Appointment | `requested → confirmed` or `cancelled`; `confirmed → completed` or `cancelled` |
| Made to order | `requested → reviewing`, `declined`, or `cancelled`; `reviewing → fitting`, `quoted`, `declined`, or `cancelled`; `fitting → quoted`, `declined`, or `cancelled`; `quoted → awaiting_approval`, `declined`, or `cancelled`; `awaiting_approval → declined` or `cancelled` |

Quoting requires a positive amount in integer paise. The staff API cannot set a request to `approved`, `in_progress`, or `completed`; a customer-approval flow and its downstream fulfillment gates must be implemented before those states can be used. A quotation inquiry does not create an order or charge a customer.

Audit entries record the actor, request ID, entity ID, action, and from/to states. They do not copy customer names, contact details, measurements, or request notes into the audit metadata.

## Endpoints

- `GET /api/v1/admin/appointments`
- `PATCH /api/v1/admin/appointments/{id}`
- `GET /api/v1/admin/custom-order-requests`
- `PATCH /api/v1/admin/custom-order-requests/{id}`

The OpenAPI file describes role, CSRF, filters, request bodies, and conflict responses.
