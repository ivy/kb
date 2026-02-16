#!/usr/bin/env python3
"""
Count tokens in markdown files for embedding cost estimation.

Uses tiktoken with cl100k_base encoding (used by OpenAI embedding models
like text-embedding-ada-002, text-embedding-3-small, text-embedding-3-large).
"""

import argparse
import sys
from pathlib import Path
from typing import List

import tiktoken
import numpy as np


def count_tokens_in_file(file_path: Path, encoding) -> int:
    """Count tokens in a single file."""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            text = f.read()
        return len(encoding.encode(text))
    except Exception as e:
        print(f"Warning: Failed to process {file_path}: {e}", file=sys.stderr)
        return 0


def scan_directory(directory: Path, encoding) -> List[tuple]:
    """
    Recursively scan directory for *.md files and count tokens.

    Returns list of (file_path, token_count) tuples.
    """
    results = []
    for md_file in directory.rglob("*.md"):
        if md_file.is_file():
            token_count = count_tokens_in_file(md_file, encoding)
            results.append((md_file, token_count))
    return results


def compute_statistics(token_counts: List[int]) -> dict:
    """Compute statistical summary of token counts."""
    if not token_counts:
        return {}

    arr = np.array(token_counts)
    return {
        'count': len(token_counts),
        'total': int(np.sum(arr)),
        'min': int(np.min(arr)),
        'max': int(np.max(arr)),
        'mean': float(np.mean(arr)),
        'median': float(np.median(arr)),
        'std_dev': float(np.std(arr)),
        'p25': float(np.percentile(arr, 25)),
        'p50': float(np.percentile(arr, 50)),
        'p75': float(np.percentile(arr, 75)),
        'p90': float(np.percentile(arr, 90)),
        'p95': float(np.percentile(arr, 95)),
        'p99': float(np.percentile(arr, 99)),
    }


def format_cost_estimate(total_tokens: int) -> str:
    """
    Format cost estimates for common OpenAI embedding models.

    Pricing as of 2025 (verify current rates):
    - text-embedding-3-small: $0.02 / 1M tokens
    - text-embedding-3-large: $0.13 / 1M tokens
    - text-embedding-ada-002: $0.10 / 1M tokens
    """
    cost_small = (total_tokens / 1_000_000) * 0.02
    cost_large = (total_tokens / 1_000_000) * 0.13
    cost_ada = (total_tokens / 1_000_000) * 0.10

    return f"""
Cost Estimates (verify current pricing):
  text-embedding-3-small: ${cost_small:.4f}
  text-embedding-3-large: ${cost_large:.4f}
  text-embedding-ada-002:  ${cost_ada:.4f}"""


def main():
    parser = argparse.ArgumentParser(
        description="Count tokens in markdown files for embedding cost estimation"
    )
    parser.add_argument(
        "directory",
        type=Path,
        help="Directory to scan for *.md files (scans recursively)"
    )
    parser.add_argument(
        "--encoding",
        default="cl100k_base",
        help="Tiktoken encoding to use (default: cl100k_base for embeddings)"
    )
    parser.add_argument(
        "--top",
        type=int,
        default=10,
        help="Show top N largest files (default: 10)"
    )

    args = parser.parse_args()

    if not args.directory.is_dir():
        print(f"Error: {args.directory} is not a directory", file=sys.stderr)
        sys.exit(1)

    # Load encoding
    try:
        encoding = tiktoken.get_encoding(args.encoding)
    except Exception as e:
        print(f"Error: Failed to load encoding '{args.encoding}': {e}", file=sys.stderr)
        sys.exit(1)

    # Scan directory
    print(f"Scanning {args.directory} for *.md files...")
    results = scan_directory(args.directory, encoding)

    if not results:
        print("No markdown files found.")
        sys.exit(0)

    # Extract token counts
    token_counts = [count for _, count in results]

    # Compute statistics
    stats = compute_statistics(token_counts)

    # Print summary
    print(f"\n{'=' * 60}")
    print(f"Token Count Summary ({args.encoding} encoding)")
    print(f"{'=' * 60}")
    print(f"Files processed:    {stats['count']:,}")
    print(f"Total tokens:       {stats['total']:,}")
    print(f"Min tokens:         {stats['min']:,}")
    print(f"Max tokens:         {stats['max']:,}")
    print(f"Mean tokens:        {stats['mean']:,.2f}")
    print(f"Median tokens:      {stats['median']:,.2f}")
    print(f"Std deviation:      {stats['std_dev']:,.2f}")
    print(f"\nPercentiles:")
    print(f"  25th (p25):       {stats['p25']:,.2f}")
    print(f"  50th (p50):       {stats['p50']:,.2f}")
    print(f"  75th (p75):       {stats['p75']:,.2f}")
    print(f"  90th (p90):       {stats['p90']:,.2f}")
    print(f"  95th (p95):       {stats['p95']:,.2f}")
    print(f"  99th (p99):       {stats['p99']:,.2f}")

    print(format_cost_estimate(stats['total']))

    # Show largest files
    print(f"\n{'=' * 60}")
    print(f"Top {args.top} Largest Files")
    print(f"{'=' * 60}")
    sorted_results = sorted(results, key=lambda x: x[1], reverse=True)
    for i, (file_path, count) in enumerate(sorted_results[:args.top], 1):
        rel_path = file_path.relative_to(args.directory)
        print(f"{i:2}. {count:,} tokens - {rel_path}")


if __name__ == "__main__":
    main()
