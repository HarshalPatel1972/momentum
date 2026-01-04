import { motion } from 'framer-motion';
import { Settings, Zap } from 'lucide-react';

interface WelcomeScreenProps {
    onStart: () => void;
    onSettings: () => void;
}

export default function WelcomeScreen({ onStart, onSettings }: WelcomeScreenProps) {
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
                    Remote Bridge
                </motion.h1>

                {/* Slogan */}
                <motion.p 
                    className="slogan"
                    initial={{ y: 20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.4 }}
                >
                    Your Agent, Unchained.
                </motion.p>

                {/* Start Button */}
                <motion.button 
                    className="start-btn"
                    onClick={onStart}
                    initial={{ y: 20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.5 }}
                    whileHover={{ scale: 1.05 }}
                    whileTap={{ scale: 0.98 }}
                >
                    Start
                </motion.button>
            </div>
        </motion.div>
    );
}
