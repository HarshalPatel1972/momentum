import { useState } from 'react';
import { Send } from 'lucide-react';
import { api } from '../lib';

type Phase = 'idle' | 'waiting' | 'ok' | 'bad';

/** Sends a real page to the phone and shows the answer coming back on the pager's screen. */
export default function TestQuestion({ onDone }: { onDone?: () => void }) {
    const [phase, setPhase] = useState<Phase>('idle');
    const [answer, setAnswer] = useState('');

    const send = async () => {
        setPhase('waiting');
        const res = await api.sendTest();
        setAnswer(res.replace(/^Error:\s*/, ''));
        setPhase(res.startsWith('Error') ? 'bad' : 'ok');
        if (!res.startsWith('Error')) onDone?.();
    };

    const screen = {
        idle: { top: 'TEST', main: 'READY TO PAGE', sub: 'goes through the exact path your agents use' },
        waiting: { top: 'TEST · SENT', main: 'CHECK YOUR PHONE', sub: 'tap a button there…' },
        ok: { top: 'TEST · ANSWERED', main: 'IT WORKS ✓', sub: `you answered “${answer}”` },
        bad: { top: 'TEST · FAILED', main: 'NO SIGNAL', sub: answer },
    }[phase];

    return (
        <div className="device">
            <div className="device-label"><span>Test page</span><span className="live"><span className={`led ${phase === 'waiting' ? 'on blink' : phase === 'ok' ? 'green' : ''}`} />{phase}</span></div>
            <div className="lcd" style={{ minHeight: 150 }}>
                <div className="lcd-top"><span>{screen.top}</span><span /></div>
                <div className="lcd-main">{screen.main}{phase === 'waiting' && <span className="cursor" />}</div>
                <div className="lcd-sub">{screen.sub}</div>
            </div>
            <div className="device-keys">
                <button className="key go" onClick={send} disabled={phase === 'waiting'}><Send size={14} /> {phase === 'idle' ? 'Send test page' : 'Send another'}</button>
            </div>
        </div>
    );
}
