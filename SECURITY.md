# Security

QQQShare is intended for trusted private LANs. Do not expose its port through router forwarding or a public tunnel.

Report vulnerabilities privately through the repository security advisory feature. Do not include real shared URLs or access tokens in public issues.

## Defaults

- Random high port and separate random 144-bit owner/read-only tokens per launch
- Shared URLs can only list and download; upload, settings, revoke, and stop require the owner token
- Files expire after one minute by default
- Uploads use a size limit, temporary file, sanitized basename, and atomic rename
- New downloads are blocked before expired files are archived
- Transport is currently plain HTTP; use only on a trusted private LAN
- Treat owner URLs as secrets and never paste them into shared chats or public logs
- Browser responses disable MIME sniffing, framing, and referrer leakage
