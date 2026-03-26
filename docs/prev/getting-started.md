# prev Getting Started

Use `prev` to review local diffs, git history, and merge/pull requests from the terminal.

## Install

Choose one of the supported install paths:

```bash
brew install sanix-darker/tap/prev
# or
go install github.com/sanix-darker/prev@latest
```

## Configure a Provider

Set credentials for the provider you want to use. OpenAI is the default:

```bash
export OPENAI_API_KEY=sk-xxx
```

Other supported providers include Anthropic, Azure OpenAI, Gemini, Ollama, Groq, Together, LM Studio, and generic OpenAI-compatible endpoints.

## First Commands

```bash
prev diff fixtures/test_diff1.py,fixtures/test_diff2.py
prev branch feature-branch --repo /path/to/repo
prev mr review my-group/my-project 42 --dry-run
prev config init
prev config validate
```

## Next Reading

- `README.md` for the full command surface
- `docs/prev/features.md` for reviewer capabilities
- `WIKI.md` for configuration reference
