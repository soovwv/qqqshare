# Security

QQQShare is intended for trusted private LANs. Do not expose its port through router forwarding or a public tunnel.

## Supported versions

Security fixes are provided for the latest published release only. Development builds and older releases may receive fixes at the maintainer's discretion.

## Reporting a vulnerability

Use [GitHub private vulnerability reporting](https://github.com/soovwv/qqqshare/security/advisories/new). Do not open a public issue and do not include real shared URLs, owner tokens, personal file names, or confidential file contents.

Include the affected version and operating system, impact, safe reproduction steps, and any suggested mitigation. The maintainer will acknowledge the report when reasonably possible, investigate it privately, and coordinate disclosure after a fix or mitigation is available. Please avoid public disclosure while the report is being assessed.

## Defaults

- Random high port and separate random 144-bit owner/read-only tokens per launch
- Shared URLs can only list and download; upload, settings, revoke, and stop require the owner token
- Files expire after one minute by default
- Uploads use a size limit, temporary file, sanitized basename, and atomic rename
- New downloads are blocked before expired files are archived
- Transport is currently plain HTTP; use only on a trusted private LAN
- Treat owner URLs as secrets and never paste them into shared chats or public logs
- Browser responses disable MIME sniffing, framing, and referrer leakage

## Out of scope

- Use on public or hostile networks without an additional secure transport
- Exposure through port forwarding, a public tunnel, reverse proxy, or unsupported relay
- Availability issues caused by local firewall, router, or network isolation policies
- Social engineering that convinces a user to reveal an owner URL or share a sensitive file
