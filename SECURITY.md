# Security policy

## Report a vulnerability privately

**Reporting channel: open a support ticket at https://portal.imprezahost.com/submitticket.php**
(the portal is also reachable over its onion address). Mark it as a security report.

Do not post exploit details or sensitive information in public issues.

A useful report includes the source commit, build toolchain, affected configuration,
expected behavior and a minimal reproduction with synthetic data. Omit credentials,
access keys, customer data, private endpoints and identifying infrastructure details.
The maintainer will arrange any sensitive evidence through the selected channel.

## Handling and updates

The maintainer will assess impact, reproduce reports in isolation and coordinate
fixes and disclosure with reporters. No response deadline or support window is
promised before the public maintenance policy is announced. Availability of source
or a reproducible build does not mean a report has been resolved.

Use releases and checksums announced by this fork's maintainer. Upstream update
feeds do not identify this fork. The build recipe requires Go 1.26.10 and the
tracked dependency lock. Check both source and executable with a current,
independently obtained vulnerability database, recording its timestamp.
Retain all findings and assess reachability; no clean-scanner claim is made here.

## Scope

This policy covers the maintained fork and its build tooling. Deployment security
also depends on authentication, authorization, TLS, encryption configuration,
network access and backups chosen by the operator. Do not use a production system
or another person's data to demonstrate a vulnerability.

License and attribution notices are preserved in LICENSE, NOTICE and the source.
