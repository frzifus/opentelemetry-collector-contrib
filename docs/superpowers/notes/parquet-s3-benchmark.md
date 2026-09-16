# S3 Query Performance Benchmark Results

This document contains benchmark results for the Parquet-over-S3 trace query implementation, measuring real-world latency and S3 API request costs.

## Setup

**Prerequisites:**
- Real S3 bucket pre-populated with Parquet trace blocks (`.parquet` files) and metadata (`.meta.json` files)
- AWS credentials configured (via environment, `~/.aws/credentials`, or IAM role)
- Known trace IDs and service names from the bucket data

**Environment Variables:**

| Variable | Description | Default |
|----------|-------------|---------|
| `AWS_S3_BENCH_BUCKET` | S3 bucket name (required; benchmark skips if unset) | (none) |
| `AWS_REGION` | AWS region | `us-east-1` |
| `AWS_S3_BENCH_ENDPOINT` | Custom S3 endpoint (for testing with MinIO/LocalStack) | (none) |
| `AWS_S3_BENCH_PREFIX` | Object key prefix for trace blocks | `traces/` |
| `AWS_S3_BENCH_TRACEID` | Hex-encoded trace ID for TraceByID benchmark | `00000000000000000000000000000000` |
| `AWS_S3_BENCH_SERVICE` | Service name for Search benchmark | `frontend` |

**Running the Benchmarks:**

```bash
cd extension/s3queryextension

# Set required variables
export AWS_S3_BENCH_BUCKET=my-traces-bucket
export AWS_REGION=us-west-2
export AWS_S3_BENCH_TRACEID=a1b2c3d4e5f6789012345678901234ab  # real trace ID from your bucket
export AWS_S3_BENCH_SERVICE=my-service                        # real service name from your bucket

# Run TraceByID benchmark
go test ./internal/store/ -run '^$' -bench BenchmarkQueryTraceByID -benchtime 20x -v

# Run Search benchmark
go test ./internal/store/ -run '^$' -bench BenchmarkQuerySearch -benchtime 20x -v
```

**Notes:**
- The bucket must be pre-populated with blocks; these benchmarks measure read-path performance, not write-path.
- Adjust the trace ID and service name to match actual data in your bucket.
- Use `-benchtime Nx` (e.g., `20x`) to run exactly N iterations; use `-benchtime 30s` for time-based runs.
- The benchmark hard-codes a 24-hour time window starting from a fixed timestamp; adjust `startNano`/`endNano` in `bench_test.go` if your bucket contains data outside that window.

## Results

### TraceByID Query

**Configuration:** N blocks, trace ID `<hex>`, 24h time window

| Blocks | Latency/op (ms) | GETs/op | LISTs/op | Notes |
|--------|-----------------|---------|----------|-------|
| TBD | TBD | TBD | TBD | Run against real S3 |
| TBD | TBD | TBD | TBD | Run against real S3 |
| TBD | TBD | TBD | TBD | Run against real S3 |

### Search Query

**Configuration:** N blocks, `service.name = "<service>"`, 24h time window, limit 10

| Blocks | Latency/op (ms) | GETs/op | LISTs/op | Notes |
|--------|-----------------|---------|----------|-------|
| TBD | TBD | TBD | TBD | Run against real S3 |
| TBD | TBD | TBD | TBD | Run against real S3 |
| TBD | TBD | TBD | TBD | Run against real S3 |

## Interpretation vs Paper's 8–16 MiB Request-Size Premise

The Parquet-over-S3 design (based on the "AnalytiDB" paper and Tempo's approach) assumes that reading an 8–16 MiB Parquet block is cost-effective for S3: one GET request fetches the entire block, and Parquet's columnar layout + predicate pushdown minimize bytes scanned.

**Expected behavior:**
- **TraceByID:** 1 LIST (fetch all `.meta.json` keys), N GETs (one per block in the time window), where N depends on block count and time window. The bloom filter in each block's footer reduces unnecessary full-block reads when the trace ID is absent.
- **Search:** Similar to TraceByID, but may require reading more rows within each block (less selective than a single trace ID lookup).

**What to look for in results:**
1. **GETs/op:** Should correlate with the number of blocks in the time window. If significantly higher, investigate whether blocks are being read multiple times or bloom filters are not pruning effectively.
2. **LISTs/op:** Should be 1 per query (one LIST to fetch all metadata files). Higher values indicate inefficiency.
3. **Latency:** Dominated by S3 round-trip time + block download time. Compare against the paper's reported latencies for similar block counts.

**Potential findings:**
- If GETs/op is much higher than block count: bloom filter may not be working, or block pruning logic may be incorrect.
- If latency is much higher than expected: check network conditions, S3 region proximity, or block size (smaller blocks = more requests).
- If LISTs/op > 1: investigate whether the `FindBlocks` method is being called multiple times per query.

## Next Steps

1. Populate this template with actual benchmark results from a real S3 bucket.
2. Compare results against the paper's reported performance.
3. Identify optimization opportunities (e.g., caching metadata, adjusting block size).
4. Document any deviations from expected behavior.
