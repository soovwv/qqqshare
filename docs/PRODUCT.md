# QQQShare product status and roadmap

Updated: 2026-07-17

## Positioning

QQQShare is not another general-purpose AirDrop clone. Its primary product is a local ephemeral artifact exchange layer that AI agents can operate. The portable desktop UI is a lightweight interface for people using the same core.

```text
Human or agent -> publish -> capability URL + manifest
Receiver       -> inspect -> receive -> SHA-256 verify
Owner agent    -> list/status/revoke -> expire
```

## Current capabilities

- Portable single-binary server for Windows and macOS
- Browser upload and download for devices on the same private LAN
- Separate owner and read-only capability tokens
- Expiry from one second to 24 hours and manual revocation
- Optional one-time downloads
- File manifests and verified CLI downloads using SHA-256
- Agent-friendly JSON for publish, list, status, inspect, receive, and revoke
- Local private registry for ID-based lifecycle management
- QR code, random port, release automation, and open-source project policies

## Competitive landscape

| Product | Primary strength | Difference from QQQShare |
| --- | --- | --- |
| [LocalSend](https://github.com/localsend/localsend) | Mature cross-platform nearby sharing with discovery and HTTPS | Human/device-first; QQQShare focuses on agent-managed expiring artifacts |
| [PairDrop](https://github.com/schlagmichdoch/PairDrop) | Browser P2P, paired devices, public rooms | Receiver discovery and WebRTC rooms rather than a local artifact lifecycle API |
| [croc](https://github.com/schollz/croc) | Encrypted relay, resume, multiple files, CLI | Both sides use croc; QQQShare recipients only need a browser |
| [Magic Wormhole](https://magic-wormhole.readthedocs.io/en/latest/) | PAKE codes and encrypted direct/relay transfer | Receiver client and interactive code exchange are required |

QQQShare's defensible workflow is `publish -> machine-readable URL/manifest -> inspect/verify -> automatic expiry`, with the same contract available to humans and agents.

## Known gaps

1. HTTP transport is appropriate only for trusted private LANs; LocalSend, croc, and Magic Wormhole provide stronger encrypted transports.
2. Interrupted downloads cannot resume.
3. CLI registry records lifecycle state, but is local to one OS user and is not a multi-user daemon database.
4. No MCP server or packaged Claude/Codex skill is shipped yet.
5. No signed Windows binary or notarized macOS release is currently guaranteed.
6. LAN discovery and IPv6 are not implemented.

## Roadmap

### P0 — trustworthy agent core

- Stabilize JSON schemas and structured errors
- Add lifecycle registry, status, ID revocation, and one-time mode
- Test concurrent downloads, expiry, corrupted transfers, and process cleanup
- Publish signed checksums and a reproducible release procedure

### P1 — agent distribution

- Ship a local MCP server exposing publish/list/status/receive/revoke
- Provide Claude and Codex skills with explicit path and expiry confirmation
- Provide thin npm and Python launchers that download or locate the same core binary
- Add policy controls for allowed roots, maximum size, maximum expiry, and network scope

### P2 — secure transport

- Add authenticated TLS for LAN use with a verifiable pairing fingerprint
- Evaluate optional Tailscale and user-operated relay adapters for remote use
- Add resumable downloads and content-range verification
- Preserve capability URLs and manifest compatibility across transports

### P3 — ecosystem

- Publish a versioned artifact manifest specification
- Add interoperability tests and SDK fixtures
- Support signed manifests and optional encrypted-at-rest staging
- Collect opt-in, privacy-preserving reliability metrics only after explicit consent

## Success criteria

- A person can share a file from a portable app to a phone in under 30 seconds.
- An agent can publish, report the URL and expiry, query status, and revoke by ID without parsing human output.
- A receiving agent can inspect and verify every byte before exposing the final file.
- No owner token is required to leave the source machine.
