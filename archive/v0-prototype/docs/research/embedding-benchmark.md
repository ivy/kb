# Embedding Benchmark: Obsidian Vault Token Analysis

**Date:** 2026-02-01
**Encoding:** `cl100k_base` (OpenAI embeddings)
**Tool:** `scripts/count_tokens.py`

## Purpose

This benchmark analyzes the token distribution across an Obsidian vault to estimate:
- Total embedding cost for indexing the entire vault
- Token distribution characteristics for chunking strategy
- Outlier files that may require special handling

## Summary Statistics

| Metric | Value |
|--------|-------|
| **Files Processed** | 1,820 |
| **Total Tokens** | 2,594,305 |
| **Min Tokens** | 0 |
| **Max Tokens** | 299,876 |
| **Mean Tokens** | 1,425.44 |
| **Median Tokens** | 421.50 |
| **Std Deviation** | 9,346.46 |

## Token Distribution

### Percentiles

| Percentile | Token Count |
|------------|-------------|
| 25th (p25) | 99.00 |
| 50th (p50) | 421.50 |
| 75th (p75) | 1,114.25 |
| 90th (p90) | 2,110.40 |
| 95th (p95) | 3,103.00 |
| 99th (p99) | 9,668.25 |

### Analysis

- **Highly skewed distribution**: Mean (1,425) >> Median (421) indicates many small files with occasional very large ones
- **Long tail**: 99th percentile at 9,668 tokens, but max is 299,876 (31x higher)
- **Most files are small**: 50% of files have ≤421 tokens (~1-2 paragraphs)
- **Large outliers exist**: Top file is 299k tokens, suggesting some files are aggregated reference docs

## Cost Estimates

Based on OpenAI pricing (current as of 2026-02-01):

| Model | Standard | Batch (50% off) |
|-------|----------|-----------------|
| **text-embedding-3-small** | $0.05 ($0.02/1M) | $0.03 ($0.01/1M) |
| **text-embedding-3-large** | $0.34 ($0.13/1M) | $0.17 ($0.065/1M) |

**Decision:** Use text-embedding-3-large. The cost difference is negligible ($0.14 extra with batch API) and the quality improvement for semantic search justifies the 6.5x price multiplier.

## Recommendations for Implementation

### Chunking Required

Given the token distribution (max 299,876 tokens vs text-embedding-3-large's 8,191 token limit), chunking will be necessary. At minimum:

- 5% of files exceed 3,103 tokens
- Top 20 files all exceed 9,000 tokens
- Largest file is 37x the embedding model's context limit

### Encoding Robustness

- Add fallback encoding detection (e.g., try latin-1, cp1252) for files that fail UTF-8
- Log encoding errors but continue processing
- Current error rate (0.1%) is acceptable

## Reproducing This Benchmark

```sh
# Install dependencies (first time only)
mise run install-python

# Run token count analysis
mise run count-tokens path/to/vault --top 20
```

Full script source: `scripts/count_tokens.py`
