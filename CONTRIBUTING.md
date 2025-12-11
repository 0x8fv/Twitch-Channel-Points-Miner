# Contributing to this repository

Thank you for your interest in contributing to **Twitch-Channel-Points-Miner**!  
This project is community-driven, and every improvement—big or small—helps make the miner more reliable and feature-rich.

---

## Getting started

Before contributing:

- Please review the **[Code of Conduct](./CODE_OF_CONDUCT.md)**.
- Check **[existing issues](https://github.com/0x8fv/Twitch-Channel-Points-Miner/issues)** to avoid duplicates.

If an issue related to your idea already exists, join the discussion instead of opening a new one.

---

## Ready to make a change? Fork the repo

### Fork using GitHub Desktop
- Install & set up GitHub Desktop:  
  <https://docs.github.com/en/desktop/installing-and-configuring-github-desktop/getting-started-with-github-desktop>
- Learn how to clone and fork repositories using Desktop:  
  <https://docs.github.com/en/desktop/contributing-and-collaborating-using-github-desktop/cloning-and-forking-repositories-from-github-desktop>

### Fork using the command line
Guide on how to fork a repo:  
<https://docs.github.com/en/get-started/quickstart/fork-a-repo>

### Fork using GitHub Codespaces
Run and edit the project in the cloud with no local setup required:  
<https://docs.github.com/en/codespaces/developing-in-codespaces/creating-a-codespace>

---

## Opening a Pull Request (PR)

Once your changes are ready, open a **[Pull Request](https://github.com/0x8fv/Twitch-Channel-Points-Miner/pulls)**.  
Be sure to fill out the PR template so reviewers understand your changes and intent.

---

## Getting your PR reviewed

- Begin with a **self-review** (checklist below).
- Maintainers/community contributors will review your PR and may request changes.
- Monitor your PR for comments and questions.
- If you encounter merge conflicts, see:  
  **[Resolving merge conflicts](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/addressing-merge-conflicts)**

### After your PR is merged
🎉 Congratulations — you’re now a contributor!  
You’ll appear in the **[Contributor Graph](https://github.com/0x8fv/Twitch-Channel-Points-Miner/graphs/contributors)**.

---

# Types of contributions 📝

You can help by:

- Reporting bugs  
- Improving documentation  
- Submitting patches or new features  
- Suggesting enhancements  
- Writing or improving tests  
- Helping with code quality and performance  

Every contribution is appreciated!

---

## Issues

Use **[Issues](https://github.com/0x8fv/Twitch-Channel-Points-Miner/issues)** to report:

- Bugs  
- Feature requests  
- Questions  
- Enhancement ideas  
- Clarifications  

Please use the appropriate template when opening an issue.

---

## Labels

Labels help identify an issue’s purpose and status:

- **`bug`** — Something isn’t working  
- **`documentation`** — README or docs updates  
- **`duplicate`** — Already reported  
- **`enhancement`** — New feature or improvement  
- **`help wanted`** — Help needed from contributors  
- **`improvements`** — Improve existing feature  
- **`invalid`** — Issue doesn’t meet guidelines  
- **`question`** — Additional details needed  
- **`wontfix`** — Issue won’t be addressed  

Browse all labels here:  
<https://github.com/0x8fv/Twitch-Channel-Points-Miner/labels>

---

# Opening a Pull Request

For small edits (documentation, typos), feel free to use GitHub’s web editor.

For code changes:

1. **Fork** the repository  
2. **Clone** your fork  
3. Create a **new branch**  
4. Implement your updates  
5. Run formatting, linting, vetting, and tests  
6. Commit & push  
7. Open a **Pull Request**  

General PR documentation:  
<https://docs.github.com/en/pull-requests>

---

# Go Style Guide

This project follows standard Go tooling and conventions.

---

## Required Formatting

All Go code **must** follow `gofmt` formatting.

Run:

```bash
gofmt -w .
```

Or:

```bash
go fmt ./...
```

Unformatted code will not be accepted.

---

## Linting

We recommend using **golangci-lint**:

- Documentation: <https://golangci-lint.run/>

Run linting:

```bash
golangci-lint run
```

---

## Vetting

Run Go’s vet tool to catch common mistakes:

```bash
go vet ./...
```

---

## Testing

Use tests to ensure functionality remains stable:

```bash
go test ./...
```

All tests must pass before your PR will be accepted.

---

# Self-review checklist

Before marking your PR as “Ready for review,” ensure:

- [ ] The PR fully addresses the linked issue  
- [ ] Code compiles with no errors  
- [ ] All Go files are formatted (`gofmt` / `go fmt`)  
- [ ] `go vet` shows no critical issues  
- [ ] Linting (`golangci-lint run`) passes  
- [ ] All tests pass (`go test ./...`)  
- [ ] Code follows Go best practices and project structure  
- [ ] Documentation is updated where needed  
- [ ] All CI checks are passing  

---

## Suggested changes from reviewers

Reviewers may request modifications using:

- Inline comments  
- Suggested code snippets  
- Review summaries  

When updating your PR:

- Push commits normally (do **not** squash unless asked)
- Respond to reviewer feedback
- Mark discussions as **[Resolved](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests#resolving-conversations)** using GitHub’s interface

---

Thank you for contributing to **Twitch-Channel-Points-Miner**! 🚀  
Your help makes the project better for everyone.
