import { useState, useEffect } from 'react';
import { motion } from 'framer-motion';
import { SaveConfig, LoadConfig } from "../../wailsjs/go/main/App";

type Channel = 'telegram' | 'whatsapp' | 'gmail' | 'sms';

interface ConfigFormProps {
    channel: Channel;
    source: 'agent' | 'mcp';
    onSave: () => void;
}

interface FormFields {
    [key: string]: string;
}

const fieldConfigs: Record<Channel, { label: string; key: string; type?: string; placeholder: string }[]> = {
    telegram: [
        { label: 'Bot Token', key: 'bot_token', placeholder: '123456:ABC-DEF...' },
        { label: 'Chat ID', key: 'chat_id', placeholder: '123456789' }
    ],
    whatsapp: [
        { label: 'CallMeBot API Key', key: 'api_key', placeholder: '123456' },
        { label: 'Your Phone Number', key: 'phone', placeholder: '+1234567890' }
    ],
    gmail: [
        { label: 'Your Email', key: 'email', type: 'email', placeholder: 'you@gmail.com' },
        { label: 'App Password', key: 'app_password', type: 'password', placeholder: '••••••••••••' }
    ],
    sms: [
        { label: 'Twilio Account SID', key: 'twilio_sid', placeholder: 'ACxxxxxxxx...' },
        { label: 'Auth Token', key: 'twilio_token', type: 'password', placeholder: '••••••••••••' },
        { label: 'Twilio Number', key: 'from', placeholder: '+1234567890' },
        { label: 'Your Phone', key: 'to', placeholder: '+1234567890' }
    ]
};

export default function ConfigForm({ channel, source, onSave }: ConfigFormProps) {
    const [fields, setFields] = useState<FormFields>({});
    const [ngrokToken, setNgrokToken] = useState('');
    const [saving, setSaving] = useState(false);
    const [message, setMessage] = useState('');

    // Load existing config on mount
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
            } catch (e) {
                // No existing config
            }
        });
    }, [channel]);

    const handleFieldChange = (key: string, value: string) => {
        setFields(prev => ({ ...prev, [key]: value }));
    };

    const handleSave = async () => {
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
                onSave();
            }, 500);
        }
    };

    const currentFields = fieldConfigs[channel];

    return (
        <motion.div 
            className="config-form"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
        >
            <h3 className="form-title">Configure {channel.charAt(0).toUpperCase() + channel.slice(1)}</h3>
            
            {/* Ngrok Token - Always required */}
            <div className="form-group">
                <label>Ngrok Auth Token</label>
                <input
                    type="password"
                    value={ngrokToken}
                    onChange={(e) => setNgrokToken(e.target.value)}
                    placeholder="Your ngrok authtoken"
                />
                <span className="form-hint">Required for public URL access</span>
            </div>

            <div className="form-divider" />

            {/* Channel-specific fields */}
            {currentFields.map((field) => (
                <div key={field.key} className="form-group">
                    <label>{field.label}</label>
                    <input
                        type={field.type || 'text'}
                        value={fields[field.key] || ''}
                        onChange={(e) => handleFieldChange(field.key, e.target.value)}
                        placeholder={field.placeholder}
                    />
                </div>
            ))}

            <button 
                className="save-btn"
                onClick={handleSave}
                disabled={saving || !ngrokToken}
            >
                {saving ? 'Saving...' : 'Save & Continue'}
            </button>

            {message && (
                <p className={`form-message ${message.includes('Error') ? 'error' : 'success'}`}>
                    {message}
                </p>
            )}
        </motion.div>
    );
}
