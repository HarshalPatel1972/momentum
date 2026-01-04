import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { ArrowLeft, MessageSquare, Mail, Phone, Smartphone, Check } from 'lucide-react';
import { SaveConfig, LoadConfig } from "../../wailsjs/go/main/App";

type Channel = 'telegram' | 'whatsapp' | 'gmail' | 'sms';

interface ConfigPageProps {
    channel: Channel;
    source: 'agent' | 'mcp';
    onBack: () => void;
    onComplete: () => void;
}

interface FormFields {
    [key: string]: string;
}

const channelInfo: Record<Channel, { name: string; icon: any; gradient: string }> = {
    telegram: { name: 'Telegram', icon: MessageSquare, gradient: 'linear-gradient(135deg, #0088cc 0%, #00aaff 100%)' },
    whatsapp: { name: 'WhatsApp', icon: Phone, gradient: 'linear-gradient(135deg, #25D366 0%, #128C7E 100%)' },
    gmail: { name: 'Gmail', icon: Mail, gradient: 'linear-gradient(135deg, #EA4335 0%, #FBBC05 100%)' },
    sms: { name: 'SMS', icon: Smartphone, gradient: 'linear-gradient(135deg, #52525b 0%, #3f3f46 100%)' }
};

const fieldConfigs: Record<Channel, { label: string; key: string; type?: string; placeholder: string; hint?: string }[]> = {
    telegram: [
        { label: 'Bot Token', key: 'bot_token', placeholder: '123456:ABC-DEF1234ghIkl-zyx57W2v', hint: 'Get this from @BotFather on Telegram' },
        { label: 'Chat ID', key: 'chat_id', placeholder: '123456789', hint: 'Your Telegram user/group ID' }
    ],
    whatsapp: [
        { label: 'CallMeBot API Key', key: 'api_key', placeholder: '123456', hint: 'Get this from callmebot.com' },
        { label: 'Your Phone Number', key: 'phone', placeholder: '+1234567890', hint: 'Include country code' }
    ],
    gmail: [
        { label: 'Your Gmail Address', key: 'email', type: 'email', placeholder: 'you@gmail.com' },
        { label: 'App Password', key: 'app_password', type: 'password', placeholder: '••••••••••••', hint: 'Generate in Google Account settings' }
    ],
    sms: [
        { label: 'Twilio Account SID', key: 'twilio_sid', placeholder: 'ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx' },
        { label: 'Auth Token', key: 'twilio_token', type: 'password', placeholder: '••••••••••••' },
        { label: 'Twilio Phone Number', key: 'from', placeholder: '+1234567890', hint: 'Your Twilio number' },
        { label: 'Your Phone Number', key: 'to', placeholder: '+1234567890', hint: 'Where to send SMS' }
    ]
};

export default function ConfigPage({ channel, source, onBack, onComplete }: ConfigPageProps) {
    const [fields, setFields] = useState<FormFields>({});
    const [ngrokToken, setNgrokToken] = useState('');
    const [saving, setSaving] = useState(false);
    const [message, setMessage] = useState('');

    const info = channelInfo[channel];
    const Icon = info.icon;
    const currentFields = fieldConfigs[channel];

    useEffect(() => {
        LoadConfig().then((jsonStr: string) => {
            try {
                const config = JSON.parse(jsonStr);
                if (config[channel]) {
                    setFields(config[channel]);
                }
                if (config.ngrokToken) {
                    setNgrokToken(config.ngrokToken);
                }
            } catch (e) {}
        });
    }, [channel]);

    const handleFieldChange = (key: string, value: string) => {
        setFields(prev => ({ ...prev, [key]: value }));
    };

    const isFormValid = () => {
        if (!ngrokToken) return false;
        return currentFields.every(f => fields[f.key]?.trim());
    };

    const handleSave = async () => {
        if (!isFormValid()) return;
        
        setSaving(true);
        setMessage('');

        const config = {
            channel,
            source,
            ngrokToken,
            [channel]: fields
        };

        const result = await SaveConfig(JSON.stringify(config));
        
        if (result.includes('Error')) {
            setMessage(result);
            setSaving(false);
        } else {
            setMessage('✓ Saved!');
            setTimeout(() => {
                setSaving(false);
                onComplete();
            }, 600);
        }
    };

    return (
        <motion.div 
            className="config-page"
            initial={{ opacity: 0, x: 50 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -50 }}
            transition={{ duration: 0.3 }}
        >
            <button className="back-btn" onClick={onBack}>
                <ArrowLeft size={20} />
                <span>Back</span>
            </button>

            <div className="config-page-content">
                <motion.div 
                    className="config-header"
                    initial={{ y: -20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.1 }}
                >
                    <span className="step-indicator">Step 3 of 3</span>
                    <div 
                        className="config-channel-icon"
                        style={{ background: info.gradient }}
                    >
                        <Icon size={32} />
                    </div>
                    <h2>Configure {info.name}</h2>
                    <p className="config-subtitle">Enter your credentials to enable notifications</p>
                </motion.div>

                <motion.div 
                    className="config-form-container"
                    initial={{ y: 20, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{ delay: 0.2 }}
                >
                    {/* Ngrok - Always Required */}
                    <div className="form-section">
                        <div className="form-section-title">
                            <span className="section-number">1</span>
                            Public Access
                        </div>
                        <div className="form-group">
                            <label>Ngrok Auth Token</label>
                            <input
                                type="password"
                                value={ngrokToken}
                                onChange={(e) => setNgrokToken(e.target.value)}
                                placeholder="Your ngrok authtoken"
                            />
                            <span className="form-hint">Required for remote access • Get from ngrok.com</span>
                        </div>
                    </div>

                    {/* Channel-specific fields */}
                    <div className="form-section">
                        <div className="form-section-title">
                            <span className="section-number">2</span>
                            {info.name} Credentials
                        </div>
                        {currentFields.map((field) => (
                            <div key={field.key} className="form-group">
                                <label>{field.label}</label>
                                <input
                                    type={field.type || 'text'}
                                    value={fields[field.key] || ''}
                                    onChange={(e) => handleFieldChange(field.key, e.target.value)}
                                    placeholder={field.placeholder}
                                />
                                {field.hint && <span className="form-hint">{field.hint}</span>}
                            </div>
                        ))}
                    </div>

                    <button 
                        className="save-btn large"
                        onClick={handleSave}
                        disabled={saving || !isFormValid()}
                    >
                        {saving ? (
                            'Saving...'
                        ) : (
                            <>
                                <Check size={20} />
                                Complete Setup
                            </>
                        )}
                    </button>

                    {message && (
                        <p className={`form-message ${message.includes('Error') ? 'error' : 'success'}`}>
                            {message}
                        </p>
                    )}
                </motion.div>
            </div>
        </motion.div>
    );
}
