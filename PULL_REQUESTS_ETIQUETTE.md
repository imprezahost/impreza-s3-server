# Pull request etiquette

Short guidance so a review is worth everyone's time. The contribution scope
is defined in [CONTRIBUTING.md](CONTRIBUTING.md): security, build and
maintenance only — feature pull requests are closed without review.

- One concern per pull request, against `next`, referencing its issue.
- The description states the defect, the fix and how it was verified; a
  security fix names its test (the one that fails without the change).
- No drive-by refactors, no formatting-only churn, no dependency bumps
  without an accompanying issue explaining why they are in scope.
- Be responsive to review; unaddressed feedback for two weeks closes the
  request.

Security reports follow [SECURITY.md](SECURITY.md) — never an issue and
never a pull request.
