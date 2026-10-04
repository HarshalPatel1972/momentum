import { useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { ArrowLeft, ArrowRight, Check, Lock } from 'lucide-react';
import { Brand } from '../lib';
import TelegramSetup from './TelegramSetup';
import WhatsAppSetup from './WhatsAppSetup';
import SlackSetup from './SlackSetup';
import DiscordSetup from './DiscordSetup';
import NtfySetup from './NtfySetup';
import ChannelPicker, { ChannelId } from './ChannelPicker';
import IdeList from './IdeList';
import TestQuestion from './TestQuestion';

const STEPS = [
    { title: 'Link your phone', desc: 'Telegram, Slack, Discord or ntfy' },
    { title: 'Connect your IDEs', desc: 'One click for every IDE we find' },
    { title: 'Try it for real', desc: 'Send yourself a test question' },
];

export default function Setup({ onFinish, onBack, initialStep = 0 }: { onFinish: () => void; onBack: () => void; initialStep?: number }) {
    const [step, setStep] = useState(Math.min(Math.max(initialStep, 0), 2));
    const [linked, setLinked] = useState(false);
    const [channel, setChannel] = useState<ChannelId>('telegram');
    const [tested, setTested] = useState(false);

    const canNext = step === 0 ? linked : true;

    return (
        <div className="setup">
            <aside className="setup-rail">
                <Brand />
                <div className="steps">
                    {STEPS.map((s, i) => (
                        <div key={i} className={`step ${i === step ? 'active' : ''} ${i < step ? 'done' : ''}`}>
                            <div className="step-num">{i < step ? <Check size={14} /> : i + 1}</div>
                            <div>
                                <div className="step-title">{s.title}</div>
                                <div className="step-desc">{s.desc}</div>
                            </div>
                        </div>
                    ))}
                </div>
                <div className="rail-foot">
                    <Lock size={14} />
                    <span>Everything stays on this PC. Momentum has no servers and no account. Messages go straight between your PC and your messaging app.</span>
                </div>
            </aside>

            <main className="setup-main">
                <AnimatePresence mode="wait">
                    <motion.div key={step} className="setup-inner"
                        initial={{ opacity: 0, x: 16 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, x: -16 }} transition={{ duration: .22 }}>
                        <div className="eyebrow">Step {step + 1} of 3</div>

                        {step === 0 && <>
                            <h2>Link your phone</h2>
                            <p className="lede">Choose where your agents&apos; questions should reach you. You only do this once.</p>
                            <ChannelPicker value={channel} onChange={c => { setChannel(c); setLinked(false); }} />
                            <div style={{ marginTop: 22 }}>
                                {channel === 'telegram' && <TelegramSetup onLinked={() => setLinked(true)} />}
                                {channel === 'slack' && <SlackSetup onLinked={() => setLinked(true)} />}
                                {channel === 'discord' && <DiscordSetup onLinked={() => setLinked(true)} />}
                                {channel === 'ntfy' && <NtfySetup onLinked={() => setLinked(true)} />}
                                {channel === 'whatsapp' && <WhatsAppSetup onSaved={() => setLinked(true)} />}
                            </div>
                        </>}

                        {step === 1 && <>
                            <h2>Connect your IDEs</h2>
                            <p className="lede">Momentum plugs into each IDE as an MCP server. Your agents learn to send questions to your phone when they need you.</p>
                            <IdeList compact />
                        </>}

                        {step === 2 && <>
                            <h2>Try it for real</h2>
                            <p className="lede">Send a real question to your phone. Tap a button there, and watch the answer arrive here.</p>
                            <TestQuestion onDone={() => setTested(true)} />
                        </>}

                        <div className="setup-nav">
                            <button className="btn btn-ghost" onClick={() => (step === 0 ? onBack() : setStep(step - 1))}><ArrowLeft size={16} /> Back</button>
                            <div className="spacer" />
                            {step < 2 ? (
                                <button className="btn btn-primary" disabled={!canNext} onClick={() => setStep(step + 1)}>Continue <ArrowRight size={16} /></button>
                            ) : (
                                <button className="btn btn-primary" onClick={onFinish}>{tested ? 'Open dashboard' : 'Skip and finish'} <ArrowRight size={16} /></button>
                            )}
                        </div>
                    </motion.div>
                </AnimatePresence>
            </main>
        </div>
    );
}
