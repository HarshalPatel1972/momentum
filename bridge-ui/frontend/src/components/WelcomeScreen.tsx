import { motion } from 'framer-motion';
import { Settings, Zap, Plus, Activity } from 'lucide-react';
import { useEffect, useState } from 'react';
import { GetRecentChannels, IsBridgeRunning } from '../../wailsjs/go/main/App';

interface RecentChannel {
    name: string;
    icon: string;
    config_key: string;
    last_used: string;
}

interface WelcomeScreenProps {
    onStart: () => void;
    onSettings: () => void;
    onRecentSelect?: (channel: RecentChannel) => void;
    onViewBridge?: () => void;
}

export default function WelcomeScreen({ onStart, onSettings, onRecentSelect, onViewBridge }: WelcomeScreenProps) {
    const [recents, setRecents] = useState<RecentChannel[]>([]);
    const [bridgeRunning, setBridgeRunning] = useState(false);

    useEffect(() => {
        // Load recent channels
        GetRecentChannels().then(setRecents).catch(() => setRecents([]));
        
        // Check if bridge is running
        IsBridgeRunning().then(setBridgeRunning).catch(() => setBridgeRunning(false));
    }, []);

    const getTimeAgo = (timestamp: string) => {
        try {
            const date = new Date(timestamp);
            const seconds = Math.floor((Date.now() - date.getTime()) / 1000);
            
            if (seconds < 60) return 'Just now';
            if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
            if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
            return `${Math.floor(seconds / 86400)}d ago`;
        } catch {
            return 'Recently';
        }
    };

    return (
        <motion.div 
            className="welcome-screen"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0, y: -20 }}
            transition={{ duration: 0.4 }}
        >
            {/* Settings Icon - Top Right */}
            <button className="settings-btn" onClick={onSettings} title="Settings">
                <Settings size={20} />
            </button>

            {/* Main Content */}
            <div className="welcome-content">
                {/* Logo */}
                <motion.div 
                    className="logo"
                    initial={{ scale: 0.8, opacity: 0 }}
                    animate={{ scale: 1, opacity: 1 }}
                    transition={{ delay: 0.2, type: "spring", stiffness: 200 }}
                >
                    <div className="logo-icon">
                        <Zap size={48} strokeWidth={1.5} />
                    </div>
                </motion.div>

                {/* Title */}
                <motion.h1 
                    className="title"
                    initial={{ y: 20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.3 }}
                >
                    Momentum
                </motion.h1>

                {/* Tagline */}
                <motion.p 
                    className="slogan"
                    initial={{ y: 20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.4 }}
                >
                    Permission shouldn't require presence
                </motion.p>

                {/* Bridge Running Status - if active */}
                {bridgeRunning && onViewBridge && (
                    <motion.button
                        className="bridge-status-btn"
                        onClick={onViewBridge}
                        initial={{ y: 20, opacity: 0 }}
                        animate={{ y: 0, opacity: 1 }}
                        transition={{ delay: 0.5 }}
                        whileHover={{ scale: 1.02 }}
                        whileTap={{ scale: 0.98 }}
                    >
                        <Activity size={20} className="pulse" />
                        <div className="bridge-status-text">
                            <span className="status-title">🚀 Bridge is Running</span>
                            <span className="status-subtitle">Click to view</span>
                        </div>
                    </motion.button>
                )}

                {/* Recents Section - if any exist and bridge not running */}
                {!bridgeRunning && recents.length > 0 && (
                    <motion.div
                        className="recents-section"
                        initial={{ y: 20, opacity: 0 }}
                        animate={{ y: 0, opacity: 1 }}
                        transition={{ delay: 0.5 }}
                    >
                        <h3 className="recents-title">Recent Channels</h3>
                        <div className="recents-grid">
                            {recents.map((channel, index) => (
                                <motion.button
                                    key={channel.config_key}
                                    className="recent-card"
                                    onClick={() => onRecentSelect?.(channel)}
                                    initial={{ x: -20, opacity: 0 }}
                                    animate={{ x: 0, opacity: 1 }}
                                    transition={{ delay: 0.5 + index * 0.1 }}
                                    whileHover={{ scale: 1.02, y: -2 }}
                                    whileTap={{ scale: 0.98 }}
                                >
                                    <span className="recent-icon">{channel.icon}</span>
                                    <div className="recent-info">
                                        <span className="recent-name">{channel.name}</span>
                                        <span className="recent-time">{getTimeAgo(channel.last_used)}</span>
                                    </div>
                                </motion.button>
                            ))}
                        </div>
                    </motion.div>
                )}

                {/* Add New Button - only if bridge not running */}
                {!bridgeRunning && (
                    <motion.button 
                        className="start-btn"
                        onClick={onStart}
                        initial={{ y: 20, opacity: 0 }}
                        animate={{ y: 0, opacity: 1 }}
                        transition={{ delay: recents.length > 0 ? 0.7 : 0.5 }}
                        whileHover={{ scale: 1.05 }}
                        whileTap={{ scale: 0.98 }}
                    >
                        <Plus size={20} />
                        {recents.length > 0 ? 'Add New Channel' : 'Start'}
                    </motion.button>
                )}
            </div>
        </motion.div>
    );
}
