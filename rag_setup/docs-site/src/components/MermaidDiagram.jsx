import React, { useEffect, useRef } from 'react';
import mermaid from 'mermaid';

mermaid.initialize({
    startOnLoad: false,
    theme: 'dark',
    themeVariables: {
        fontFamily: 'Inter',
        primaryColor: 'transparent',
        primaryTextColor: '#f3f7ff',
        primaryBorderColor: 'rgba(99, 102, 241, 0.4)',
        lineColor: 'rgba(255, 255, 255, 0.3)',
        secondaryColor: 'rgba(236, 72, 153, 0.1)',
        tertiaryColor: 'rgba(20, 184, 166, 0.1)'
    },
    securityLevel: 'loose'
});

export default function MermaidDiagram({ chart }) {
    const containerRef = useRef(null);

    useEffect(() => {
        if (containerRef.current && chart) {
            const renderDiagram = async () => {
                try {
                    containerRef.current.innerHTML = '';
                    const id = `mermaid-${Math.random().toString(36).substr(2, 9)}`;
                    const { svg } = await mermaid.render(id, chart);
                    containerRef.current.innerHTML = svg;
                } catch (error) {
                    console.error('Mermaid render error:', error);
                    containerRef.current.innerHTML = `<div style="color: red;">Error rendering diagram</div>`;
                }
            };

            renderDiagram();
        }
    }, [chart]);

    return (
        <div
            className="mermaid-container"
            ref={containerRef}
            style={{
                width: '100%',
                display: 'flex',
                justifyContent: 'center',
                background: 'rgba(0, 0, 0, 0.2)',
                padding: '2rem',
                borderRadius: 'var(--radius-lg)',
                border: '1px solid var(--border-color)',
                overflowX: 'auto'
            }}
        />
    );
}
