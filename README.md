## PREV

A code review CLI tool that uses AI to review diffs, commits, branches, and GitLab merge requests.

Supports multiple AI providers: **OpenAI**, **Anthropic (Claude)**, **Azure OpenAI**, and any **OpenAI-compatible** API (Ollama, Groq, LM Studio, Together, etc.).

### Installation

```bash
# From source
go install github.com/sanix-darker/prev@latest

# Or clone and build
git clone https://github.com/sanix-darker/prev.git
cd prev
go build -o prev .

# Docker
docker build -t prev .
docker run --rm -e OPENAI_API_KEY=sk-xxx prev version
```

### Quick Start

```bash
# 1. Set up your AI provider credentials
export OPENAI_API_KEY=sk-xxx          # For OpenAI (default)
# OR
export ANTHROPIC_API_KEY=sk-ant-xxx   # For Claude
# OR run locally with Ollama (no key needed)

# 2. Review a diff between two files
prev diff fixtures/test_diff1.py,fixtures/test_diff2.py

# 3. Review a git commit
prev commit abc123 --repo /path/to/repo

# 4. Review a git branch diff
prev branch feature-branch --repo /path/to/repo

# 5. Optimize a code file
prev optim myfile.py
```

### Commands

```
prev diff <file1,file2>           Review diff between two files
prev commit <hash>                Review a git commit
prev branch <name>                Review a branch diff against base
prev optim <file|clipboard>       Optimize code
prev mr review <project> <mr_id>  Review a GitLab merge request
prev mr diff <project> <mr_id>    Show MR diff locally
prev mr list <project>            List open merge requests
prev ai list                      List available AI providers
prev ai show                      Show current provider and model
prev config show                  Show current configuration
prev config init                  Create default config file
prev version                      Print version info
```

### AI Providers

Use `--provider` and `--model` flags to override the default provider for any command.

#### OpenAI (default)

```bash
export OPENAI_API_KEY=sk-xxx
export OPENAI_API_MODEL=gpt-4o     # optional, defaults to gpt-4o

prev diff file1.py,file2.py
```

#### Anthropic (Claude)

```bash
export ANTHROPIC_API_KEY=sk-ant-xxx
export ANTHROPIC_MODEL=claude-sonnet-4-20250514  # optional

prev diff file1.py,file2.py --provider anthropic
```

#### Azure OpenAI

```bash
export AZURE_OPENAI_API_KEY=xxx
export AZURE_OPENAI_ENDPOINT=https://your-resource.openai.azure.com
export AZURE_OPENAI_DEPLOYMENT=gpt-4o

prev diff file1.py,file2.py --provider azure
```

#### Ollama (local, free)

```bash
# Start ollama
ollama serve &
ollama pull llama3

prev diff file1.py,file2.py --provider ollama --model llama3
```

#### Other OpenAI-compatible APIs

```bash
# Groq
export GROQ_API_KEY=gsk_xxx
prev diff file1.py,file2.py --provider groq --model llama-3.3-70b-versatile

# Together
export TOGETHER_API_KEY=xxx
prev diff file1.py,file2.py --provider together --model meta-llama/Llama-3-70b-chat-hf

# LM Studio
prev diff file1.py,file2.py --provider lmstudio --model local-model

# Any OpenAI-compatible endpoint
export OPENAI_COMPAT_API_KEY=xxx
export OPENAI_COMPAT_BASE_URL=https://your-api.example.com/v1
prev diff file1.py,file2.py --provider openai-compat --model your-model
```

### GitLab Merge Request Reviews

Review MRs directly from your terminal, with optional inline comment posting back to GitLab.

```bash
# Set up GitLab credentials
export GITLAB_TOKEN=glpat-xxxx
export GITLAB_URL=https://gitlab.com   # optional, defaults to gitlab.com

# Review an MR (prints review to terminal)
prev mr review 12345 67 --dry-run

# Review and post comments to GitLab
prev mr review 12345 67

# Post only a summary comment (no inline comments)
prev mr review 12345 67 --summary-only

# Use a specific AI provider for the review
prev mr review 12345 67 --provider anthropic

# List open MRs for a project
prev mr list 12345

# View MR diff locally
prev mr diff 12345 67
```

### Configuration

Create a config file at `~/.config/prev/config.yml`:

```bash
prev config init
```

Example config:

```yaml
provider: openai
debug: false
stream: true
```

### Global Flags

```
--provider, -P    AI provider to use (openai, anthropic, azure, ollama, etc.)
--model, -m       Model to use for the AI provider
--stream, -s      Enable streaming output (default: true)
--debug           Enable debug output
--help, -h        Help for any command
```

### Development

```bash
# Run unit tests
make test-unit

# Run E2E tests
make test-e2e

# Build
go build -o prev .

# Docker build
docker build -t prev .
```

### License

See [LICENSE](LICENSE) file.
