import { motion } from 'framer-motion';
import { ArrowLeft, MessageSquare, Mail, Phone, Smartphone, ChevronRight } from 'lucide-react';

type Channel = 'telegram' | 'whatsapp' | 'gmail' | 'sms';

interface ChannelSelectProps {
    source: 'agent' | 'mcp';
    onBack: () => void;
    onSelect: (channel: Channel) => void;
}

const channels = [
    {
        id: 'telegram' as const,
        name: 'Telegram',
        icon: MessageSquare,
        gradient: 'linear-gradient(135deg, #0088cc 0%, #00aaff 100%)',
        description: 'Instant bot notifications',
        tag: 'Recommended'
    },
    {
        id: 'whatsapp' as const,
        name: 'WhatsApp',
        icon: Phone,
        gradient: 'linear-gradient(135deg, #25D366 0%, #128C7E 100%)',
        description: 'Via CallMeBot API',
        tag: 'Popular'
    },
    {
        id: 'gmail' as const,
        name: 'Gmail',
        icon: Mail,
        gradient: 'linear-gradient(135deg, #EA4335 0%, #FBBC05 100%)',
        description: 'Email notifications',
        tag: null
    },
    {
        id: 'sms' as const,
        name: 'SMS',
        icon: Smartphone,
        gradient: 'linear-gradient(135deg, #52525b 0%, #3f3f46 100%)',
        description: 'Via Twilio',
        tag: null
    }
];

export default function ChannelSelect({ source, onBack, onSelect }: ChannelSelectProps) {
    return (
        <motion.div 
            className="channel-select"
            initial={{ opacity: 0, x: 50 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -50 }}
            transition={{ duration: 0.3 }}
        >
            <button className="back-btn" onClick={onBack}>
                <ArrowLeft size={20} />
                <span>Back</span>
            </button>

            <div className="channel-content">
                <motion.div 
                    className="channel-header"
                    initial={{ y: -20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.1 }}
                >
                    <span className="step-indicator">Step 2 of 3</span>
                    <h2 className="channel-title">Choose Your Channel</h2>
                    <p className="channel-subtitle">
                        Where should we send notifications when your Agent needs you?
                    </p>
                </motion.div>

                <div className="channel-list">
                    {channels.map((channel, index) => {
                        const Icon = channel.icon;
                        
                        return (
                            <motion.button 
                                key={channel.id}
                                className="channel-row"
                                onClick={() => onSelect(channel.id)}
                                initial={{ y: 20, opacity: 0 }}
                                animate={{ y: 0, opacity: 1 }}
                                transition={{ delay: 0.15 + index * 0.05 }}
                                whileHover={{ x: 4 }}
                                whileTap={{ scale: 0.98 }}
                            >
                                <div 
                                    className="channel-row-icon"
                                    style={{ background: channel.gradient }}
                                >
                                    <Icon size={24} />
                                </div>
                                <div className="channel-row-info">
                                    <div className="channel-row-name">
                                        {channel.name}
                                        {channel.tag && (
                                            <span className="channel-tag">{channel.tag}</span>
                                        )}
                                    </div>
                                    <div className="channel-row-desc">{channel.description}</div>
                                </div>
                                <ChevronRight size={20} className="channel-row-arrow" />
                            </motion.button>
                        );
                    })}
                </div>
            </div>
        </motion.div>
    );
}
