import { useState, useEffect, useRef } from 'react';
import { motion } from 'framer-motion';
import { Square, ExternalLink, Copy, Check, ArrowLeft } from 'lucide-react';
import { StopBridge, StartBridge } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime";

interface BridgeControlProps {
    onStop: () => void;
    onBack?: () => void;
}

export default function BridgeControl({ onStop, onBack }: BridgeControlProps) {
    const [logs, setLogs] = useState<string[]>([]);
    const [publicURL, setPublicURL] = useState<string | null>(null);
    const [copied, setCopied] = useState(false);
    const [stopping, setStopping] = useState(false);
    const [bridgeStarted, setBridgeStarted] = useState(false);
    const logEndRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        // DO NOT auto-start - let user click Start button for control
        

        // Listen for log events
        const unsubLog = EventsOn("log", (message: string) => {
            setLogs(prev => [...prev, message]);
        });

        // Listen for public URL
        const unsubURL = EventsOn("publicURL", (url: string) => {
            setPublicURL(url);
        });

        // Listen for bridge stopped
        const unsubStopped = EventsOn("bridgeStopped", () => {
            setStopping(false);
            onStop();
        });

        return () => {
            unsubLog();
            unsubURL();
            unsubStopped();
        };
    }, [onStop]);

    // Auto-scroll logs
    useEffect(() => {
        logEndRef.current?.scrollIntoView({ behavior: "smooth" });
    }, [logs]);

    const handleStop = async () => {
        setStopping(true);
        await StopBridge();
        // Navigate back immediately after stop
        setTimeout(() => {
            onStop();
        }, 500);
    };

    const handleBack = () => {
        if (onBack) {
            onBack();
        }
    };

    const copyURL = () => {
        if (publicURL) {
            navigator.clipboard.writeText(publicURL);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        }
    };

    const handleStart = () => {
        setBridgeStarted(true);
        StartBridge().then((result) => {
            if (result.includes("Error")) {
                setLogs(prev => [...prev, `❌ ${result}`]);
            }
        });
    };

    return (
        <motion.div 
            className="bridge-control"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
        >
            {/* Back Button */}
            {onBack && (
                <button className="back-btn" onClick={handleBack} title="Back">
                    <ArrowLeft size={20} />
                </button>
            )}

            {!bridgeStarted ? (
                <div className="bridge-start-view">
                    <div className="start-prompt">
                        <h2>Ready to Start</h2>
                        <p>Click the button below to start the bridge and begin receiving notifications.</p>
                    </div>
                    <button 
                        className="start-bridge-btn"
                        onClick={handleStart}
                    >
                        Start Bridge
                    </button>
                </div>
            ) : (
                <>
                    <div className="bridge-control-header">
                        <div className="status-section">
                            <div className="status-indicator active" />
                            <span className="status-text">🚀 Bridge Running</span>
                        </div>
                        
                        {publicURL && (
                            <div className="url-badge" onClick={copyURL}>
                                <ExternalLink size={14} />
                                <span>{publicURL}</span>
                                {copied ? <Check size={14} /> : <Copy size={14} />}
                            </div>
                        )}
                    </div>

                    <div className="console">
                        <div className="console-header">
                            <span>Live Logs</span>
                            <span className="log-count">{logs.length} entries</span>
                        </div>
                        <div className="console-body">
                            {logs.map((log, i) => (
                                <div key={i} className="log-line">
                                    <span className="log-time">{new Date().toLocaleTimeString()}</span>
                                    <span className="log-message">{log}</span>
                                </div>
                            ))}
                            <div ref={logEndRef} />
                        </div>
                    </div>

                    <button 
                        className="stop-btn"
                        onClick={handleStop}
                        disabled={stopping}
                    >
                        <Square size={18} />
                        {stopping ? 'Stopping...' : 'Stop Bridge'}
                    </button>
                </>
            )}
        </motion.div>
    );
}
