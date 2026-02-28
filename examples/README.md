# Examples

This directory contains ready-to-use `prev` configuration profiles.

These files are documentation and operational examples only. They are not embedded into the binary.

## Available Profiles

- `configs/v1-minimal-openai.yml`: minimal OpenAI setup
- `configs/v1-gitlab-ci-review.yml`: CI-oriented MR review profile
- `configs/v1-strict-mr-review.yml`: strict/high-coverage MR review
- `configs/v1-gemini.yml`: Gemini provider via OpenAI-compatible endpoint
- `configs/v1-local-ollama.yml`: local/self-hosted ollama profile

## CI Examples

- `ci/gitlab-ci.yml`: GitLab pipeline job for MR review
- `ci/github-actions-review.yml`: GitHub Actions workflow for PR review

## Usage

```bash
mkdir -p "$HOME/.config/prev"
cp examples/configs/v1-minimal-openai.yml "$HOME/.config/prev/config.yml"
prev config validate
```
