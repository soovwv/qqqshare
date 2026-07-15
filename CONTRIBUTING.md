# Contributing to QQQShare

Thanks for helping improve QQQShare. Bug reports, security reviews, documentation, translations, and code contributions are welcome.

By participating, you agree to follow [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Contributions are accepted under the repository's [MIT License](LICENSE). By submitting a pull request, you confirm that you have the right to contribute the work under that license.

## Before opening an issue

- Search existing issues first.
- Never include active share URLs, owner tokens, private file names, or sensitive logs.
- Use GitHub private vulnerability reporting for security issues; follow [SECURITY.md](SECURITY.md).
- Use Discussions, if enabled, for general support and product ideas.

## Development

Requirements: Go version declared in `go.mod` and PowerShell 7 or Windows PowerShell for the release script.

```powershell
git clone https://github.com/soovwv/qqqshare.git
cd qqqshare
go mod download
go test ./...
go vet ./...
```

Format Go changes with `gofmt`. Keep the portable app dependency-light and preserve the stable JSON schemas used by agents. New network-facing behavior should include authorization, expiry, size-limit, path-safety, and negative tests.

## Pull requests

- Keep each pull request focused on one change.
- Explain user impact and security implications.
- Add or update tests and documentation.
- Confirm `go test ./...`, `go vet ./...`, and `git diff --check` pass.
- Do not commit generated `dist/` files, secrets, tokens, or personal shared data.

Maintainers may request changes, close inactive work, or decline features that expand the trusted-network scope without an appropriate security design.
