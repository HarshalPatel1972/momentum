import { motion } from 'framer-motion';
import { ArrowLeft, Palette } from 'lucide-react';

interface SettingsProps {
    onBack: () => void;
}

export default function Settings({ onBack }: SettingsProps) {
    return (
        <motion.div 
            className="settings-screen"
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

            <div className="settings-content">
                <motion.h2 
                    className="settings-title"
                    initial={{ y: -10, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.1 }}
                >
                    Settings
                </motion.h2>

                <motion.div 
                    className="coming-soon-card"
                    initial={{ y: 20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.2 }}
                >
                    <Palette size={48} strokeWidth={1} />
                    <h3>Appearance Settings</h3>
                    <p>Coming Soon</p>
                </motion.div>
            </div>
        </motion.div>
    );
}
