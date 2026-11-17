[Docs](README.md) › Safety

# Write rules

The server can send two things to a real building's management in the resident's name: a new ticket and a comment. Everything else is read-only. These rules keep the resident in control.

1. **Preview first.** `create_ticket` and `comment_ticket` without `confirm: true` resolve and validate everything, send nothing, and return the exact text, category, area and unit that would be sent. The server instructions and the tool descriptions tell agents to show the preview to the user and to confirm only after the user approved it.
2. **Validation before sending.** Subject and message must be non-empty valid UTF-8 within their limits; the category must be one a resident may use (announcements and discussions are refused); the area and unit must exist in `ticket_options`. A comment is refused when the ticket takes no comments. A failed check sends nothing.
3. **Only what the apps send.** Bodies have exactly the fields the official apps send, including the ones they send as null (`ticketPriority`, `dueDate`, the related entity's `label`, `setIgnoreMemberCountWarning`); a test checks the key sets. There are no tools to change a ticket's state, delete or edit anything, invite people, pay, or change the profile.
4. **Audit.** Every confirmed write is logged at info level (`"msg":"write"`) with the tool, the MCP client's name, the unit or ticket, the subject or new comment id and the outcome; a failed write is logged at warn level with the error. Message texts are not logged beyond the ticket subject.
5. **Secrets.** The password is read from a file or the environment, removed from the environment at start, and sent only in the body of a password grant. Tokens are cached in a 0600 file and never logged; transport errors are logged without URLs; token-shaped strings in the API's error messages are replaced with `<token>`. Redirects to plain http are refused, so the token and the login form never leave TLS.

6. **Failed writes.** When the API refuses a write (a 4xx answer), the tool says it was not sent. When the outcome is unknown (a timeout, a lost or unreadable answer, a 5xx answer), the tool says so and tells the agent to check with `list_tickets` or `get_ticket` before sending again.

A confirmed call is not idempotent: calling it twice sends two tickets or comments.
