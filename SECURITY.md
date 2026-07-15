# Security

QQQShare is intended for trusted private LANs. Do not expose its port through router forwarding or a public tunnel.

Report vulnerabilities privately through the repository security advisory feature. Do not include real shared URLs or access tokens in public issues.

## Defaults

- Random high port and random 144-bit URL token per launch
- API access requires the token
- Files expire after one minute by default
- Uploads use a size limit, temporary file, sanitized basename, and atomic rename
- New downloads are blocked before expired files are archived
- Browser responses disable MIME sniffing, framing, and referrer leakage
