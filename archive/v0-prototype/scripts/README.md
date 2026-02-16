# Scripts

## count_tokens.py

Count tokens in markdown files for embedding cost estimation.

### Purpose

Analyzes markdown files to estimate the number of tokens and associated costs for generating embeddings with OpenAI's embedding models. Uses the `cl100k_base` encoding (same as used by text-embedding-3-small, text-embedding-3-large, and text-embedding-ada-002).

### Setup

Install Python dependencies using mise:

```sh
mise run install-python
```

This creates a virtual environment at `.venv` and installs tiktoken and numpy.

### Usage

Using the mise task (recommended):

```sh
mise run count-tokens <directory>
```

Or directly with the venv Python:

```sh
.venv/bin/python scripts/count_tokens.py <directory>
```

### Options

- `directory` - Directory to scan (recursively processes all `*.md` files)
- `--encoding` - Tiktoken encoding to use (default: `cl100k_base`)
- `--top N` - Show top N largest files (default: 10)

### Example

```sh
# Count tokens in docs directory
mise run count-tokens docs

# Count tokens in a different directory
mise run count-tokens /path/to/obsidian/vault

# Show top 20 largest files
.venv/bin/python scripts/count_tokens.py docs --top 20
```

### Output

The script provides:
- File count and total tokens
- Min, max, mean, median, standard deviation
- Percentiles (p25, p50, p75, p90, p95, p99)
- Cost estimates for OpenAI embedding models
- List of largest files by token count

### Example Output

```
============================================================
Token Count Summary (cl100k_base encoding)
============================================================
Files processed:    15
Total tokens:       23,218
Min tokens:         387
Max tokens:         6,494
Mean tokens:        1,547.87
Median tokens:      996.00
Std deviation:      1,680.66

Percentiles:
  25th (p25):       689.50
  50th (p50):       996.00
  75th (p75):       1,353.00
  90th (p90):       3,502.00
  95th (p95):       5,360.00
  99th (p99):       6,267.20

Cost Estimates (verify current pricing):
  text-embedding-3-small: $0.0005
  text-embedding-3-large: $0.0030
  text-embedding-ada-002:  $0.0023
```
