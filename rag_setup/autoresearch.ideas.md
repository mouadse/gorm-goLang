# Autoresearch Ideas

Ideas for future optimization that are more complex or require more investigation.

## Status: OPTIMIZED
**Latency**: 67.0% improvement achieved through query caching (3270ms → 1080ms average)
**Quality**: 4.80/5 maintained

## Remaining Ideas (Not Worth Pursuing Now)

### Latency Optimization
- **Streaming responses**: Would improve perceived latency but not actual latency. Complex, requires client changes.
- **Local LLM**: Would eliminate network latency (~3s) but requires infrastructure changes. Out of scope.
- **Embedding caching**: Query caching already covers this. Embedding step is fast (~50-200ms) vs LLM call (~3s).

### Quality Optimization
- **Reranking**: Would add ~100-300ms latency per query. Quality is already 4.80/5 so ROI is limited.
- **Query expansion**: Could improve recall for query q3 (currently 4/5) but might overfit to benchmark.
- **Different embedding model**: Would require re-ingestion. Quality is already very high.

### Production Features (Already Implemented)
- ✅ **Cache TTL**: Implemented with RAG_CACHE_TTL_SECONDS env var (default 1 hour)
- ✅ **Cache hit/miss tracking**: GET /cache/stats now includes hits and misses
- ✅ **Cache management**: GET /cache/stats, POST /cache/clear endpoints

### Production Features (Not Pursuing)
- **Cache persistence**: Persist cache to disk to survive restarts. Adds complexity.
- **Observability**: OpenTelemetry tracing for debugging. Could be added later if needed.

## Tried and Not Worth Pursuing
- **Hybrid retrieval tuning**: Adjusting top_k values didn't help (tried 4, 8, 12)
- **Shorter prompts**: Didn't help with latency
- **Different LLM models**: All tested models have similar latency (~3s)
- **Smaller max_tokens**: Didn't help with latency
- **Query mode changes**: Default vs hybrid didn't affect latency