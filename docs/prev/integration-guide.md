# Integration Guide

## GitLab CI/CD Pipeline Integration

Add `prev` to your GitLab CI pipeline to automatically review merge requests.

### Pipeline Configuration

Add to your `.gitlab-ci.yml`:

```yaml
code-review:
  stage: review
  image: golang:1.24
  variables:
    PREV_PROVIDER: "openai"
  before_script:
    # Pin prev in CI for reproducible builds. Update this tag intentionally.
    - go install github.com/sanix-darker/prev@v0.0.6
  script:
    - prev mr review $CI_PROJECT_ID $CI_MERGE_REQUEST_IID
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
  allow_failure: true
```

`prev` currently requires Go `>= 1.24` (see `go.mod`). If your runner uses
an older Go image, `go install` will fail.

### Required CI/CD Variables

Set these in your GitLab project under **Settings > CI/CD > Variables**:

| Variable | Description | Required |
|----------|-------------|----------|
| `GITLAB_TOKEN` | GitLab personal access token (scope: `api`) | Yes |
| `OPENAI_API_KEY` | OpenAI API key | If using OpenAI |
| `ANTHROPIC_API_KEY` | Anthropic API key | If using Claude |

### Creating a GitLab Personal Access Token

1. Go to **User Settings > Access Tokens**
2. Create a token with the `api` scope
3. Store it as a CI/CD variable named `GITLAB_TOKEN`

### Dry Run Mode

Use `--dry-run` to test without posting comments:

```yaml
script:
  - prev mr review $CI_PROJECT_ID $CI_MERGE_REQUEST_IID --dry-run
```

### Using Different Providers

```yaml
# With Claude
variables:
  PREV_PROVIDER: "anthropic"
script:
  - prev mr review $CI_PROJECT_ID $CI_MERGE_REQUEST_IID --provider anthropic
```

## Pre-commit Hook

Run `prev` as a pre-commit hook to review staged changes before committing.

### Setup

Create `.git/hooks/pre-commit`:

```bash
#!/bin/bash
# Run prev on staged files
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM)

if [ -z "$STAGED_FILES" ]; then
  exit 0
fi

# Create temp files for the staged version
for file in $STAGED_FILES; do
  git show ":$file" > "/tmp/prev_staged_$file" 2>/dev/null || continue
  if [ -f "$file" ]; then
    prev diff "/tmp/prev_staged_$file,$file" --stream=false 2>/dev/null
  fi
  rm -f "/tmp/prev_staged_$file"
done
```

Make it executable:

```bash
chmod +x .git/hooks/pre-commit
```

## Docker Usage

### Basic Usage

```bash
docker run --rm \
  -e OPENAI_API_KEY=sk-xxx \
  -v $(pwd):/workspace \
  prev diff /workspace/file1.py,/workspace/file2.py
```

### GitLab MR Review

```bash
docker run --rm \
  -e GITLAB_TOKEN=glpat-xxx \
  -e OPENAI_API_KEY=sk-xxx \
  prev mr review 12345 67
```

### With Local Ollama

```bash
docker run --rm \
  --network host \
  prev diff file1.py,file2.py --provider ollama --model llama3
```

## Provider Configuration Reference

### Environment Variables

| Provider | Key Variable | Model Variable | Base URL Variable |
|----------|-------------|----------------|-------------------|
| OpenAI | `OPENAI_API_KEY` | `OPENAI_API_MODEL` | `OPENAI_API_BASE` |
| Anthropic | `ANTHROPIC_API_KEY` | `ANTHROPIC_MODEL` | `ANTHROPIC_BASE_URL` |
| Azure | `AZURE_OPENAI_API_KEY` | `AZURE_OPENAI_DEPLOYMENT` | `AZURE_OPENAI_ENDPOINT` |
| Ollama | — | `OLLAMA_MODEL` | `OLLAMA_BASE_URL` |
| Groq | `GROQ_API_KEY` | `GROQ_MODEL` | — |
| Together | `TOGETHER_API_KEY` | `TOGETHER_MODEL` | — |
| LM Studio | — | `LMSTUDIO_MODEL` | `LMSTUDIO_BASE_URL` |
| OpenAI-compat | `OPENAI_COMPAT_API_KEY` | `OPENAI_COMPAT_MODEL` | `OPENAI_COMPAT_BASE_URL` |

### Config File

Location: `~/.config/prev/config.yml`

```yaml
# Default provider
provider: openai

# Debug mode
debug: false

# Streaming output
stream: true
```

Generate a default config:

```bash
prev config init
```
