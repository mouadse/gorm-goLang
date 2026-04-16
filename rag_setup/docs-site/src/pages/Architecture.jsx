import MermaidDiagram from '../components/MermaidDiagram';
import { Blocks, Database, Layout, RefreshCw } from 'lucide-react';
import './Pages.css';

const architectureDiagram = `graph LR
    User[User] -->|1 Ask question| Streamlit[Streamlit UI]
    Streamlit -->|2 POST query| API[FastAPI Backend]
    API -->|3 Search top k| Qdrant[(Qdrant Vector DB)]
    Qdrant -->|4 Return chunks| API
    API -->|5 Send prompt with context| OpenRouter[OpenRouter Gemini]
    OpenRouter -->|6 Return answer| API
    API -->|7 Return JSON| Streamlit
    Streamlit -->|8 Show answer| User

    Admin[Admin or CI job] -->|Run ingest| Ingest[Ingestion Service]
    Books[books PDF files] -->|Read source docs| Ingest
    Ingest -->|Chunk text| Chunker[Chunker]
    Chunker -->|Create embeddings| Embedder[Embedding Model]
    Embedder -->|Upsert vectors| Qdrant

    style Streamlit fill:#2d1a3e,stroke:#ec4899,stroke-width:2px,color:#fff
    style API fill:#1a1a3e,stroke:#6366f1,stroke-width:2px,color:#fff
    style Ingest fill:#1a1a3e,stroke:#6366f1,stroke-width:2px,color:#fff
    style Chunker fill:#1a1a3e,stroke:#6366f1,stroke-width:2px,color:#fff
    style Qdrant fill:#0d2b2b,stroke:#14b8a6,stroke-width:2px,color:#fff
    style OpenRouter fill:#2b2000,stroke:#f59e0b,stroke-width:2px,color:#fff
    style Embedder fill:#2b2000,stroke:#f59e0b,stroke-width:2px,color:#fff
    style Books fill:#1c1c26,stroke:#6b7280,stroke-width:1px,color:#a0a0ab
    style User fill:#1c1c26,stroke:#6b7280,stroke-width:1px,color:#a0a0ab
    style Admin fill:#1c1c26,stroke:#6b7280,stroke-width:1px,color:#a0a0ab`;

export default function Architecture() {
    return (
        <div className="page-container">
            <header className="page-header">
                <h1 className="page-title text-gradient">System Architecture</h1>
                <p className="page-subtitle">A high-level overview of the components and how they interact to power the RAG pipeline.</p>
            </header>

            <section className="diagram-section glass-panel hover-lift">
                <h2 className="section-title">Architecture Diagram</h2>
                <MermaidDiagram chart={architectureDiagram} />
            </section>

            <div className="grid grid-2">
                <div className="card glass-panel hover-lift">
                    <div className="card-header">
                        <div className="icon-wrapper icon-pink">
                            <Layout size={24} />
                        </div>
                        <h3>Streamlit UI</h3>
                    </div>
                    <div className="card-body">
                        <p>
                            Provides a beautiful, chat-based interface. It manages chat history in session state and sends HTTP requests to the FastAPI backend.
                        </p>
                    </div>
                </div>

                <div className="card glass-panel hover-lift">
                    <div className="card-header">
                        <div className="icon-wrapper icon-indigo">
                            <Blocks size={24} />
                        </div>
                        <h3>FastAPI Backend</h3>
                    </div>
                    <div className="card-body">
                        <p>
                            The core API powered by LlamaIndex. It processes incoming queries, executes similarity search on Qdrant, and orchestrates the OpenRouter LLM call using a focused prompt.
                        </p>
                    </div>
                </div>

                <div className="card glass-panel hover-lift">
                    <div className="card-header">
                        <div className="icon-wrapper icon-teal">
                            <Database size={24} />
                        </div>
                        <h3>Qdrant Vector Database</h3>
                    </div>
                    <div className="card-body">
                        <p>
                            A persistent vector DB running in Docker. It stores dense embeddings of chunks and metadata to enable fast semantic search (and optionally sparse search for hybrid mode).
                        </p>
                    </div>
                </div>

                <div className="card glass-panel hover-lift">
                    <div className="card-header">
                        <div className="icon-wrapper icon-purple">
                            <RefreshCw size={24} />
                        </div>
                        <h3>Ingestion Engine</h3>
                    </div>
                    <div className="card-body">
                        <p>
                            An idempotent job container. Triggers when running <code>make ingest</code> or automatically if PDFs change. Rebuilds the vector index only if there are state mismatches.
                        </p>
                    </div>
                </div>
            </div>
        </div>
    );
}
