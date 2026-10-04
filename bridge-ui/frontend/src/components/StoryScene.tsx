import { useEffect, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { Pause, Play } from 'lucide-react';
import { Logo } from '../lib';

// A looping, self-explaining demo: agent works → needs approval → phone buzzes
// → you tap → agent continues. Each beat lasts STEP_MS[i].
const STEP_MS = [2400, 1700, 2000, 1100, 3200];
const CAPTIONS = [
    'Your agent is working on its own…',
    '…until it needs your OK.',
    'Your phone buzzes. You decide.',
    'One tap.',
    'And it keeps going.',
];

export default function StoryScene() {
    const [step, setStep] = useState(0);

    useEffect(() => {
        const t = setTimeout(() => setStep(s => (s + 1) % STEP_MS.length), STEP_MS[step]);
        return () => clearTimeout(t);
    }, [step]);

    const asking = step >= 1;
    const answered = step >= 4;

    return (
        <div className="scene">
            {/* IDE */}
            <motion.div className="ide" initial={{ opacity: 0, x: -20 }} animate={{ opacity: 1, x: 0 }} transition={{ duration: .6 }}>
                <div className="ide-bar"><i /><i /><i /><span>my-app — Agent</span></div>
                <div className="ide-body">
                    <Line show d={0}><span className="k">agent</span> Refactoring auth module…</Line>
                    <Line show d={.25}><span className="ok">✓</span> Updated 6 files</Line>
                    <Line show d={.5}><span className="ok">✓</span> Moved helpers to <span className="k">lib/</span></Line>
                    <Line show={asking} d={0}><span className="w">?</span> Delete 14 unused files in <span className="k">src/legacy</span>?</Line>
                    <Line show={answered} d={.3}><span className="ok">✓</span> Removed 14 files</Line>
                    <Line show={answered} d={.6}><span className="ok">✓</span> Tests: 128 passed</Line>
                    <AnimatePresence mode="wait">
                        {asking && !answered && (
                            <motion.div key="w" className="ide-status wait" initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
                                <Pause size={12} /> Waiting for your approval
                            </motion.div>
                        )}
                        {answered && (
                            <motion.div key="g" className="ide-status go" initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0 }}>
                                <Play size={12} /> Approved from your phone · continuing
                            </motion.div>
                        )}
                    </AnimatePresence>
                </div>
            </motion.div>

            {/* Phone */}
            <motion.div
                className="phone"
                initial={{ opacity: 0, y: 30 }}
                animate={{ opacity: 1, y: 0, rotate: step === 2 ? [0, -1.5, 1.5, -1, 0] : 0 }}
                transition={{ duration: step === 2 ? .5 : .7 }}
            >
                <div className="phone-screen">
                    <div className="phone-notch" />
                    <div className="tg-head">
                        <div className="tg-avatar"><Logo size={28} ping={false} /></div>
                        <div>
                            <div className="tg-name">Momentum Bot</div>
                            <div className="tg-sub">bot</div>
                        </div>
                    </div>
                    <AnimatePresence>
                        {step === 1 && (
                            <motion.div className="tg-toast" initial={{ y: -40, opacity: 0 }} animate={{ y: 0, opacity: 1 }} exit={{ y: -40, opacity: 0 }}>
                                <b>Momentum Bot</b>Input needed · Cursor · my-app
                            </motion.div>
                        )}
                    </AnimatePresence>
                    <div className="tg-chat">
                        <AnimatePresence>
                            {step >= 2 && (
                                <motion.div className="tg-msg" initial={{ opacity: 0, y: 16, scale: .96 }} animate={{ opacity: 1, y: 0, scale: 1 }} exit={{ opacity: 0 }}>
                                    <b>🤖 Input needed</b><br />
                                    <span className="src">Cursor · my-app</span><br /><br />
                                    Delete 14 unused files in src/legacy?
                                    {answered ? (
                                        <div className="tg-answer">✅ Answered: Approve</div>
                                    ) : (
                                        <div className="tg-btns">
                                            <div className={`tg-btn ${step === 3 ? 'tap' : ''}`}>Approve</div>
                                            <div className="tg-btn">Deny</div>
                                            {/* The finger is anchored to the Approve button, so it always lands on it. */}
                                            <motion.div
                                                className="finger"
                                                initial={{ opacity: 0, x: 70, y: 60 }}
                                                animate={step === 3 ? { opacity: 1, x: 0, y: 0, scale: .8 } : { opacity: 1, x: 0, y: 0, scale: 1 }}
                                                transition={{ delay: step === 2 ? .8 : 0, duration: step === 2 ? .7 : .15, ease: 'easeInOut' }}
                                            />
                                        </div>
                                    )}
                                </motion.div>
                            )}
                        </AnimatePresence>
                    </div>
                </div>
            </motion.div>

            <div className="scene-caption">
                <div className="scene-steps">{STEP_MS.map((_, i) => <i key={i} className={i === step ? 'on' : ''} />)}</div>
                <AnimatePresence mode="wait">
                    <motion.span key={step} initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -4 }} transition={{ duration: .25 }}>
                        {CAPTIONS[step]}
                    </motion.span>
                </AnimatePresence>
            </div>
        </div>
    );
}

function Line({ show, d, children }: { show: boolean; d: number; children: React.ReactNode }) {
    return (
        <AnimatePresence>
            {show && (
                <motion.div className="ide-line" initial={{ opacity: 0, x: -6 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0 }} transition={{ delay: d, duration: .3 }}>
                    {children}
                </motion.div>
            )}
        </AnimatePresence>
    );
}
