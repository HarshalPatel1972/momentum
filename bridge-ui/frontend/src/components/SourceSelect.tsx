import { motion } from 'framer-motion';
import { Bot, Server, ArrowLeft } from 'lucide-react';

interface SourceSelectProps {
    onSelect: (source: 'agent' | 'mcp') => void;
    onBack: () => void;
}

export default function SourceSelect({ onSelect, onBack }: SourceSelectProps) {
    return (
        <motion.div 
            className="source-select"
            initial={{ opacity: 0, x: 50 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -50 }}
            transition={{ duration: 0.3 }}
        >
            {/* Back Button */}
            <button className="back-btn" onClick={onBack}>
                <ArrowLeft size={20} />
                <span>Back</span>
            </button>

            <div className="source-content">
                <motion.h2 
                    className="source-title"
                    initial={{ y: -10, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.1 }}
                >
                    Select Input Source
                </motion.h2>
                <motion.p 
                    className="source-subtitle"
                    initial={{ y: -10, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.15 }}
                >
                    Where is the request coming from?
                </motion.p>

                <div className="source-cards">
                    {/* AI Agent Card */}
                    <motion.div 
                        className="source-card"
                        onClick={() => onSelect('agent')}
                        initial={{ y: 20, opacity: 0 }}
                        animate={{ y: 0, opacity: 1 }}
                        transition={{ delay: 0.2 }}
                        whileHover={{ scale: 1.02, y: -4 }}
                        whileTap={{ scale: 0.98 }}
                    >
                        <div className="card-icon agent">
                            <Bot size={32} />
                        </div>
                        <h3>AI Agent</h3>
                        <p>VS Code Copilot, Cursor, Windsurf</p>
                    </motion.div>

                    {/* MCP Server Card */}
                    <motion.div 
                        className="source-card"
                        onClick={() => onSelect('mcp')}
                        initial={{ y: 20, opacity: 0 }}
                        animate={{ y: 0, opacity: 1 }}
                        transition={{ delay: 0.3 }}
                        whileHover={{ scale: 1.02, y: -4 }}
                        whileTap={{ scale: 0.98 }}
                    >
                        <div className="card-icon mcp">
                            <Server size={32} />
                        </div>
                        <h3>MCP Server</h3>
                        <p>Claude Desktop, Custom Tools</p>
                    </motion.div>
                </div>
            </div>
        </motion.div>
    );
}
