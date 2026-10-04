import { useState } from 'react';
import { motion } from 'framer-motion';
import { Check, Send, Smartphone, X } from 'lucide-react';
import { api } from '../lib';

type Phase = 'idle' | 'waiting' | 'ok' | 'bad';

/** Sends a real question to the phone and shows the answer coming back. */
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

    return (
        <div className="card test-stage">
            <motion.div key={phase} className={`test-orb ${phase === 'waiting' ? 'wait' : phase}`} initial={{ scale: .9 }} animate={{ scale: 1 }}>
                {phase === 'ok' ? <Check size={40} /> : phase === 'bad' ? <X size={40} /> : <Smartphone size={38} />}
            </motion.div>
            {phase === 'idle' && <>
                <h3>Send yourself a real question</h3>
                <p className="muted" style={{ maxWidth: 380 }}>It goes through the exact path your agents will use. Tap a button when it arrives on your phone.</p>
                <button className="btn btn-primary btn-lg" onClick={send}><Send size={16} /> Send test to my phone</button>
            </>}
            {phase === 'waiting' && <>
                <h3>Check your phone</h3>
                <p className="muted">Waiting for your tap in Telegram…</p>
            </>}
            {phase === 'ok' && <>
                <h3>It works.</h3>
                <p className="muted">You answered <b style={{ color: 'var(--text)' }}>“{answer}”</b>. Your agents can now reach you anywhere.</p>
                <button className="btn btn-ghost btn-sm" onClick={send}>Send another</button>
            </>}
            {phase === 'bad' && <>
                <h3>That didn't go through</h3>
                <p className="muted" style={{ maxWidth: 420 }}>{answer}</p>
                <button className="btn btn-secondary" onClick={send}>Try again</button>
            </>}
        </div>
    );
}
