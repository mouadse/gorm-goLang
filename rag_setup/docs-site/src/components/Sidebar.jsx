import { Book, Cpu, Database, Blocks, GitBranch } from 'lucide-react';
import { NavLink } from 'react-router-dom';
import './Sidebar.css';

export default function Sidebar() {
    return (
        <aside className="sidebar glass-panel">
            <div className="sidebar-header">
                <div className="logo-container">
                    <Book className="logo-icon" size={28} />
                </div>
                <div>
                    <h2 className="brand-title text-gradient">Book RAG</h2>
                    <span className="brand-subtitle">Architecture Docs</span>
                </div>
            </div>

            <nav className="sidebar-nav">
                <div className="nav-section">
                    <h3 className="nav-heading">Overview</h3>
                    <NavLink to="/" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
                        <Cpu size={18} />
                        <span>System Architecture</span>
                    </NavLink>
                    <NavLink to="/flow" className={({ isActive }) => `nav-link ${isActive ? 'active' : ''}`}>
                        <GitBranch size={18} />
                        <span>Process Flow</span>
                    </NavLink>
                </div>

                <div className="nav-section">
                    <h3 className="nav-heading">Components</h3>
                    <div className="nav-item-static">
                        <Blocks size={18} className="text-secondary" />
                        <div className="component-details">
                            <span>FastAPI Backend</span>
                            <span className="badge">Python</span>
                        </div>
                    </div>
                    <div className="nav-item-static">
                        <Database size={18} className="text-secondary" />
                        <div className="component-details">
                            <span>Qdrant DB</span>
                            <span className="badge badge-teal">Vector</span>
                        </div>
                    </div>
                    <div className="nav-item-static">
                        <Book size={18} className="text-secondary" />
                        <div className="component-details">
                            <span>Streamlit UI</span>
                            <span className="badge badge-pink">Frontend</span>
                        </div>
                    </div>
                </div>
            </nav>

            <div className="sidebar-footer">
                <p>Built for the RAG MVP Project</p>
            </div>
        </aside>
    );
}
