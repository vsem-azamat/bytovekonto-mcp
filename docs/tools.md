[Docs](README.md) › Tools

# Tools

Dates are `YYYY-MM-DD`; times are as the API sends them (ISO 8601, mostly UTC). Amounts are in the account's currency.

## Reading

| Tool | Arguments | Returns |
| --- | --- | --- |
| `home_overview` | – | Tenancy (unit, period, people, monthly payment with account and symbols), management contacts, announcements, money past due and to pay, the user's open tickets with their last message, unread notification count |
| `building_info` | – | Owner and manager companies with ids and bank accounts, management contacts, announcements, the building's FAQ |
| `list_tickets` | `scope` mine (default), building, all; `state` open, closed, all (default); `query`; `limit` (default 50, max 500) | Tickets newest first with state, category, area, unit, whether the user created it, the last message; `total` before the limit |
| `get_ticket` | `id` | The ticket with its description, priority, members, the conversation oldest first (deleted messages left out), the change history, the number of attachments, whether it takes comments |
| `ticket_options` | – | Categories a resident may use, areas, units (with `own`), the default area |
| `list_payments` | `filter` to_pay (default), overpaid, all; `year`; `month` (needs `year`) | Accounting documents with amount, paid, remaining, due date, status |
| `get_payment` | `id` | One document with its items, variable, specific and constant symbol, due date and note |
| `payments_overview` | – | Past due (and since when), due, paid, overpaid, planned; the same per month, as the API sends it |
| `costs` | `from`, `to` (default `to` today and `from` 1 January of the year of `to`; at most about 13 months); `granularity` day, week, month (default) | Cost per delivery (rent, heat, water, electricity) and period with consumption and unit, whether the period is settled, the current rent |
| `list_meters` | – | Meters of the account's units with id, kind, name and units |
| `get_meter` | `id`; `from`, `to`; `granularity` hour, day, month (default) | Last reading per channel, consumption per period |
| `list_documents` | `query` | Shared documents with category, file name, type and size, newest first |
| `download_document` | `id` | The file, saved in the download directory (path in `saved_to`) and returned inline as an embedded resource when at most 4 MiB; the result is also given as text |
| `notifications` | – | In-app notifications as the API sends them, and the unread count |

## Writing

Both write tools return a preview and send nothing unless `confirm` is true; see [safety.md](safety.md).

| Tool | Arguments | Sends |
| --- | --- | --- |
| `create_ticket` | `category` (code or name: FAULT, ACCIDENT, REQUEST, QUESTION, COMPLAINT, SUGGESTION, ORDER, SERVICE, CATEGORY_OTHER); `area` (default AREA_OTHER); `unit` (id, or a name that only one unit has; default the user's own flat); `subject` (≤ 200 characters); `description` (≤ 10 000); `private` (default false); `confirm` | A new ticket to the management. Returns it as created. |
| `comment_ticket` | `ticket_id`; `message` (≤ 10 000); `confirm` | A message in the ticket's conversation. Refused when the ticket takes no comments; the preview warns when the ticket is another resident's. |
