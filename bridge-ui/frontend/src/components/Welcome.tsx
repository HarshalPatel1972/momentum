import { motion } from 'framer-motion';
import { ArrowRight, Github, ShieldCheck } from 'lucide-react';
import { api, Brand, REPO_URL } from '../lib';
import StoryScene from './StoryScene';

export default function Welcome({ onStart }: { onStart: () => void }) {
    const rise = (d: number) => ({ initial: { opacity: 0, y: 14 }, animate: { opacity: 1, y: 0 }, transition: { delay: d, duration: .5 } });

    return (
        <div className="welcome">
            <div className="welcome-left">
                <Brand />
                <div className="welcome-copy">
                    <motion.div className="eyebrow" {...rise(.05)}>For AI coding agents</motion.div>
                    <motion.h1 {...rise(.12)}>
                        Your agent keeps going.<br /><span className="grad">Even when you’re away.</span>
                    </motion.h1>
                    <motion.p className="lede" {...rise(.2)}>
                        Agents in VS Code, Cursor, Windsurf, Claude and others stop and wait whenever they need your OK.
                        Momentum sends those questions to your phone. You tap once, and the work continues.
                    </motion.p>
                    <motion.div className="welcome-cta" {...rise(.28)}>
                        <button className="btn btn-primary btn-lg" onClick={onStart}>
                            Set up in 2 minutes <ArrowRight size={17} />
                        </button>
                        <button className="btn btn-ghost btn-lg" onClick={() => api.openURL(REPO_URL)}>
                            <Github size={17} /> View source
                        </button>
                    </motion.div>
                    <motion.div className="trust-row" {...rise(.36)}>
                        <span><ShieldCheck size={14} /> Free and open source</span>
                        <span><ShieldCheck size={14} /> No account, no Momentum servers</span>
                        <span><ShieldCheck size={14} /> Nothing on your PC is exposed to the internet</span>
                    </motion.div>
                </div>
            </div>
            <div className="welcome-right">
                <StoryScene />
            </div>
        </div>
    );
}
