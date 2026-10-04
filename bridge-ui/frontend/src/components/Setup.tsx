import { useState } from 'react';
import { ArrowLeft, ArrowRight, Check, Lock } from 'lucide-react';
import TelegramSetup from './TelegramSetup';
import WhatsAppSetup from './WhatsAppSetup';
import SlackSetup from './SlackSetup';
import DiscordSetup from './DiscordSetup';
import NtfySetup from './NtfySetup';
import ChannelPicker, { ChannelId } from './ChannelPicker';
import IdeList from './IdeList';
import TestQuestion from './TestQuestion';

const STEPS = [
    { title: 'Link your phone', desc: 'Where the pager reaches you' },
    { title: 'Plug in your IDEs', desc: 'One click for each IDE found' },
    { title: 'Send a test page', desc: 'See it work for real' },
];

export default function Setup({ onFinish, onBack, initialStep = 0 }: { onFinish: () => void; onBack: () => void; initialStep?: number }) {
    const [step, setStep] = useState(Math.min(Math.max(initialStep, 0), 2));
    const [linked, setLinked] = useState(false);
    const [channel, setChannel] = useState<ChannelId>('telegram');
    const [tested, setTested] = useState(false);
    const done = () => setLinked(true);

    return (
        <div className="setup">
            <aside className="setup-rail">
                <div className="cap" style={{ margin: '0 10px 8px' }}>Setup</div>
                {STEPS.map((s, i) => (
                    <div key={i} className={`setup-step ${i === step ? 'on' : ''} ${i < step ? 'done' : ''}`}>
                        <div className="num">{i < step ? <Check size={14} /> : i + 1}</div>
                        <div><b>{s.title}</b><span>{s.desc}</span></div>
                    </div>
                ))}
                <div className="foot"><Lock size={14} style={{ flex: 'none', marginTop: 2 }} /><span>Everything stays on this PC. Momentum has no servers and no account; it talks straight to your messaging app.</span></div>
            </aside>

            <main className="setup-main">
                <div className="setup-inner">
                    <div className="cap">Step {step + 1} of 3</div>

                    {step === 0 && <>
                        <h2>Link your phone</h2>
                        <p className="lede">Choose where your agents should page you. You only do this once.</p>
                        <ChannelPicker value={channel} onChange={c => { setChannel(c); setLinked(false); }} />
                        <div className="plate plate-pad" style={{ marginTop: 14 }}>
                            {channel === 'telegram' && <TelegramSetup onLinked={done} />}
                            {channel === 'slack' && <SlackSetup onLinked={done} />}
                            {channel === 'discord' && <DiscordSetup onLinked={done} />}
                            {channel === 'ntfy' && <NtfySetup onLinked={done} />}
                            {channel === 'whatsapp' && <WhatsAppSetup onSaved={done} />}
                        </div>
                    </>}

                    {step === 1 && <>
                        <h2>Plug in your IDEs</h2>
                        <p className="lede">Momentum plugs into each IDE as an MCP server. Its agents learn to page you when they need a decision.</p>
                        <IdeList compact />
                    </>}

                    {step === 2 && <>
                        <h2>Send a test page</h2>
                        <p className="lede">A real page, through the exact path your agents use. Answer it on your phone and watch it arrive here.</p>
                        <TestQuestion onDone={() => setTested(true)} />
                    </>}

                    <div className="setup-nav">
                        <button className="key flat" onClick={() => (step === 0 ? onBack() : setStep(step - 1))}><ArrowLeft size={15} /> Back</button>
                        <span className="spacer" />
                        {step < 2
                            ? <button className="key go" disabled={step === 0 && !linked} onClick={() => setStep(step + 1)}>Continue <ArrowRight size={15} /></button>
                            : <button className="key go" onClick={onFinish}>{tested ? 'Start paging' : 'Skip and finish'} <ArrowRight size={15} /></button>}
                    </div>
                </div>
            </main>
        </div>
    );
}
