# Drive RAG Query Transformation to Agentic RAG Implementation Plan

## Summary

Current `DriveService.QueryRAG` should not pass the user's question directly to retrieval as the only search text. The first step is a lightweight query planning layer that separates `originalQuery` for answer generation from `retrievalQueries` for search. Later phases can add LLM planning, reranking, sufficiency checks, and a bounded agent loop.

## Phase 1: Deterministic Query Expansion

- Add `DriveRAGQueryPlan` and `DriveRAGRetrievalQuery`.
- Generate retrieval terms such as `白い インテリア 家具 デスク 椅子 棚 ソファ 木製 観葉植物 ミニマル` from `白いインテリアに合う家具は？`.
- Keep `originalQuery` unchanged for the final answer prompt.
- Run retrieval against expanded queries while preserving existing permission, DLP, scan status, and OpenFGA filtering through `SearchDocuments`.

## Phase 2: LLM Query Rewriting

- Add Drive RAG policy settings:
  - `queryRewriteEnabled`
  - `queryRewriteMode`: `none | deterministic | llm`
  - `queryRewriteMaxQueries`
- Reuse the configured local RAG generation runtime/model.
- Ask the model to return JSON with `searchQueries`, `keywords`, `mustHave`, and `avoid`.
- Fall back to deterministic expansion when LLM rewrite fails.

## Phase 3: Multi-Query Retrieval and Merge

- Execute multiple retrieval queries with hybrid search.
- Deduplicate candidates by `filePublicId`.
- Merge snippets and matches from duplicate hits.
- Rank by lexical overlap, semantic score, and query coverage.

## Phase 4: Rerank and Sufficiency Check

- Score candidates before adding them to RAG context.
- Check whether retrieved candidates cover core query signals.
- If coverage is insufficient, run one additional search with missing terms.

## Phase 5: Agentic RAG

- Introduce a `DriveRAGAgent` that controls query planning, retrieval, reranking, sufficiency check, one optional retry, and final cited answer generation.
- Cap retrieval loops at 2.
- Keep all external tool use limited to permission-filtered Drive search.

## Public Interfaces

- `DriveRAGPolicy` gains query rewrite settings with deterministic rewrite enabled by default.
- API response remains compatible: `answer`, `citations`, `matches`, and `blocked` are unchanged.
- Retrieval trace types can be exposed later for admin/debug use without changing the normal UI contract.

## Test Plan

- Unit tests cover deterministic expansion for `白いインテリアに合う家具は？`.
- Unit tests cover single-term `インテリア` search preservation.
- Unit tests cover LLM rewrite JSON parsing and deterministic fallback path by construction.
- Unit tests cover multi-query merge deduplication by file public ID.
- Existing rank tests continue to guard unrelated semantic hit filtering.

