import MermaidDiagram from '../components/MermaidDiagram';
import { FileDown, Search } from 'lucide-react';
import './Pages.css';

const ingestionFlow = `sequenceDiagram
    participant Admin
    participant Ingest
    participant Manifest
    participant Docs
    participant Embedder
    participant Qdrant

    Admin->>Ingest: run ingest
    Ingest->>Manifest: compare fingerprint
    alt unchanged
        Manifest-->>Ingest: up to date
        Ingest-->>Admin: skip reindex
    else changed
        Ingest->>Docs: load PDFs
        Ingest->>Ingest: chunk text
        Ingest->>Embedder: create embeddings
        Embedder-->>Ingest: vectors
        Ingest->>Qdrant: upsert vectors
        Ingest->>Manifest: save fingerprint
        Ingest-->>Admin: reindex complete
    end`;

const queryFlow = `sequenceDiagram
    participant User
    participant Streamlit
    participant API
    participant Qdrant
    participant OpenRouter

    User->>Streamlit: ask question
    Streamlit->>API: post query
    API->>Qdrant: search top k chunks
    Qdrant-->>API: return chunks
    API->>API: build grounded prompt
    API->>OpenRouter: generate answer
    OpenRouter-->>API: return text
    API-->>Streamlit: return JSON answer
    Streamlit-->>User: show answer`;

export default function ProcessFlow() {
    return (
        <div className="page-container">
            <header className="page-header">
                <h1 className="page-title text-gradient">Process Flows</h1>
                <p className="page-subtitle">Deep dive into the pipeline logic: Ingestion Idempotency and the Query Execution Path.</p>
            </header>

            <div className="flow-sections">
                <section className="diagram-section glass-panel hover-lift">
                    <div className="section-header">
                        <div className="icon-wrapper icon-purple">
                            <FileDown size={24} />
                        </div>
                        <h2 className="section-title">Data Ingestion Pipeline</h2>
                    </div>
                    <p className="section-description">
                        The ingestion process is carefully designed to be idempotent. It tracks a manifest
                        based on file hashes, size, and config parameters to avoid unnecessary processing and API costs.
                    </p>
                    <MermaidDiagram chart={ingestionFlow} />
                </section>

                <section className="diagram-section glass-panel hover-lift">
                    <div className="section-header">
                        <div className="icon-wrapper icon-teal">
                            <Search size={24} />
                        </div>
                        <h2 className="section-title">Query Execution and Retrieval</h2>
                    </div>
                    <p className="section-description">
                        The FastAPI endpoint leverages LlamaIndex to query the Qdrant DB. Context is bundled into
                        a strict prompt passed to the OpenRouter/Gemini LLM, enforcing concise responses based solely on context.
                    </p>
                    <MermaidDiagram chart={queryFlow} />
                </section>
            </div>
        </div>
    );
}
